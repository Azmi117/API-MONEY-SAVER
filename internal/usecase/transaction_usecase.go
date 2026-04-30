package usecase

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/service"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/gemini"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/ocr"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/utils"
	"google.golang.org/api/gmail/v1"
)

type TransactionUsecase interface {
	CreateManual(ctx context.Context, userID uint, req dto.CreateTransactionRequest) error
	ProcessScan(ctx context.Context, userID uint, workspaceID uint, imgData []byte, mimeType string) (*models.Transaction, error)
	ConfirmTransaction(ctx context.Context, transactionID uint) error
	SyncGmailTransactions(ctx context.Context) error
	ProcessEmailMandiri(ctx context.Context, userID uint, workspaceID uint, subject string, body string) (*models.Transaction, error)
	GetHistory(workspaceID uint) ([]models.Transaction, error)
	DeleteTransaction(transactionID uint) error
	ProcessScanHybrid2(ctx context.Context, userID uint, workspaceID uint, imgData []byte, mimeType string) (*dto.ProcessScanHybridResult, error)
	HardDeleteTransaction(id uint) error
	ApproveEmailLog(ctx context.Context, logID uint, userID uint) error
	RejectEmailLog(ctx context.Context, logID uint, userID uint) error
	GetPendingEmailLogs(userID uint) ([]models.EmailParsed, error)
	ProcessScanAlternative(ctx context.Context, imagePath string, userID uint, workspaceID uint) (*dto.ProcessScanHybridResult, error)
}

type transactionUsecase struct {
	repo           repository.TransactionRepository
	authRepo       repository.AuthRepository
	googleService  service.GoogleAuthService
	geminiClient   *gemini.GeminiClient
	hybridScanner  *ocr.HybridScanner
	wsRepo         repository.WorkspaceRepository
	ocrSpaceClient *ocr.OCRSpaceClient
}

func NewTransactionUsecase(repo repository.TransactionRepository, authRepo repository.AuthRepository, googleService service.GoogleAuthService, gemini *gemini.GeminiClient, hybridScanner *ocr.HybridScanner, wsRepo repository.WorkspaceRepository, ocrSpace *ocr.OCRSpaceClient) TransactionUsecase {
	return &transactionUsecase{
		repo:           repo,
		authRepo:       authRepo,
		googleService:  googleService,
		geminiClient:   gemini,
		hybridScanner:  hybridScanner,
		wsRepo:         wsRepo,
		ocrSpaceClient: ocrSpace,
	}
}

func (u *transactionUsecase) CreateManual(ctx context.Context, userID uint, req dto.CreateTransactionRequest) error {
	cleanMerchant := strings.ToLower(strings.TrimSpace(req.Merchant))
	isDuplicate, err := u.repo.IsDuplicate(req.WorkspaceID, req.Amount, cleanMerchant, req.Date)
	if err != nil {
		return apperror.Internal("Failed to check transaction duplicates!")
	}
	if isDuplicate {
		return apperror.Conflict("Similar transaction has already been recorded!")
	}

	tx := &models.Transaction{
		UserID:      userID,
		WorkspaceID: req.WorkspaceID,
		CategoryID:  req.CategoryID,
		Amount:      req.Amount,
		Type:        req.Type,
		Date:        req.Date,
		Note:        req.Note,
		Merchant:    cleanMerchant,
		Method:      req.Method,
		Source:      req.Source,
		Status:      "approved",
		GmailID:     req.GmailID,
	}

	isMember, err := u.wsRepo.IsMember(req.WorkspaceID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return errors.New("akses ditolak: lu bukan member di workspace ini")
	}

	if err := u.repo.Create(tx); err != nil {
		return apperror.Internal("Failed to save manual transaction!")
	}

	return nil
}

func (u *transactionUsecase) ProcessScan(ctx context.Context, userID uint, workspaceID uint, imgData []byte, mimeType string) (*models.Transaction, error) {

	user, err := u.authRepo.FindByID(userID) // Pastiin u.userRepo udah di-inject
	if err != nil {
		return nil, err
	}

	// 2. Cek Limit
	limit := u.getOCRLimit(user.AccountTier)
	if user.OCRUsageCount >= limit {
		return nil, fmt.Errorf("kuota scan mingguan lu abis cuy (Tier %s: %d/%d)", user.AccountTier, user.OCRUsageCount, limit)
	}

	result, err := u.geminiClient.ScanReceipt(ctx, imgData, mimeType)
	if err != nil {
		return nil, apperror.Internal("Gemini AI failed to process receipt scan!")
	}

	cleanMerchant := strings.Title(strings.ToLower(strings.TrimSpace(result.Merchant)))

	parsedDate, err := time.Parse("2006-01-02 15:04:05", result.Date)
	if err != nil {
		parsedDate = time.Now()
	}

	tx := &models.Transaction{
		UserID:           userID,
		WorkspaceID:      workspaceID,
		Amount:           result.Amount,
		Merchant:         cleanMerchant,
		Method:           result.Method,
		Note:             result.Note,
		Date:             parsedDate,
		Type:             result.Type,
		Source:           "scan",
		Status:           "pending",
		GmailID:          fmt.Sprintf("SCAN-%d", time.Now().UnixNano()),
		TransactionItems: []models.TransactionItem{},
	}

	for _, item := range result.Items {
		tx.TransactionItems = append(tx.TransactionItems, models.TransactionItem{
			Description: item.Description,
			Quantity:    item.Quantity,
			Price:       item.UnitPrice,
			Total:       item.Total,
		})
	}

	if err := u.repo.Create(tx); err != nil {
		return nil, apperror.Internal("Failed to save scanned transaction and items!")
	}

	if err := u.authRepo.IncrementOCRUsage(userID); err != nil {
		log.Printf("⚠️ Gagal increment OCR usage untuk user %d: %v", userID, err)
	}

	return tx, nil
}

func (u *transactionUsecase) SyncGmailTransactions(ctx context.Context) error {
	users, err := u.authRepo.FindAllWithGmail()
	if err != nil {
		log.Printf("❌ [Robot Sync] Gagal tarik user: %v", err)
		return apperror.Internal("Failed to retrieve users with Gmail integration!")
	}

	log.Printf("🔍 [Robot Sync] Menemukan %d user dengan integrasi Gmail", len(users))

	for _, user := range users {
		srv, err := u.googleService.GetGmailService(user.GoogleRefreshToken)
		if err != nil {
			log.Printf("❌ [Robot Sync] Gagal dapet Gmail Service buat user %d: %v", user.ID, err)
			continue
		}

		// --- TEST: Lebarin query dulu biar yakin email ketangkep ---
		query := "(from:no-reply@bankmandiri.co.id OR from:noreply.livin@bankmandiri.co.id)"

		res, err := srv.Users.Messages.List("me").Q(query).Do()
		if err != nil {
			log.Printf("❌ [Robot Sync] Gagal List Messages: %v", err)
			continue
		}

		log.Printf("📩 [Robot Sync] Scan Berhasil! Menemukan %d email potensial untuk user %s", len(res.Messages), user.Email)

		for _, m := range res.Messages {
			// Cek apakah sudah pernah diproses
			existingLog, _ := u.repo.GetEmailLogByGmailID(m.Id)
			if existingLog != nil {
				// Jangan log ini setiap menit biar terminal gak penuh, cukup skip aja
				continue
			}

			log.Printf("✨ [Robot Sync] Memproses email baru ID: %s", m.Id)

			fullMsg, err := srv.Users.Messages.Get("me", m.Id).Do()
			if err != nil {
				log.Printf("❌ [Robot Sync] Gagal ambil detail email %s: %v", m.Id, err)
				continue
			}

			var subject string
			for _, h := range fullMsg.Payload.Headers {
				if h.Name == "Subject" {
					subject = h.Value
				}
			}

			bodyStr := u.getBody(fullMsg.Payload)
			parsed := utils.ParseMandiriEmail(subject, bodyStr)

			// --- POINT PENTING: GmailID WAJIB DIISI ---
			emailLog := &models.EmailParsed{
				UserID:     user.ID,
				RawEmail:   bodyStr,
				BankSource: "Mandiri",
				Status:     "Pending",
				GmailID:    m.Id, // <--- INI JANGAN SAMPE KETINGGALAN LAGI MI!
			}

			if parsed != nil {
				emailLog.Amount = parsed.Amount
				emailLog.Merchant = parsed.Merchant
				emailLog.ParsedDate = parsed.Date
				emailLog.Method = parsed.Method
				emailLog.Note = parsed.Note
				emailLog.Type = "expense"
			} else {
				log.Printf("⚠️ [Robot Sync] Email %s ketemu tapi gagal diparse Regex", m.Id)
			}

			err = u.repo.CreateEmailLog(emailLog)
			if err != nil {
				log.Printf("❌ [Robot Sync] Gagal simpan ke DB: %v", err)
			} else {
				log.Printf("✅ [Robot Sync] Email berhasil dicatat: %s", m.Id)
			}
		}
	}
	return nil
}

func (u *transactionUsecase) getBody(payload *gmail.MessagePart) string {
	data := ""
	if payload.Body.Data != "" {
		data = payload.Body.Data
	} else {
		for _, part := range payload.Parts {
			if part.MimeType == "text/plain" || part.MimeType == "text/html" {
				data = part.Body.Data
				break
			}
			if len(part.Parts) > 0 {
				data = u.getBody(part)
				if data != "" {
					break
				}
			}
		}
	}

	decoded, _ := base64.URLEncoding.DecodeString(data)
	return string(decoded)
}

func (u *transactionUsecase) ProcessEmailMandiri(ctx context.Context, userID uint, workspaceID uint, subject string, body string) (*models.Transaction, error) {
	parsed := utils.ParseMandiriEmail(subject, body)
	if parsed == nil {
		return nil, apperror.UnprocessableEntity("Email format is not a supported Mandiri transaction!")
	}

	tx := &models.Transaction{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Amount:      parsed.Amount,
		Merchant:    parsed.Merchant,
		Date:        parsed.Date,
		Type:        "expense",
		Source:      "email",
		Status:      "pending",
	}

	if err := u.repo.Create(tx); err != nil {
		return nil, apperror.Internal("Failed to save email-processed transaction!")
	}
	return tx, nil
}

func (u *transactionUsecase) ProcessScanHybrid2(ctx context.Context, userID uint, workspaceID uint, imgData []byte, mimeType string) (*dto.ProcessScanHybridResult, error) {

	user, err := u.authRepo.FindByID(userID) // Pastiin u.userRepo udah di-inject
	if err != nil {
		return nil, err
	}

	if err := u.checkAndResetQuota(user); err != nil {
		log.Printf("⚠️ Gagal reset quota: %v", err)
	}

	// 2. Cek Limit
	limit := u.getOCRLimit(user.AccountTier)
	if user.OCRUsageCount >= limit {
		// Hitung sisa hari buat kasih info ke user
		nextReset := user.LastResetUsage.AddDate(0, 0, 7)
		daysLeft := int(time.Until(nextReset).Hours() / 24)

		if daysLeft == 0 {
			hoursLeft := int(time.Until(nextReset).Hours())
			return nil, fmt.Errorf("kuota abis! Reset otomatis %d jam lagi", hoursLeft)
		}
		return nil, fmt.Errorf("kuota scan mingguan abis (%d/%d). Reset %d hari lagi", user.OCRUsageCount, limit, daysLeft)
	}

	result, err := u.hybridScanner.ScanReceiptHybrid(ctx, imgData, mimeType)
	if err != nil {
		return nil, apperror.Internal("Hybrid scanner failed to process image!")
	}

	cleanMerchant := strings.Title(strings.ToLower(strings.TrimSpace(result.Merchant)))

	parsedDate := time.Now()
	if strings.TrimSpace(result.Date) != "" {
		layouts := []string{
			"2006-01-02 15:04:05",
			"2006-01-02 15:04",
			"2006-01-02",
		}

		for _, layout := range layouts {
			t, parseErr := time.Parse(layout, result.Date)
			if parseErr == nil {
				parsedDate = t
				break
			}
		}
	}

	tx := &models.Transaction{
		UserID:           userID,
		WorkspaceID:      workspaceID,
		Amount:           result.Amount,
		Merchant:         cleanMerchant,
		Method:           result.Method,
		Note:             result.Note,
		Date:             parsedDate,
		Type:             result.Type,
		Source:           result.Source,
		Status:           "pending",
		GmailID:          fmt.Sprintf("SCAN-%d", time.Now().UnixNano()),
		TransactionItems: []models.TransactionItem{},
	}

	for _, item := range result.Items {
		tx.TransactionItems = append(tx.TransactionItems, models.TransactionItem{
			Description: item.Description,
			Quantity:    item.Quantity,
			Price:       item.UnitPrice,
			Total:       item.Total,
		})
	}

	if err := u.repo.Create(tx); err != nil {
		return nil, apperror.Internal("Failed to save hybrid scan result!")
	}

	if err := u.authRepo.IncrementOCRUsage(userID); err != nil {
		log.Printf("⚠️ Gagal increment OCR usage untuk user %d: %v", userID, err)
	}

	return &dto.ProcessScanHybridResult{
		Transaction:  tx,
		Engine:       result.Engine,
		Confidence:   result.Confidence,
		FallbackUsed: result.FallbackUsed,
	}, nil
}

func (u *transactionUsecase) getOCRLimit(tier string) int {
	baseFree := 2
	multiplierPro := 5
	multiplierUlt := 10

	switch strings.ToLower(tier) {
	case "ultimate":
		return baseFree * multiplierPro * multiplierUlt // 100
	case "pro":
		return baseFree * multiplierPro // 10
	default:
		return baseFree // 2
	}
}

func (u *transactionUsecase) checkAndResetQuota(user *models.User) error {
	now := time.Now()

	// Jika LastResetUsage kosong (user baru), set ke sekarang dulu
	if user.LastResetUsage.IsZero() {
		user.LastResetUsage = now
		return u.authRepo.ResetOCRUsage(user.ID, now)
	}

	// Logic Refill: Cek apakah sudah lewat 30 hari (Single Period V1)
	// 30 hari * 24 jam = 720 jam
	if now.Sub(user.LastResetUsage).Hours() >= 168 {
		log.Printf("♻️ [Reset Quota] User %d masuk periode MINGGUAN baru. Resetting...", user.ID)

		// Reset di memory object biar logic selanjutnya (cek limit) pake angka 0
		user.OCRUsageCount = 0
		user.LastResetUsage = now

		// Reset di Database
		return u.authRepo.ResetOCRUsage(user.ID, now)
	}

	return nil
}

func (u *transactionUsecase) ApproveEmailLog(ctx context.Context, logID uint, userID uint) error {
	// 1. Ambil data log mentah
	logData, err := u.repo.GetEmailLogByID(logID)
	if err != nil {
		return apperror.NotFound("Data email log tidak ditemukan!")
	}

	// 2. Keamanan: Pastiin ini log punya user yang login
	if logData.UserID != userID {
		return apperror.Forbidden("Lu nggak berhak akses log ini!")
	}

	// 3. Cek status (pastiin masih pending)
	if logData.Status != "Pending" {
		return apperror.BadRequest("Email log ini sudah diproses!")
	}

	// 4. MAPPING DETAIL (Gak perlu logic if-else manual lagi!)
	// Langsung ambil dari logData yang sudah di-parse oleh parser_mandiri saat email masuk
	tx := &models.Transaction{
		UserID:      logData.UserID,
		WorkspaceID: 1, // Lu bisa buat dinamis nanti kalau udah main multi-workspace
		Amount:      logData.Amount,
		Merchant:    logData.Merchant,
		Date:        logData.ParsedDate,
		Source:      "email_auto",
		Status:      "approved",
		Method:      logData.Method, // Ambil dari hasil parsing awal
		Note:        logData.Note,   // Ambil dari hasil parsing awal (biar "Test QR" masuk sini)
		Type:        "expense",
		GmailID:     logData.GmailID,
	}

	// 5. Eksekusi simpan ke transaksi
	if err := u.repo.Create(tx); err != nil {
		return apperror.Internal("Gagal membuat transaksi dari email: " + err.Error())
	}

	// 6. Update status log jadi Success dan hubungkan TransactionID-nya
	// Pastiin repo UpdateEmailLogStatus lu juga bisa update TransactionID biar sinkron
	return u.repo.UpdateEmailLogStatus(logID, "Success")
}

func (u *transactionUsecase) RejectEmailLog(ctx context.Context, logID uint, userID uint) error {
	log, err := u.repo.GetEmailLogByID(logID)
	if err != nil {
		return apperror.NotFound("Data email log tidak ditemukan!")
	}

	if log.UserID != userID {
		return apperror.Forbidden("Akses ditolak!")
	}

	// Cukup update status jadi Rejected, nggak perlu masukin ke tabel transactions
	return u.repo.UpdateEmailLogStatus(logID, "Rejected")
}

func (u *transactionUsecase) ConfirmTransaction(ctx context.Context, transactionID uint) error {
	err := u.repo.UpdateStatus(transactionID, "approved")
	if err != nil {
		return apperror.NotFound("Transaction record not found to confirm!")
	}
	return nil
}

func (u *transactionUsecase) GetHistory(workspaceID uint) ([]models.Transaction, error) {
	history, err := u.repo.GetByWorkspaceID(workspaceID)
	if err != nil {
		return nil, apperror.Internal("Failed to retrieve transaction history!")
	}
	return history, nil
}

func (u *transactionUsecase) DeleteTransaction(transactionID uint) error {
	err := u.repo.Delete(transactionID)
	if err != nil {
		return apperror.NotFound("Transaction not found or already deleted!")
	}
	return nil
}

func (u *transactionUsecase) HardDeleteTransaction(id uint) error {
	return u.repo.HardDelete(id)
}

func (u *transactionUsecase) GetPendingEmailLogs(userID uint) ([]models.EmailParsed, error) {
	return u.repo.GetPendingEmailLogs(userID)
}

func (u *transactionUsecase) ProcessScanAlternative(ctx context.Context, imagePath string, userID uint, workspaceID uint) (*dto.ProcessScanHybridResult, error) {
	// 1. Ambil teks mentah dari OCR.space (Engine 2)
	rawText, err := u.ocrSpaceClient.ExtractRawText(imagePath)
	if err != nil {
		return nil, err
	}

	// 2. Panggil Manual Parser (Murni Logic Go)
	merchant, amount, transactionDate, items := u.manualParserV3(rawText)

	// 3. Mapping ke Model dengan status PENDING
	transaction := &models.Transaction{
		UserID:           userID,
		WorkspaceID:      workspaceID,
		Merchant:         merchant,
		Amount:           amount,
		Date:             transactionDate,
		Type:             "expense",
		Source:           "ocr_space_pure",
		Status:           "pending", // Sesuai flow: User must check
		Note:             "Auto-parsed: Please review amount and date",
		GmailID:          fmt.Sprintf("SCAN-ALT-%d-%d", userID, time.Now().UnixNano()),
		TransactionItems: items,
	}

	// 4. Save ke Database
	if err := u.repo.Create(transaction); err != nil {
		return nil, fmt.Errorf("failed to save pending scan: %w", err)
	}

	return &dto.ProcessScanHybridResult{
		Transaction:  transaction,
		Engine:       "OCR_SPACE_PURE",
		Confidence:   0,
		FallbackUsed: false,
	}, nil
}

// Helper Parser tanpa Gemini
func (u *transactionUsecase) manualParserV3(raw string) (string, float64, time.Time, []models.TransactionItem) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r", ""), "\n")

	var merchant string
	var amount float64
	var items []models.TransactionItem
	transactionDate := time.Now()

	// Regex untuk duit/angka ribuan
	reMoney := regexp.MustCompile(`\d{1,3}([.,]\d{3})+`)

	// 1. MERCHANT: Logic sekuensial dengan filter "Noise"
	for _, line := range lines {
		clean := strings.TrimSpace(line)
		upper := strings.ToUpper(clean)
		noiseRegex := regexp.MustCompile(`(?i)(CASHIER|RECEIPT|DATE|TELP|HP|JL|ALAMAT|NPWP|===|ITEM|QTY|NO\.|CUSTOMER|WELCOME|PRINT|COMPUTER)`)

		if len(clean) > 3 && !noiseRegex.MatchString(upper) {
			merchant = strings.Split(upper, "/")[0]
			merchant = strings.Split(merchant, "\t")[0]
			merchant = strings.Split(merchant, " - ")[0]
			merchant = strings.TrimSpace(merchant)
			break
		}
	}

	// 2. DATE: Support format angka & teks
	reDateStandard := regexp.MustCompile(`\d{2}[./-]\d{2}[./-]\d{2,4}`)
	reDateText := regexp.MustCompile(`(?i)\d{1,2}\s+(January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{4}`)

	if match := reDateText.FindString(raw); match != "" {
		t, _ := time.Parse("02 January 2006", match)
		if !t.IsZero() {
			transactionDate = t
		}
	} else if match := reDateStandard.FindString(raw); match != "" {
		normalized := strings.NewReplacer("/", ".", "-", ".").Replace(match)
		layouts := []string{"02.01.06", "02.01.2006"}
		for _, l := range layouts {
			if t, err := time.Parse(l, normalized); err == nil {
				transactionDate = t
				break
			}
		}
	}

	// 3. AMOUNT: Strategi Keyword Prioritas
	keywordFound := false
	priorityKeys := []string{"TOTAL BELANJA", "GRAND TOTAL", "TOTAL TAGIHAN", "TOTAL BAYAR", "NET TOTAL", "TOTAL RP"}

	for _, key := range priorityKeys {
		for i, line := range lines {
			if strings.Contains(strings.ToUpper(line), key) {
				match := reMoney.FindString(line)
				if match == "" && i+1 < len(lines) {
					match = reMoney.FindString(lines[i+1])
				}
				if match != "" {
					cleanNum := strings.NewReplacer(".", "", ",", "").Replace(match)
					val, _ := strconv.ParseFloat(cleanNum, 64)
					if val > 0 {
						amount = val
						keywordFound = true
						break
					}
				}
			}
		}
		if keywordFound {
			break
		}
	}

	if !keywordFound {
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.Contains(strings.ToUpper(lines[i]), "TOTAL") {
				match := reMoney.FindString(lines[i])
				if match == "" && i+1 < len(lines) {
					match = reMoney.FindString(lines[i+1])
				}
				if match != "" {
					cleanNum := strings.NewReplacer(".", "", ",", "").Replace(match)
					val, _ := strconv.ParseFloat(cleanNum, 64)
					if val > 0 {
						amount = val
						keywordFound = true
						break
					}
				}
			}
		}
	}

	if !keywordFound {
		allMatches := reMoney.FindAllString(raw, -1)
		for _, m := range allMatches {
			cleanNum := strings.NewReplacer(".", "", ",", "").Replace(m)
			num, _ := strconv.ParseFloat(cleanNum, 64)
			if num > amount && num < 2000000 {
				amount = num
			}
		}
	}

	// --- 4. ITEMS: Extract detail barang (Fix Multi-item & Quantity) ---
	for i, line := range lines {
		upperLine := strings.ToUpper(line)

		// Skip baris sampah
		skipRegex := regexp.MustCompile(`(?i)(TOTAL|BELANJA|NPWP|TELP|CASH|TUNAI|KEMBALI|===|---|SUBTOTAL|PB1|TAX|PAJAK|DISC|ITEM|QTY|POWERED|BY)`)
		if skipRegex.MatchString(upperLine) || len(strings.TrimSpace(line)) < 3 {
			continue
		}

		// 1. Deteksi apakah ada dua harga di satu baris (OCR sering gabungin baris)
		// Contoh: "1x Nugget Rp12.000 1x Kentang Rp12.000"
		matches := reMoney.FindAllString(line, -1)

		// Kita olah per-baris, kalau ada lebih dari satu match harga, kita coba split
		if len(matches) > 0 {
			// Logic baru: Ambil harga terakhir di baris tersebut
			priceMatch := matches[len(matches)-1]
			cleanNum := strings.NewReplacer(".", "", ",", "", "Rp", "").Replace(priceMatch)
			price, _ := strconv.ParseFloat(cleanNum, 64)

			if price > 0 && price < amount {
				itemName := strings.TrimSpace(strings.Replace(line, priceMatch, "", 1))
				itemName = strings.ReplaceAll(itemName, "Rp", "")

				// 2. Logic Lookbehind (Pola Unggul Mart)
				isJustMath := regexp.MustCompile(`^[\d\sX@x,.*:-]*$`).MatchString(itemName)
				if (len(itemName) < 2 || isJustMath) && i > 0 {
					potentialName := strings.TrimSpace(lines[i-1])
					if len(potentialName) > 2 && !skipRegex.MatchString(potentialName) {
						itemName = potentialName
					}
				}

				// 3. Logic EXTRACTION QUANTITY (Pola Razz Coffee / Alfamart)
				// Cari angka sebelum 'x' atau 'X' (contoh: 3x, 1 x, 4x)
				qty := 1.0
				reQty := regexp.MustCompile(`(\d+)\s*[xX]`)
				qtyMatch := reQty.FindStringSubmatch(itemName)
				if len(qtyMatch) > 1 {
					qty, _ = strconv.ParseFloat(qtyMatch[1], 64)
					// Hapus tulisan "3x" dari nama barang biar bersih
					itemName = strings.TrimSpace(reQty.ReplaceAllString(itemName, ""))
				}

				// 4. Final Cleaning
				itemName = regexp.MustCompile(`[:@\t]`).ReplaceAllString(itemName, " ")
				itemName = regexp.MustCompile(`\s+`).ReplaceAllString(itemName, " ")
				itemName = strings.TrimSpace(itemName)

				if len(itemName) > 2 {
					items = append(items, models.TransactionItem{
						Description: itemName,
						Quantity:    int(qty), // Sekarang dapet Qty asli (3, 4, dst)
						Price:       price,
						Total:       price * qty, // Total per item
					})
				}
			}
		}
	}

	return merchant, amount, transactionDate, items
}

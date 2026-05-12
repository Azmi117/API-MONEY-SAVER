package usecase

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	CreateManual(ctx context.Context, userID uint, req dto.CreateTransactionRequest) (string, error)
	ProcessScan(ctx context.Context, userID uint, workspaceID uint, imgData []byte, mimeType string) (*models.Transaction, error)
	ConfirmTransaction(ctx context.Context, id uint) (string, error)
	SyncGmailTransactions(ctx context.Context) error
	ProcessEmailMandiri(ctx context.Context, userID uint, workspaceID uint, subject string, body string) (*models.Transaction, error)
	GetHistory(workspaceID uint) ([]models.Transaction, error)
	DeleteTransaction(transactionID uint) error
	ProcessScanHybrid2(ctx context.Context, userID uint, workspaceID uint, imgData []byte, mimeType string) (*dto.ProcessScanHybridResult, error)
	HardDeleteTransaction(id uint) error
	ApproveEmailLog(ctx context.Context, logID uint, userID uint) error
	RejectEmailLog(ctx context.Context, logID uint, userID uint) error
	GetPendingEmailLogs(userID uint) ([]models.EmailParsed, error)
	ProcessScanAlternative(ctx context.Context, imagePath string, userID uint, workspaceID uint) (*dto.ProcessScanHybridResult, uint, error)
	ConfirmScanTransaction(ctx context.Context, txData *models.Transaction, items []models.TransactionItem) error
	ConfirmPendingTransaction(ctx context.Context, pendingID uint) (string, error)
	CheckWorkspaceTarget(workspaceID uint) (string, error)
	ConfirmEmailTransaction(ctx context.Context, userID uint, req dto.ConfirmEmailRequest) (string, error)
	ProcessTelegramInput(ctx context.Context, msg string) (string, bool, float64)
	AssignSplitBill(ctx context.Context, transactionID uint, items []dto.SplitItemRequest) error
	CreatePendingSplit(ctx context.Context, userID uint, workspaceID uint, imagePath string) (uint, error)
}

type transactionUsecase struct {
	repo           repository.TransactionRepository
	authRepo       repository.AuthRepository
	debtRepo       repository.DebtRepository
	debtUsecase    DebtUsecase
	targetRepo     repository.TargetRepository
	pendingRepo    repository.PendingTransactionRepository
	googleService  service.GoogleAuthService
	geminiClient   *gemini.GeminiClient
	hybridScanner  *ocr.HybridScanner
	wsRepo         repository.WorkspaceRepository
	ocrSpaceClient *ocr.OCRSpaceClient
	categoryRepo   repository.CategoryRepository
}

func NewTransactionUsecase(repo repository.TransactionRepository, authRepo repository.AuthRepository, googleService service.GoogleAuthService, gemini *gemini.GeminiClient, hybridScanner *ocr.HybridScanner, wsRepo repository.WorkspaceRepository, ocrSpace *ocr.OCRSpaceClient, pendingRepo repository.PendingTransactionRepository, targetRepo repository.TargetRepository, debtRepo repository.DebtRepository, debtUsecase DebtUsecase, categoryRepo repository.CategoryRepository) TransactionUsecase {
	return &transactionUsecase{
		repo:           repo,
		authRepo:       authRepo,
		googleService:  googleService,
		geminiClient:   gemini,
		hybridScanner:  hybridScanner,
		wsRepo:         wsRepo,
		ocrSpaceClient: ocrSpace,
		pendingRepo:    pendingRepo,
		targetRepo:     targetRepo,
		debtRepo:       debtRepo,
		debtUsecase:    debtUsecase,
		categoryRepo:   categoryRepo,
	}
}

func (u *transactionUsecase) CreateManual(ctx context.Context, userID uint, req dto.CreateTransactionRequest) (string, error) {
	// 1. Standarisasi nama merchant menjadi lowercase dan hapus spasi berlebih
	cleanMerchant := strings.ToLower(strings.TrimSpace(req.Merchant))

	// 2. Validasi Duplikasi: Cegah input double untuk data yang sama di hari yang sama
	isDuplicate, err := u.repo.IsDuplicate(req.WorkspaceID, req.Amount, cleanMerchant, req.Date)
	if err != nil {
		return "", apperror.Internal("Failed to check transaction duplicates!")
	}
	if isDuplicate {
		return "", apperror.Conflict("Similar transaction has already been recorded!")
	}

	var categoryID uint
	if req.CategoryID != nil && *req.CategoryID != 0 {
		// Kita ambil nilai asli dari pointer-nya
		categoryID = *req.CategoryID

		cat, err := u.categoryRepo.FindByID(categoryID)
		if err != nil || cat == nil || cat.WorkspaceID != req.WorkspaceID {
			return "", errors.New("kategori gak valid buat workspace ini, Mi")
		}
	}

	// 3. Mapping data dari DTO Request ke Model Database
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
		Status:      "approved", // Manual chat langsung dianggap approved
		GmailID:     req.GmailID,
	}

	// 4. Validasi Otoritas: Pastikan user yang input adalah member dari workspace tersebut
	isMember, err := u.wsRepo.IsMember(req.WorkspaceID, userID)
	if err != nil {
		return "", err
	}
	if !isMember {
		return "", errors.New("akses ditolak: lu bukan member di workspace ini")
	}

	// 5. Simpan transaksi ke database utama
	if err := u.repo.Create(tx); err != nil {
		return "", apperror.Internal("Failed to save manual transaction!")
	}

	// --- LOGIC THE GUARDIAN: UPDATE PROGRESS SAVING ---
	// Jika tipe transaksi adalah income, kita update progres target di workspace ini
	if tx.Type == "income" {
		month := time.Now().Format("2006-01")
		target, err := u.targetRepo.GetByWorkspaceAndPeriod(tx.WorkspaceID, month)

		if err == nil && target != nil {
			// Tambahkan nominal income ke current_amount target
			target.CurrentAmount += tx.Amount
			// Update data target di database
			u.targetRepo.Update(target)
		}
	}

	// 6. LOGIC THE GUARDIAN: Hitung akumulasi budget bulanan setelah transaksi disimpan
	notification, err := u.CheckWorkspaceTarget(req.WorkspaceID)
	if err != nil {
		log.Printf("Target Check Error: %v", err)
		return "Transaksi dicatat!", nil
	}

	// Jika pengeluaran melebihi limit atau tabungan bertambah, kembalikan pesannya
	if notification != "" {
		return fmt.Sprintf("Sip! Udah dicatat ya. \n\n%s", notification), nil
	}

	return "Transaksi berhasil dicatat!", nil
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
				UserID: user.ID,
				// RawEmail:   bodyStr,
				RawEmail:   "HTML Content Truncated",
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

func (u *transactionUsecase) ConfirmTransaction(ctx context.Context, id uint) (string, error) {
	// 1. Cari data transaksi di database
	var tx models.Transaction
	if err := u.repo.FindByID(&tx, id); err != nil {
		return "", err
	}

	// 2. Update status transaksi menjadi approved
	if err := u.repo.UpdateStatus(id, "approved"); err != nil {
		return "", err
	}

	// 3. LOGIC THE GUARDIAN: Cek sisa budget atau progres tabungan
	notification, err := u.CheckWorkspaceTarget(tx.WorkspaceID)
	if err != nil {
		// Log error tapi transaksi tetap dianggap sukses dikonfirmasi
		log.Printf("Target Check Error (Hybrid): %v", err)
		return "Transaksi berhasil dikonfirmasi!", nil
	}

	if notification != "" {
		return fmt.Sprintf("Transaksi Berhasil! \n\n%s", notification), nil
	}

	return "Transaksi berhasil dikonfirmasi!", nil
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

func (u *transactionUsecase) ProcessScanAlternative(ctx context.Context, imagePath string, userID uint, workspaceID uint) (*dto.ProcessScanHybridResult, uint, error) {
	// 1. Ambil data user buat cek limit (Sesuai sistem OCRUsageCount lu)
	user, err := u.authRepo.FindByID(userID)
	if err != nil {
		return nil, 0, err
	}

	// 2. Cek Limit berdasarkan Tier (Logic yang lu pake di hybrid2)
	limit := 2 // Default Free
	if user.AccountTier == "pro" {
		limit = 10
	} else if user.AccountTier == "ultimate" {
		limit = 100
	}

	if user.OCRUsageCount >= limit {
		return nil, 0, apperror.Forbidden("Weekly scan limit reached for your tier")
	}

	// 3. Ekstrak Teks via OCR Space
	rawText, err := u.ocrSpaceClient.ExtractRawText(imagePath)
	if err != nil {
		return nil, 0, err
	}

	// 4. Parse secara manual (The Beast Version)
	merchant, amount, transactionDate, items := u.manualParserV3(rawText)

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

	// Simpan ke tabel PendingTransaction
	jsonData, _ := json.Marshal(transaction)
	pending := &models.PendingTransaction{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Source:      "telegram_alt",
		RawData:     string(jsonData),
		Status:      "pending",
	}

	err = u.pendingRepo.Create(pending)
	if err != nil {
		return nil, 0, err // Sekarang ini sudah benar karena signature fungsinya minta 3 return
	}

	_ = u.authRepo.IncrementOCRUsage(userID)

	return &dto.ProcessScanHybridResult{
		Transaction: transaction,
		Engine:      "OCR_SPACE_PURE",
	}, pending.ID, nil
}

func (u *transactionUsecase) ConfirmScanTransaction(ctx context.Context, txData *models.Transaction, items []models.TransactionItem) error {
	// 1. Set relasi items ke header transaction
	txData.TransactionItems = items

	// 2. Simpan ke Database (Sekali jalan karena sudah ada relasi di struct)
	// Ini bakal nge-save ke tabel transactions DAN transaction_items otomatis via GORM
	return u.repo.CreateWithItems(txData)
}

func (u *transactionUsecase) ConfirmPendingTransaction(ctx context.Context, pendingID uint) (string, error) {
	// 1. Ambil data dari tabel penampung (pending_transactions) berdasarkan ID
	pending, err := u.pendingRepo.FindByID(pendingID)
	if err != nil {
		return "", err
	}

	// 2. Decode JSON mentah dari field RawData ke struct Transaction
	var txData models.Transaction
	if err := json.Unmarshal([]byte(pending.RawData), &txData); err != nil {
		return "", err
	}

	// 3. Set status transaksi menjadi "approved" agar masuk sebagai transaksi resmi
	txData.Status = "approved"

	// --- FIX LOGIC: VALIDASI KATEGORI (Pointer Handling) ---
	// Cek apakah pointernya ada isinya (tidak nil) dan isinya bukan 0
	if txData.CategoryID != nil && *txData.CategoryID != 0 {
		cat, err := u.categoryRepo.FindByID(*txData.CategoryID)
		if err != nil || cat == nil || cat.WorkspaceID != txData.WorkspaceID {
			return "", errors.New("kategori yang dipilih gak valid buat workspace ini, Mi")
		}
	}

	// 4. Pindahkan data ke tabel transaksi permanen
	err = u.ConfirmScanTransaction(ctx, &txData, txData.TransactionItems)
	if err != nil {
		return "", err
	}

	// 5. Update status di tabel pending menjadi "approved"
	if err := u.pendingRepo.UpdateStatus(pendingID, "approved"); err != nil {
		return "", err
	}

	// --- LOGIC THE GUARDIAN: UPDATE PROGRESS SAVING ---
	if txData.Type == "income" {
		month := time.Now().Format("2006-01")
		target, err := u.targetRepo.GetByWorkspaceAndPeriod(txData.WorkspaceID, month)

		if err == nil && target != nil {
			target.CurrentAmount += txData.Amount
			u.targetRepo.Update(target)
		}
	}

	// 6. LOGIC THE GUARDIAN: Check Target & Limit
	notification, err := u.CheckWorkspaceTarget(txData.WorkspaceID)
	if err != nil {
		log.Printf("Target Check Error: %v", err)
		return "Transaksi berhasil dikonfirmasi!", nil
	}

	if notification != "" {
		return fmt.Sprintf("Transaksi Berhasil! \n\n%s", notification), nil
	}

	return "Transaksi berhasil dikonfirmasi!", nil
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

func (u *transactionUsecase) CheckWorkspaceTarget(workspaceID uint) (string, error) {
	month := time.Now().Format("2006-01")

	// 1. Ambil data target bulan ini (Bukan dari model Workspace langsung)
	target, err := u.targetRepo.GetByWorkspaceAndPeriod(workspaceID, month)
	if err != nil {
		// Kalau target belum diset, kita kasih default 0 atau error handling
		return "⚠️ Target untuk bulan ini belum diatur, Mi! Pakai command set target dulu.", nil
	}

	// 2. Ambil Total Global (Expense & Savings)
	totalExpense, _ := u.repo.GetTotalByWorkspace(workspaceID, "expense", month)

	// 3. Ambil Rincian Per Member
	expenseSummaries, _ := u.repo.GetSummaryByWorkspace(workspaceID, "expense", month)
	savingsSummaries, _ := u.repo.GetSummaryByWorkspace(workspaceID, "savings", month)

	// 4. Rangkai Pesan
	res := fmt.Sprintf("📊 *Status Budget Bulan %s*\n\n", month)

	// Bagian Expense
	res += "💸 *Limit (Expense):*\n"
	res += fmt.Sprintf("🚨 Rp%.2f / Rp%.2f\n", totalExpense, target.AmountLimit)
	res += fmt.Sprintf("Sisa: Rp%.2f\n\n", target.AmountLimit-totalExpense)

	if len(expenseSummaries) > 0 {
		res += "👤 *Rincian :*\n"
		for _, s := range expenseSummaries {
			res += fmt.Sprintf("- %s: Rp%.2f\n", s.UserName, s.Total)
		}
		res += "\n"
	}

	// Bagian Savings
	res += "💰 *Progres Tabungan (Savings):*\n"
	res += fmt.Sprintf("📈 Rp%.2f / Rp%.2f\n\n", target.CurrentAmount, target.SavingsTarget)

	if len(savingsSummaries) > 0 {
		res += "👤 *Rincian Nabung:*\n"
		for _, s := range savingsSummaries {
			res += fmt.Sprintf("- %s: Rp%.2f\n", s.UserName, s.Total)
		}
	}

	return res, nil
}

func (u *transactionUsecase) ConfirmEmailTransaction(ctx context.Context, userID uint, req dto.ConfirmEmailRequest) (string, error) {
	// 1. Ambil data mentah dari table email_parsed
	emailData, err := u.repo.GetEmailParsedByID(req.EmailParsedID)
	if err != nil {
		return "", err
	}

	// 2. Mapping ke Model Transaction Utama
	// Langsung pake emailData.ParsedDate karena tipenya udah sama-sama time.Time
	newTransaction := &models.Transaction{
		UserID:      userID,
		WorkspaceID: req.WorkspaceID, // ID pilihan user dari cegatan
		Amount:      emailData.Amount,
		Merchant:    emailData.Merchant,
		Date:        emailData.ParsedDate, // Langsung pasang disini
		Type:        "expense",
		Source:      "Email",
		Status:      "approved",
		GmailID:     emailData.GmailID, // Pastiin G dan ID kapital[cite: 1]
	}

	// 3. Simpan ke database table transactions
	if err := u.repo.Create(newTransaction); err != nil {
		return "", err
	}

	// 4. Hapus data draf di table email_parsed (Ruang Tunggu)
	_ = u.repo.DeleteEmailParsed(req.EmailParsedID)

	// 5. Jalankan The Guardian (Cek limit budget)
	notification, _ := u.CheckWorkspaceTarget(req.WorkspaceID)

	return notification, nil
}

func (u *transactionUsecase) ProcessTelegramInput(ctx context.Context, msg string) (string, bool, float64) {
	// 1. Cek simbol di awal atau di akhir pesan
	isIncome := strings.Contains(msg, "+")
	isExpense := strings.Contains(msg, "-")

	// 2. Ekstrak angka (pake regex lu yang lama)
	amount := u.extractAmount(msg)

	// 3. Logic Hybrid
	if isIncome {
		return "income", true, amount
	}
	if isExpense {
		return "expense", true, amount
	}

	// 4. Kalau gak ada tanda, kembalikan false buat memicu munculnya button
	return "", false, amount
}

func (u *transactionUsecase) extractAmount(msg string) float64 {
	re := regexp.MustCompile(`(\d+)`)
	match := re.FindString(msg)

	// Cek apakah string-nya tidak kosong, bukan dibandingin sama nil
	if match != "" {
		amount, _ := strconv.ParseFloat(match, 64)
		return amount
	}
	return 0
}

func (u *transactionUsecase) AssignSplitBill(ctx context.Context, transactionID uint, items []dto.SplitItemRequest) error {
	// 1. Ambil data asli dari DB (Sebagai acuan/polisi)
	var originalTx models.Transaction
	if err := u.repo.FindByID(&originalTx, transactionID); err != nil {
		return fmt.Errorf("transaksi dengan ID %d kagak ketemu, Mi", transactionID)
	}

	// Buat map quantity dari struk asli
	originalQtyMap := make(map[string]int)
	for _, actualItem := range originalTx.TransactionItems {
		name := strings.TrimSpace(actualItem.Description)
		originalQtyMap[name] = actualItem.Quantity
	}

	// --- LAYER VALIDASI: QUANTITY & TOTAL NOMINAL ---
	var totalInputAmount float64
	inputQtyMap := make(map[string]int)
	userDebts := make(map[uint]float64)

	for _, input := range items {
		itemName := strings.TrimSpace(input.ItemName)

		// 1. Akumulasi Quantity per item
		inputQtyMap[itemName] += input.Quantity

		// 2. Akumulasi Total Nominal Input (Logic Baru sesuai ide lu)
		totalInputAmount += (input.Price * float64(input.Quantity))

		// 3. Cek apakah item ada di struk
		qtyInDB, exists := originalQtyMap[itemName]
		if !exists {
			return fmt.Errorf("item '%s' gak ada di struk asli", itemName)
		}

		// 4. Validasi Quantity (Satpam Quantity)
		if inputQtyMap[itemName] > qtyInDB {
			return fmt.Errorf("item '%s' kelebihan: di struk cuma %d, tapi lu tag total %d",
				itemName, qtyInDB, inputQtyMap[itemName])
		}

		// 5. Kelompokkan nominal utang (skip kalau yang bayar diri sendiri)
		if input.UserID != originalTx.UserID {
			userDebts[input.UserID] += (input.Price * float64(input.Quantity))
		}
	}

	// --- FINAL LAYER: CEK TOTAL DUIT (Satpam Financial) ---
	// Kita kasih toleransi 1 perak buat jaga-jaga pembulatan float64
	if totalInputAmount > (originalTx.Amount + 1) {
		return fmt.Errorf("total tagihan split (Rp%.2f) melebihi total struk asli (Rp%.2f)!",
			totalInputAmount, originalTx.Amount)
	}

	// 6. Simpan ke tabel Debts secara batch
	if len(userDebts) > 0 {
		var debtsToSave []models.Debt
		for targetUserID, totalAmount := range userDebts {

			// --- TAMBAHKAN INI ---
			// Panggil generator kode unik 4 karakter
			shortCode, err := u.debtUsecase.GenerateUniqueShortCode(originalTx.WorkspaceID)
			if err != nil {
				// Fallback kalau amit-amit generatornya error
				shortCode = "ERR1"
			}
			// ---------------------

			debtsToSave = append(debtsToSave, models.Debt{
				WorkspaceID: originalTx.WorkspaceID,
				FromUserID:  targetUserID,
				ToUserID:    originalTx.UserID,
				Amount:      totalAmount,
				ShortCode:   shortCode, // <-- MASUKIN KE SINI
				Note:        "Split bill dari merchant: " + originalTx.Merchant,
				IsPaid:      false,
			})
		}
		return u.debtRepo.CreateInBatch(debtsToSave)
	}

	return nil
}

func (u *transactionUsecase) CreatePendingSplit(ctx context.Context, userID uint, workspaceID uint, imagePath string) (uint, error) {
	pendingTx := &models.PendingTransaction{
		UserID:      userID,
		WorkspaceID: workspaceID,
		ImagePath:   imagePath,
		Status:      "pending",
		Source:      "telegram_split",
		// Tambahin ini Mi, biar Postgres gak protes soal syntax JSON
		RawData:    "{}",
		RawOCRData: "{}",
	}

	err := u.pendingRepo.Create(pendingTx)
	if err != nil {
		return 0, err
	}

	return pendingTx.ID, nil
}

package usecase

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/service"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/gemini"
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
}

type transactionUsecase struct {
	repo          repository.TransactionRepository
	authRepo      repository.AuthRepository
	googleService service.GoogleAuthService
	geminiClient  *gemini.GeminiClient
}

func NewTransactionUsecase(repo repository.TransactionRepository, authRepo repository.AuthRepository, googleService service.GoogleAuthService, gemini *gemini.GeminiClient) TransactionUsecase {
	return &transactionUsecase{
		repo:          repo,
		authRepo:      authRepo,
		googleService: googleService,
		geminiClient:  gemini,
	}
}

// 1. Logic buat input manual
func (u *transactionUsecase) CreateManual(ctx context.Context, userID uint, req dto.CreateTransactionRequest) error {
	cleanMerchant := strings.ToLower(strings.TrimSpace(req.Merchant))
	isDuplicate, err := u.repo.IsDuplicate(req.WorkspaceID, req.Amount, cleanMerchant, req.Date)
	if err != nil {
		return err
	}
	if isDuplicate {
		return fmt.Errorf("transaksi serupa sudah tercatat (duplikat merchant: %s)", cleanMerchant)
	}

	tx := &models.Transaction{
		UserID:      userID,
		WorkspaceID: req.WorkspaceID,
		CategoryID:  &req.CategoryID,
		Amount:      req.Amount,
		Type:        req.Type,
		Date:        req.Date,
		Note:        req.Note,
		Merchant:    cleanMerchant,
		Source:      req.Source,
		Status:      "approved",
	}

	return u.repo.Create(tx)
}

// 2. Logic buat scan struk pake Gemini
func (u *transactionUsecase) ProcessScan(ctx context.Context, userID uint, workspaceID uint, imgData []byte, mimeType string) (*models.Transaction, error) {
	// 1. Panggil Gemini buat scan
	result, err := u.geminiClient.ScanReceipt(ctx, imgData, mimeType)
	if err != nil {
		return nil, err
	}

	cleanMerchant := strings.Title(strings.ToLower(strings.TrimSpace(result.Merchant)))

	parsedDate, err := time.Parse("2006-01-02 15:04:05", result.Date)
	if err != nil {
		parsedDate = time.Now()
	}

	// 2. Siapkan struct Transaction utama
	tx := &models.Transaction{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Amount:      result.Amount,
		Merchant:    cleanMerchant,
		Method:      result.Method,
		Note:        result.Note,
		Date:        parsedDate,
		Type:        result.Type,
		Source:      "scan",
		Status:      "pending",
		// Inisialisasi slice buat nampung item
		TransactionItems: []models.TransactionItem{},
	}

	// 3. Mapping Item dari Gemini (ResultScan) ke Model Database (TransactionItem)
	for _, item := range result.Items {
		tx.TransactionItems = append(tx.TransactionItems, models.TransactionItem{
			Description: item.Description,
			Quantity:    item.Quantity,
			Price:       item.UnitPrice,
			Total:       item.Total,
		})
	}

	// 4. Simpan ke Repo.
	// Karena lo pake GORM db.Create(tx), GORM bakal otomatis
	// simpan Transaction dulu, ambil ID-nya, lalu simpan semua TransactionItems.
	if err := u.repo.Create(tx); err != nil {
		return nil, fmt.Errorf("gagal simpan draft scan beserta item: %v", err)
	}

	return tx, nil
}

// 3. Sync Gmail (THE ROBOT)
func (u *transactionUsecase) SyncGmailTransactions(ctx context.Context) error {
	users, err := u.authRepo.FindAllWithGmail()
	if err != nil {
		return err
	}

	for _, user := range users {
		srv, err := u.googleService.GetGmailService(user.GoogleRefreshToken)
		if err != nil {
			fmt.Printf("Gagal dapet Gmail service buat user %d: %v\n", user.ID, err)
			continue
		}

		query := fmt.Sprintf("(from:no-reply@bankmandiri.co.id OR from:noreply.livin@bankmandiri.co.id) after:%s",
			time.Now().AddDate(0, 0, -1).Format("2006/01/02"))

		res, err := srv.Users.Messages.List("me").Q(query).Do()
		if err != nil || len(res.Messages) == 0 {
			continue
		}

		for _, m := range res.Messages {
			existing, _ := u.repo.GetByGmailID(m.Id)
			if existing != nil && existing.ID != 0 {
				continue
			}

			fullMsg, err := srv.Users.Messages.Get("me", m.Id).Do()
			if err != nil {
				continue
			}

			// Get Subject
			var subject string
			for _, h := range fullMsg.Payload.Headers {
				if h.Name == "Subject" {
					subject = h.Value
				}
			}

			// --- FIX: Logic Ambil Body yang Lebih Sakti ---
			bodyStr := u.getBody(fullMsg.Payload)

			// DEBUG: Aktifkan ini kalau mau liat isi email asli di terminal
			// fmt.Printf("\n--- BODY EMAIL (%s) ---\n%s\n------------------\n", m.Id, bodyStr)

			parsed := utils.ParseMandiriEmail(subject, bodyStr)
			if parsed != nil {
				tx := &models.Transaction{
					UserID:      user.ID,
					WorkspaceID: 1, // Workspace default
					Amount:      parsed.Amount,
					Merchant:    parsed.Merchant, // Nama Penerima (Diva Puti, etc)
					Method:      parsed.Method,   // Transfer / QRIS / Top-up
					Note:        parsed.Note,     // Pesan/Keterangan asli
					Date:        parsed.Date,
					Type:        "expense",
					Source:      "email_auto",
					Status:      "pending",
					GmailID:     m.Id,
				}

				if err := u.repo.Create(tx); err != nil {
					fmt.Printf("Gagal simpan transaksi: %v\n", err)
					continue
				}

				// Log terminal yang lebih informatif buat lo pantau
				fmt.Printf("✅ Berhasil Sync: [%s] Ke: %s | Rp%.2f | Pesan: %s\n",
					parsed.Method, parsed.Merchant, parsed.Amount, parsed.Note)
			}
		}
	}
	return nil
}

// Helper untuk bongkar body email Gmail yang berlapis-lapis
func (u *transactionUsecase) getBody(payload *gmail.MessagePart) string {
	data := ""
	if payload.Body.Data != "" {
		data = payload.Body.Data
	} else {
		// Cari di dalam Parts kalau body utama kosong
		for _, part := range payload.Parts {
			if part.MimeType == "text/plain" || part.MimeType == "text/html" {
				data = part.Body.Data
				break
			}
			// Kalau masih nested, rekursif tipis-tipis
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
		return nil, fmt.Errorf("email bukan format transaksi Mandiri yang didukung")
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
		return nil, err
	}
	return tx, nil
}

func (u *transactionUsecase) ConfirmTransaction(ctx context.Context, transactionID uint) error {
	return u.repo.UpdateStatus(transactionID, "approved")
}

func (u *transactionUsecase) GetHistory(workspaceID uint) ([]models.Transaction, error) {
	return u.repo.GetByWorkspaceID(workspaceID)
}

func (u *transactionUsecase) DeleteTransaction(transactionID uint) error {
	return u.repo.Delete(transactionID)
}

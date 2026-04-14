package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/gemini"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/utils"
)

type TransactionUsecase interface {
	CreateManual(ctx context.Context, userID uint, req dto.CreateTransactionRequest) error
	ProcessScan(ctx context.Context, userID uint, workspaceID uint, imgData []byte, mimeType string) (*models.Transaction, error)
	ConfirmTransaction(ctx context.Context, transactionID uint) error
	ProcessEmailMandiri(ctx context.Context, userID uint, workspaceID uint, subject string, body string) (*models.Transaction, error)
	GetHistory(workspaceID uint) ([]models.Transaction, error)
	DeleteTransaction(transactionID uint) error
}

type transactionUsecase struct {
	repo         repository.TransactionRepository
	geminiClient *gemini.GeminiClient
}

func NewTransactionUsecase(repo repository.TransactionRepository, gemini *gemini.GeminiClient) TransactionUsecase {
	return &transactionUsecase{
		repo:         repo,
		geminiClient: gemini,
	}
}

// 1. Logic buat input manual
func (u *transactionUsecase) CreateManual(ctx context.Context, userID uint, req dto.CreateTransactionRequest) error {
	// KUNCI: Kita lowercase merchant sebelum diproses
	cleanMerchant := strings.ToLower(strings.TrimSpace(req.Merchant))

	// Cek duplikat pake Satpam Repository
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
		Merchant:    cleanMerchant, // Simpan yang sudah bersih
		Source:      req.Source,
		Status:      "approved",
	}

	return u.repo.Create(tx)
}

// 2. Logic buat scan struk pake Gemini
func (u *transactionUsecase) ProcessScan(ctx context.Context, userID uint, workspaceID uint, imgData []byte, mimeType string) (*models.Transaction, error) {
	result, err := u.geminiClient.ScanReceipt(ctx, imgData, mimeType)
	if err != nil {
		return nil, err
	}

	cleanMerchant := strings.ToLower(strings.TrimSpace(result.Merchant))
	parsedDate, _ := time.Parse("2006-01-02 15:04:05", result.Date)

	tx := &models.Transaction{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Amount:      result.Amount,
		Merchant:    cleanMerchant,
		Date:        parsedDate,
		Type:        result.Type,
		Source:      "scan", // Kita kasih label 'scan' biar beda sama input manual
		Status:      "pending",
		// PENTING: Kasih CategoryID default (misal 1 untuk 'Uncategorized')
		// Karena di model lu itu 'not null'. Kalau kosong, DB bakal nolak.
	}

	// Simpan dulu ke DB sebagai draft (status pending)
	if err := u.repo.Create(tx); err != nil {
		return nil, fmt.Errorf("gagal simpan draft scan: %v", err)
	}

	return tx, nil
}

// 3. TAMBAHAN: Logic buat parsing email Mandiri (Budget Hemat)
func (u *transactionUsecase) ProcessEmailMandiri(ctx context.Context, userID uint, workspaceID uint, subject string, body string) (*models.Transaction, error) {
	// Panggil parser buatan kita tadi
	parsed := utils.ParseMandiriEmail(subject, body)
	if parsed == nil {
		return nil, fmt.Errorf("email bukan format transaksi Mandiri yang didukung")
	}

	// Cek duplikat (Satpam main aman)
	isDuplicate, _ := u.repo.IsDuplicate(workspaceID, parsed.Amount, parsed.Merchant, parsed.Date)
	if isDuplicate {
		return nil, fmt.Errorf("transaksi dari email ini sudah tercatat")
	}

	tx := &models.Transaction{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Amount:      parsed.Amount,
		Merchant:    parsed.Merchant,
		Date:        parsed.Date,
		Type:        "expense", // Default email Mandiri biasanya pengeluaran
		Source:      "email",
		Status:      "pending",
	}

	if err := u.repo.Create(tx); err != nil {
		return nil, err
	}

	return tx, nil
}

func (u *transactionUsecase) ConfirmTransaction(ctx context.Context, transactionID uint) error {
	// Di sini kita butuh fungsi UpdateStatus di repository nanti
	return u.repo.UpdateStatus(transactionID, "approved")
}

func (u *transactionUsecase) GetHistory(workspaceID uint) ([]models.Transaction, error) {
	return u.repo.GetByWorkspaceID(workspaceID)
}

func (u *transactionUsecase) DeleteTransaction(transactionID uint) error {
	// Di sini lu bisa tambahin validasi kalau mau, tapi buat MVP langsung delete aja
	return u.repo.Delete(transactionID)
}

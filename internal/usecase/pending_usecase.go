package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
)

type PendingUsecase interface {
	CreatePendingSplit(ctx context.Context, userID uint, workspaceID uint, imagePath string) (uint, error)
	ConfirmPendingTransaction(ctx context.Context, pendingID uint) (*dto.BudgetStatusResponse, error)
	GetPendingEmailLogs(userID uint) ([]models.EmailParsed, error)
	ApproveEmailLog(ctx context.Context, logID uint, userID uint) error
	RejectEmailLog(ctx context.Context, logID uint, userID uint) error
	ConfirmEmailTransaction(ctx context.Context, userID uint, req dto.ConfirmEmailRequest) (*dto.BudgetStatusResponse, error) // UPDATE: return DTO
}

type pendingUsecase struct {
	pendingRepo   repository.PendingTransactionRepository
	txRepo        repository.TransactionRepository
	categoryRepo  repository.CategoryRepository
	targetUsecase TargetUsecase
	txUsecase     TransactionUsecase // Inject Usecase Utama
}

func NewPendingUsecase(
	pendingRepo repository.PendingTransactionRepository,
	txRepo repository.TransactionRepository,
	categoryRepo repository.CategoryRepository,
	targetUsecase TargetUsecase,
	txUsecase TransactionUsecase,
) PendingUsecase {
	return &pendingUsecase{
		pendingRepo:   pendingRepo,
		txRepo:        txRepo,
		categoryRepo:  categoryRepo,
		targetUsecase: targetUsecase,
		txUsecase:     txUsecase,
	}
}

// ---------------------------------------------------------
// 1. PENDING SCAN OCR & SPLIT BILL
// ---------------------------------------------------------
func (u *pendingUsecase) CreatePendingSplit(ctx context.Context, userID uint, workspaceID uint, imagePath string) (uint, error) {
	pendingTx := &models.PendingTransaction{
		UserID:      userID,
		WorkspaceID: workspaceID,
		ImagePath:   imagePath,
		Status:      "pending",
		Source:      "telegram_split",
		RawData:     "{}",
		RawOCRData:  "{}",
	}

	if err := u.pendingRepo.Create(pendingTx); err != nil {
		return 0, err
	}

	return pendingTx.ID, nil
}

func (u *pendingUsecase) ConfirmPendingTransaction(ctx context.Context, pendingID uint) (*dto.BudgetStatusResponse, error) {
	pending, err := u.pendingRepo.FindByID(pendingID)
	if err != nil {
		return nil, err
	}

	var txData models.Transaction
	if err := json.Unmarshal([]byte(pending.RawData), &txData); err != nil {
		return nil, err
	}

	txData.Status = "approved"

	if txData.CategoryID != nil && *txData.CategoryID != 0 {
		cat, err := u.categoryRepo.FindByID(*txData.CategoryID)
		if err != nil || cat == nil || cat.WorkspaceID != txData.WorkspaceID {
			return nil, errors.New("invalid category for this workspace")
		}
	}

	// FIX: Sekarang nampung 2 return value (budgetData, err)
	budgetData, err := u.txUsecase.ConfirmScanTransaction(ctx, &txData, txData.TransactionItems)
	if err != nil {
		return nil, err
	}

	if err := u.pendingRepo.UpdateStatus(pendingID, "approved"); err != nil {
		return nil, err
	}

	// Logic update income ke target
	if txData.Type == "income" {
		_ = u.targetUsecase.AddIncomeToTarget(txData.WorkspaceID, txData.Amount)
	}

	return budgetData, nil
}

// ---------------------------------------------------------
// 2. PENDING EMAIL MANDIRI
// ---------------------------------------------------------
func (u *pendingUsecase) GetPendingEmailLogs(userID uint) ([]models.EmailParsed, error) {
	return u.txRepo.GetPendingEmailLogs(userID)
}

func (u *pendingUsecase) ApproveEmailLog(ctx context.Context, logID uint, userID uint) error {
	logData, err := u.txRepo.GetEmailLogByID(logID)
	if err != nil {
		return apperror.NotFound("Email log data not found")
	}

	if logData.UserID != userID {
		return apperror.Forbidden("You are not authorized to access this log")
	}

	if logData.Status != "Pending" {
		return apperror.BadRequest("This email log has already been processed")
	}

	tx := &models.Transaction{
		UserID:      logData.UserID,
		WorkspaceID: 1,
		Amount:      logData.Amount,
		Merchant:    logData.Merchant,
		Date:        logData.ParsedDate,
		Source:      "email_auto",
		Status:      "approved",
		Method:      logData.Method,
		Note:        logData.Note,
		Type:        "expense",
		GmailID:     logData.GmailID,
	}

	if err := u.txRepo.Create(tx); err != nil {
		return apperror.Internal("Failed to create transaction from email")
	}

	return u.txRepo.UpdateEmailLogStatus(logID, "Success")
}

func (u *pendingUsecase) RejectEmailLog(ctx context.Context, logID uint, userID uint) error {
	logData, err := u.txRepo.GetEmailLogByID(logID)
	if err != nil {
		return apperror.NotFound("Email log data not found")
	}

	if logData.UserID != userID {
		return apperror.Forbidden("Access denied")
	}

	return u.txRepo.UpdateEmailLogStatus(logID, "Rejected")
}

func (u *pendingUsecase) ConfirmEmailTransaction(ctx context.Context, userID uint, req dto.ConfirmEmailRequest) (*dto.BudgetStatusResponse, error) {
	emailData, err := u.txRepo.GetEmailParsedByID(req.EmailParsedID)
	if err != nil {
		return nil, err // FIX: return nil instead of ""
	}

	newTransaction := &models.Transaction{
		UserID:      userID,
		WorkspaceID: req.WorkspaceID,
		Amount:      emailData.Amount,
		Merchant:    emailData.Merchant,
		Date:        emailData.ParsedDate,
		Type:        "expense",
		Source:      "Email",
		Status:      "approved",
		GmailID:     emailData.GmailID,
	}

	if err := u.txRepo.Create(newTransaction); err != nil {
		return nil, err // FIX: return nil instead of ""
	}

	_ = u.txRepo.DeleteEmailParsed(req.EmailParsedID)

	// FIX: Sekarang return *dto.BudgetStatusResponse
	budgetData, err := u.targetUsecase.CheckWorkspaceTarget(req.WorkspaceID)
	if err != nil {
		log.Printf("Target Check Error during email confirmation: %v", err)
		return nil, nil // Tetap sukses meskipun budget check gagal
	}

	return budgetData, nil
}

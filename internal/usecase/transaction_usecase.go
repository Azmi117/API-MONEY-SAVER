package usecase

import (
	"context"
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
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/gemini"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/ocr"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/utils"
)

type TransactionUsecase interface {
	CreateManual(ctx context.Context, userID uint, req dto.CreateTransactionRequest) (*models.Transaction, *dto.BudgetStatusResponse, error)
	ConfirmTransaction(ctx context.Context, pendingID uint) (*models.Transaction, *dto.BudgetStatusResponse, error)
	GetHistory(workspaceID uint) ([]models.Transaction, error)
	DeleteTransaction(ctx context.Context, transactionID uint, userID uint) error
	ProcessScanHybrid2(ctx context.Context, userID uint, workspaceID uint, imgData []byte, mimeType string) (*dto.ProcessScanHybridResult, uint, error)
	HardDeleteTransaction(id uint) error
	ProcessScanAlternative(ctx context.Context, imagePath string, userID uint, workspaceID uint) (*dto.ProcessScanHybridResult, uint, error)
	ConfirmScanTransaction(ctx context.Context, tx *models.Transaction, items []models.TransactionItem) (*dto.BudgetStatusResponse, error)
	ProcessTelegramInput(ctx context.Context, msg string) (string, bool, float64)
}

type transactionUsecase struct {
	repo           repository.TransactionRepository
	authRepo       repository.AuthRepository
	debtRepo       repository.DebtRepository
	debtUsecase    DebtUsecase
	pendingRepo    repository.PendingTransactionRepository
	geminiClient   *gemini.GeminiClient
	hybridScanner  *ocr.HybridScanner
	wsRepo         repository.WorkspaceRepository
	ocrSpaceClient *ocr.OCRSpaceClient
	categoryRepo   repository.CategoryRepository
	targetUsecase  TargetUsecase
}

func NewTransactionUsecase(
	repo repository.TransactionRepository,
	authRepo repository.AuthRepository,
	gemini *gemini.GeminiClient,
	hybridScanner *ocr.HybridScanner,
	wsRepo repository.WorkspaceRepository,
	ocrSpace *ocr.OCRSpaceClient,
	pendingRepo repository.PendingTransactionRepository,
	categoryRepo repository.CategoryRepository,
	targetUsecase TargetUsecase,
) TransactionUsecase {
	return &transactionUsecase{
		repo:           repo,
		authRepo:       authRepo,
		geminiClient:   gemini,
		hybridScanner:  hybridScanner,
		wsRepo:         wsRepo,
		ocrSpaceClient: ocrSpace,
		pendingRepo:    pendingRepo,
		categoryRepo:   categoryRepo,
		targetUsecase:  targetUsecase,
	}
}

func (u *transactionUsecase) CreateManual(ctx context.Context, userID uint, req dto.CreateTransactionRequest) (*models.Transaction, *dto.BudgetStatusResponse, error) {
	cleanMerchant := strings.ToLower(strings.TrimSpace(req.Merchant))

	// 1. Validasi Akses Workspace
	isMember, err := u.wsRepo.IsMember(req.WorkspaceID, userID)
	if err != nil {
		return nil, nil, err
	}
	if !isMember {
		return nil, nil, errors.New("access denied: you are not a member of this workspace")
	}

	// 2. Cek Duplikat
	isDuplicate, err := u.repo.IsDuplicate(req.WorkspaceID, req.Amount, cleanMerchant, req.Date)
	if err != nil {
		return nil, nil, apperror.Internal("failed to check transaction duplicates")
	}
	if isDuplicate {
		return nil, nil, apperror.Conflict("similar transaction has already been recorded")
	}

	// 3. Validasi Category
	var categoryID uint
	if req.CategoryID != nil && *req.CategoryID != 0 {
		categoryID = *req.CategoryID
		cat, err := u.categoryRepo.FindByID(categoryID)
		if err != nil || cat == nil || cat.WorkspaceID != req.WorkspaceID {
			return nil, nil, errors.New("invalid category for this workspace")
		}
	}

	// 4. Handle GmailID biar gak duplikat constraint
	fakeGmailID := req.GmailID
	if fakeGmailID == "" {
		fakeGmailID = fmt.Sprintf("MANUAL-%d", time.Now().UnixNano())
	}

	// 5. Inisialisasi Model
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
		GmailID:     fakeGmailID,
	}

	// 6. Simpan ke Database
	if err := u.repo.Create(tx); err != nil {
		return nil, nil, apperror.Internal("failed to save manual transaction")
	}

	// 7. Update Target Progress jika Income
	if tx.Type == "income" {
		_ = u.targetUsecase.AddIncomeToTarget(tx.WorkspaceID, tx.Amount)
	}

	// 8. Panggil The Guardian (Return DTO Data Murni)
	budgetData, err := u.targetUsecase.CheckWorkspaceTarget(req.WorkspaceID)
	if err != nil {
		log.Printf("Target Check Error: %v", err)
		return tx, nil, nil
	}

	return tx, budgetData, nil
}

func (u *transactionUsecase) ProcessScanHybrid2(ctx context.Context, userID uint, workspaceID uint, imgData []byte, mimeType string) (*dto.ProcessScanHybridResult, uint, error) {
	user, err := u.authRepo.FindByID(userID)
	if err != nil {
		return nil, 0, err
	}

	if err := u.checkAndResetQuota(user); err != nil {
		log.Printf("⚠️ Failed to reset quota: %v", err)
	}

	limit := u.getOCRLimit(user.AccountTier)
	if user.OCRUsageCount >= limit {
		nextReset := user.LastResetUsage.AddDate(0, 0, 7)
		daysLeft := int(time.Until(nextReset).Hours() / 24)

		if daysLeft == 0 {
			hoursLeft := int(time.Until(nextReset).Hours())
			return nil, 0, fmt.Errorf("Quota empty! Auto reset in %d hours", hoursLeft)
		}
		return nil, 0, fmt.Errorf("Weekly scan limit reached (%d/%d). Resets in %d days", user.OCRUsageCount, limit, daysLeft)
	}

	result, err := u.hybridScanner.ScanReceiptHybrid(ctx, imgData, mimeType)
	if err != nil {
		return nil, 0, apperror.Internal("Hybrid scanner failed to process image")
	}

	cleanMerchant := strings.Title(strings.ToLower(strings.TrimSpace(result.Merchant)))
	parsedDate := time.Now()
	if strings.TrimSpace(result.Date) != "" {
		layouts := []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"}
		for _, layout := range layouts {
			if t, parseErr := time.Parse(layout, result.Date); parseErr == nil {
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
		Source:           "scan_hybrid",
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

	jsonData, _ := json.Marshal(tx)
	pending := &models.PendingTransaction{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Source:      "scan_hybrid",
		RawData:     string(jsonData),
		Status:      "pending",
	}

	if err := u.pendingRepo.Create(pending); err != nil {
		return nil, 0, apperror.Internal("Failed to save pending transaction")
	}

	_ = u.authRepo.IncrementOCRUsage(userID)

	return &dto.ProcessScanHybridResult{
		Transaction:  tx,
		Engine:       result.Engine,
		Confidence:   result.Confidence,
		FallbackUsed: result.FallbackUsed,
	}, pending.ID, nil
}

func (u *transactionUsecase) getOCRLimit(tier string) int {
	switch strings.ToLower(tier) {
	case "ultimate":
		return 50
	case "pro":
		return 10
	default:
		return 2
	}
}

func (u *transactionUsecase) checkAndResetQuota(user *models.User) error {
	now := time.Now()
	if user.LastResetUsage.IsZero() {
		user.LastResetUsage = now
		return u.authRepo.ResetOCRUsage(user.ID, now)
	}

	if now.Sub(user.LastResetUsage).Hours() >= 168 {
		user.OCRUsageCount = 0
		user.LastResetUsage = now
		return u.authRepo.ResetOCRUsage(user.ID, now)
	}
	return nil
}

func (u *transactionUsecase) ConfirmTransaction(ctx context.Context, pendingID uint) (*models.Transaction, *dto.BudgetStatusResponse, error) {
	// FIX: Pake method FindByID dari repository lu, balikan aslinya (*models.PendingTransaction, error)
	pending, err := u.pendingRepo.FindByID(pendingID)
	if err != nil || pending == nil {
		return nil, nil, apperror.NotFound("Pending transaction not found")
	}

	if pending.Status == "confirmed" {
		return nil, nil, apperror.BadRequest("Transaction already confirmed")
	}

	var tx models.Transaction
	if err := json.Unmarshal([]byte(pending.RawData), &tx); err != nil {
		return nil, nil, apperror.Internal("Failed to parse pending data")
	}

	tx.Status = "approved"

	if len(tx.TransactionItems) > 0 {
		if err := u.repo.CreateWithItems(&tx); err != nil {
			return nil, nil, apperror.Internal("Failed to save transaction with items")
		}
	} else {
		if err := u.repo.Create(&tx); err != nil {
			return nil, nil, apperror.Internal("Failed to save transaction")
		}
	}

	_ = u.pendingRepo.UpdateStatus(pendingID, "confirmed")

	if tx.Type == "income" {
		_ = u.targetUsecase.AddIncomeToTarget(tx.WorkspaceID, tx.Amount)
	}

	budgetData, _ := u.targetUsecase.CheckWorkspaceTarget(tx.WorkspaceID)

	return &tx, budgetData, nil
}

func (u *transactionUsecase) GetHistory(workspaceID uint) ([]models.Transaction, error) {
	return u.repo.GetByWorkspaceID(workspaceID)
}

func (u *transactionUsecase) DeleteTransaction(ctx context.Context, transactionID uint, userID uint) error {
	var tx models.Transaction
	if err := u.repo.FindByID(&tx, transactionID); err != nil {
		return apperror.NotFound("Transaction not found or already deleted")
	}

	// Cek apakah yang hapus itu owner transaksi atau owner workspace
	if tx.UserID != userID {
		return apperror.Forbidden("You are not authorized to delete this transaction")
	}

	return u.repo.Delete(transactionID)
}

func (u *transactionUsecase) HardDeleteTransaction(id uint) error {
	return u.repo.HardDelete(id)
}

func (u *transactionUsecase) GetPendingEmailLogs(userID uint) ([]models.EmailParsed, error) {
	return u.repo.GetPendingEmailLogs(userID)
}

func (u *transactionUsecase) ProcessScanAlternative(ctx context.Context, imagePath string, userID uint, workspaceID uint) (*dto.ProcessScanHybridResult, uint, error) {
	user, err := u.authRepo.FindByID(userID)
	if err != nil {
		return nil, 0, err
	}

	limit := u.getOCRLimit(user.AccountTier)
	if user.OCRUsageCount >= limit {
		return nil, 0, apperror.Forbidden("Weekly scan limit reached for your tier")
	}

	rawText, err := u.ocrSpaceClient.ExtractRawText(imagePath)
	if err != nil {
		return nil, 0, err
	}

	merchant, amount, transactionDate, items := utils.ManualParserV3(rawText)

	transaction := &models.Transaction{
		UserID:           userID,
		WorkspaceID:      workspaceID,
		Merchant:         merchant,
		Amount:           amount,
		Date:             transactionDate,
		Type:             "expense",
		Source:           "ocr_space_pure",
		Status:           "pending",
		Note:             "Auto-parsed: Please review amount and date",
		GmailID:          fmt.Sprintf("SCAN-ALT-%d-%d", userID, time.Now().UnixNano()),
		TransactionItems: items,
	}

	jsonData, _ := json.Marshal(transaction)
	pending := &models.PendingTransaction{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Source:      "telegram_alt",
		RawData:     string(jsonData),
		Status:      "pending",
	}

	if err = u.pendingRepo.Create(pending); err != nil {
		return nil, 0, err
	}

	_ = u.authRepo.IncrementOCRUsage(userID)

	return &dto.ProcessScanHybridResult{
		Transaction: transaction,
		Engine:      "OCR_SPACE_PURE",
	}, pending.ID, nil
}

func (u *transactionUsecase) ConfirmScanTransaction(ctx context.Context, txData *models.Transaction, items []models.TransactionItem) (*dto.BudgetStatusResponse, error) {
	// Pake method bawaan repo lu biar gak undefined
	txData.TransactionItems = items
	if err := u.repo.CreateWithItems(txData); err != nil {
		return nil, apperror.Internal("failed to confirm scanned transaction")
	}

	if txData.Type == "income" {
		_ = u.targetUsecase.AddIncomeToTarget(txData.WorkspaceID, txData.Amount)
	}

	budgetData, err := u.targetUsecase.CheckWorkspaceTarget(txData.WorkspaceID)
	if err != nil {
		log.Printf("Target Check Error: %v", err)
		return nil, nil
	}

	return budgetData, nil
}

func (u *transactionUsecase) ProcessTelegramInput(ctx context.Context, msg string) (string, bool, float64) {
	isIncome := strings.Contains(msg, "+")
	isExpense := strings.Contains(msg, "-")

	amount := u.extractAmount(msg)

	if isIncome {
		return "income", true, amount
	}
	if isExpense {
		return "expense", true, amount
	}

	return "", false, amount
}

func (u *transactionUsecase) extractAmount(msg string) float64 {
	re := regexp.MustCompile(`(\d+)`)
	match := re.FindString(msg)

	if match != "" {
		amount, _ := strconv.ParseFloat(match, 64)
		return amount
	}
	return 0
}

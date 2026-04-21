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
}

type transactionUsecase struct {
	repo          repository.TransactionRepository
	authRepo      repository.AuthRepository
	googleService service.GoogleAuthService
	geminiClient  *gemini.GeminiClient
	hybridScanner *ocr.HybridScanner
}

func NewTransactionUsecase(repo repository.TransactionRepository, authRepo repository.AuthRepository, googleService service.GoogleAuthService, gemini *gemini.GeminiClient, hybridScanner *ocr.HybridScanner) TransactionUsecase {
	return &transactionUsecase{
		repo:          repo,
		authRepo:      authRepo,
		googleService: googleService,
		geminiClient:  gemini,
		hybridScanner: hybridScanner,
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
		CategoryID:  &req.CategoryID,
		Amount:      req.Amount,
		Type:        req.Type,
		Date:        req.Date,
		Note:        req.Note,
		Merchant:    cleanMerchant,
		Source:      req.Source,
		Status:      "approved",
	}

	if err := u.repo.Create(tx); err != nil {
		return apperror.Internal("Failed to save manual transaction!")
	}

	return nil
}

func (u *transactionUsecase) ProcessScan(ctx context.Context, userID uint, workspaceID uint, imgData []byte, mimeType string) (*models.Transaction, error) {
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

	return tx, nil
}

func (u *transactionUsecase) SyncGmailTransactions(ctx context.Context) error {
	users, err := u.authRepo.FindAllWithGmail()
	if err != nil {
		return apperror.Internal("Failed to retrieve users with Gmail integration!")
	}

	for _, user := range users {
		srv, err := u.googleService.GetGmailService(user.GoogleRefreshToken)
		if err != nil {
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

			var subject string
			for _, h := range fullMsg.Payload.Headers {
				if h.Name == "Subject" {
					subject = h.Value
				}
			}

			bodyStr := u.getBody(fullMsg.Payload)
			parsed := utils.ParseMandiriEmail(subject, bodyStr)
			if parsed != nil {
				tx := &models.Transaction{
					UserID:      user.ID,
					WorkspaceID: 1,
					Amount:      parsed.Amount,
					Merchant:    parsed.Merchant,
					Method:      parsed.Method,
					Note:        parsed.Note,
					Date:        parsed.Date,
					Type:        "expense",
					Source:      "email_auto",
					Status:      "pending",
					GmailID:     m.Id,
				}

				_ = u.repo.Create(tx)
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

	return &dto.ProcessScanHybridResult{
		Transaction:  tx,
		Engine:       result.Engine,
		Confidence:   result.Confidence,
		FallbackUsed: result.FallbackUsed,
	}, nil
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

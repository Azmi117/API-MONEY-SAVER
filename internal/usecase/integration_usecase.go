package usecase

import (
	"context"
	"encoding/base64"
	"log"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/service"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/utils"
	"google.golang.org/api/gmail/v1"
)

type IntegrationUsecase interface {
	SyncGmailTransactions(ctx context.Context) error
	ProcessEmailMandiri(ctx context.Context, userID uint, workspaceID uint, subject string, body string) (*models.Transaction, error)
}

type integrationUsecase struct {
	txRepo        repository.TransactionRepository
	authRepo      repository.AuthRepository
	googleService service.GoogleAuthService
}

func NewIntegrationUsecase(
	txRepo repository.TransactionRepository,
	authRepo repository.AuthRepository,
	googleService service.GoogleAuthService,
) IntegrationUsecase {
	return &integrationUsecase{
		txRepo:        txRepo,
		authRepo:      authRepo,
		googleService: googleService,
	}
}

func (u *integrationUsecase) SyncGmailTransactions(ctx context.Context) error {
	users, err := u.authRepo.FindAllWithGmail()
	if err != nil {
		log.Printf("❌ [Robot Sync] Failed to retrieve users: %v", err)
		return apperror.Internal("Failed to retrieve users with Gmail integration")
	}

	for _, user := range users {
		srv, err := u.googleService.GetGmailService(user.GoogleRefreshToken)
		if err != nil {
			continue
		}

		query := "(from:no-reply@bankmandiri.co.id OR from:noreply.livin@bankmandiri.co.id)"
		res, err := srv.Users.Messages.List("me").Q(query).Do()
		if err != nil {
			continue
		}

		for _, m := range res.Messages {
			existingLog, _ := u.txRepo.GetEmailLogByGmailID(m.Id)
			if existingLog != nil {
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

			emailLog := &models.EmailParsed{
				UserID:     user.ID,
				RawEmail:   "HTML Content Truncated",
				BankSource: "Mandiri",
				Status:     "Pending",
				GmailID:    m.Id,
			}

			if parsed != nil {
				emailLog.Amount = parsed.Amount
				emailLog.Merchant = parsed.Merchant
				emailLog.ParsedDate = parsed.Date
				emailLog.Method = parsed.Method
				emailLog.Note = parsed.Note
				emailLog.Type = "expense"
			}

			_ = u.txRepo.CreateEmailLog(emailLog)
		}
	}
	return nil
}

func (u *integrationUsecase) getBody(payload *gmail.MessagePart) string {
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

func (u *integrationUsecase) ProcessEmailMandiri(ctx context.Context, userID uint, workspaceID uint, subject string, body string) (*models.Transaction, error) {
	parsed := utils.ParseMandiriEmail(subject, body)
	if parsed == nil {
		return nil, apperror.UnprocessableEntity("Email format is not a supported Mandiri transaction")
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

	if err := u.txRepo.Create(tx); err != nil {
		return nil, apperror.Internal("Failed to save email-processed transaction")
	}
	return tx, nil
}

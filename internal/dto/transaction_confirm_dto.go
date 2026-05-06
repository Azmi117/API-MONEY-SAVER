package dto

import "github.com/Azmi117/API-MONEY-SAVER.git/internal/models"

type ConfirmTransactionRequest struct {
	WorkspaceID uint                     `json:"workspace_id" binding:"required"`
	Merchant    string                   `json:"merchant" binding:"required"`
	Amount      float64                  `json:"amount" binding:"required"`
	Date        string                   `json:"date" binding:"required"` // Format: 2006-01-02
	Type        string                   `json:"type" binding:"required"`
	CategoryID  *uint                    `json:"category_id"`
	Note        string                   `json:"note"`
	Items       []models.TransactionItem `json:"items"`
}

type ConfirmEmailRequest struct {
	EmailParsedID uint `json:"email_parsed_id" binding:"required"`
	WorkspaceID   uint `json:"workspace_id" binding:"required"`
}

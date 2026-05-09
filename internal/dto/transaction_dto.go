package dto

import "time"

type CreateTransactionRequest struct {
	WorkspaceID uint      `json:"workspace_id" validate:"required"`
	CategoryID  *uint     `json:"category_id" validate:"required"`
	Amount      float64   `json:"amount" validate:"required,gt=0"`
	Type        string    `json:"type" validate:"required,oneof=income expense"`
	Date        time.Time `json:"date" validate:"required"`
	Note        string    `json:"note"`
	Merchant    string    `json:"merchant"` // Opsional kalau manual
	Source      string    `json:"source" validate:"required,oneof=web telegram"`
	Method      string    `json:"method"`
	GmailID     string    `json:"gmail_id"`
}

type ScanReceiptRequest struct {
	WorkspaceID uint   `json:"workspace_id" validate:"required"`
	Source      string `json:"source" default:"web"`
	// Di level controller/handler nanti kita tangkap multipart.File-nya
}

// Tetap di file yang sama ya Mi
type TelegramTransactionRequest struct {
	RawMessage string  `json:"raw_message"`
	UserID     uint    `json:"user_id"`
	Amount     float64 `json:"amount"`
	Type       string  `json:"type" validate:"oneof=income expense"` // Untuk nampung hasil hybrid
	Source     string  `json:"source" default:"telegram"`
}

type UserTransactionSummary struct {
	UserID   uint    `json:"user_id"`
	UserName string  `json:"user_name"`
	Total    float64 `json:"total"`
}

type SplitItemRequest struct {
	ItemName string  `json:"item_name"`
	UserID   uint    `json:"user_id"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type SplitBillRequest struct {
	TransactionID uint               `json:"transaction_id"`
	Items         []SplitItemRequest `json:"items"`
}

package dto

import "time"

type CreateTransactionRequest struct {
	WorkspaceID uint      `json:"workspace_id" validate:"required"`
	CategoryID  uint      `json:"category_id" validate:"required"`
	Amount      float64   `json:"amount" validate:"required,gt=0"`
	Type        string    `json:"type" validate:"required,oneof=income expense"`
	Date        time.Time `json:"date" validate:"required"`
	Note        string    `json:"note"`
	Merchant    string    `json:"merchant"` // Opsional kalau manual
	Source      string    `json:"source" validate:"required,oneof=web telegram"`
}

type ScanReceiptRequest struct {
	WorkspaceID uint   `json:"workspace_id" validate:"required"`
	Source      string `json:"source" default:"web"`
	// Di level controller/handler nanti kita tangkap multipart.File-nya
}

package dto

type TransactionItemConfirm struct {
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
	UserID      uint    `json:"user_id"` // Ini kunci buat Split Bill
}

type ConfirmTransactionRequest struct {
	WorkspaceID uint    `json:"workspace_id" binding:"required"`
	Merchant    string  `json:"merchant" binding:"required"`
	Amount      float64 `json:"amount" binding:"required"`
	PayerID     uint    `json:"payer_id" binding:"required"` // Tambahkan ini buat tau siapa yang bayar struknya
	Date        string  `json:"date" binding:"required"`
	Type        string  `json:"type" binding:"required"`
	CategoryID  *uint   `json:"category_id"`
	Note        string  `json:"note"`

	// Ganti []models.TransactionItem jadi struct pembantu tadi
	Items []TransactionItemConfirm `json:"items"`
}

type ConfirmEmailRequest struct {
	EmailParsedID uint `json:"email_parsed_id" binding:"required"`
	WorkspaceID   uint `json:"workspace_id" binding:"required"`
}

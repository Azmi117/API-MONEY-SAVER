package models

import (
	"time"

	"gorm.io/gorm"
)

type EmailParsed struct {
	gorm.Model
	UserID        uint      `gorm:"not null" json:"user_id"`
	TransactionID uint      `json:"transaction_id"` // null if not approved yet
	Amount        float64   `gorm:"type:decimal(15,2);not null" json:"amount"`
	Merchant      string    `gorm:"type:varchar(100)" json:"merchant"`
	ParsedDate    time.Time `gorm:"type:date" json:"parsed_date"`
	Status        string    `gorm:"type:varchar(20);default:'Pending'" json:"status"` // Approve, Pending
	BankSource    string    `gorm:"type:varchar(50)" json:"bank_source"`              // Mandiri, CIMB
	RawEmail      string    `gorm:"type:text" json:"raw_email"`
}

package models

import (
	"time"

	"gorm.io/gorm"
)

type EmailParsed struct {
	gorm.Model
	UserID        uint      `gorm:"not null" json:"user_id"`
	TransactionID *uint     `json:"transaction_id"`
	Amount        float64   `gorm:"type:decimal(15,2);not null" json:"amount"`
	Merchant      string    `gorm:"type:varchar(100)" json:"merchant"`
	ParsedDate    time.Time `gorm:"type:date" json:"parsed_date"`
	Status        string    `gorm:"type:varchar(20);default:'Pending'" json:"status"`
	BankSource    string    `gorm:"type:varchar(50)" json:"bank_source"`
	RawEmail      string    `gorm:"type:text" json:"raw_email"`
	GmailID       string    `gorm:"type:varchar(100);uniqueIndex" json:"gmail_id"`
	Method        string    `gorm:"type:varchar(50)" json:"method"`
	Note          string    `gorm:"type:text" json:"note"`
	Type          string    `gorm:"type:varchar(20)" json:"type"`

	// --- RELATIONSHIPS ---
	User        User        `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Transaction Transaction `gorm:"foreignKey:TransactionID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`
}

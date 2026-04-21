package models

import (
	"gorm.io/gorm"
)

type TransactionItem struct {
	gorm.Model
	TransactionID uint    `gorm:"not null" json:"transaction_id"`
	Description   string  `gorm:"type:varchar(255);not null" json:"description"`
	Quantity      int     `gorm:"default:1" json:"quantity"`
	Price         float64 `gorm:"type:decimal(15,2);not null" json:"price"`
	Total         float64 `gorm:"type:decimal(15,2);not null" json:"total"`

	// --- RELATIONSHIPS ---
	// TransactionItem BELONGS TO Transaction
	Transaction Transaction `gorm:"foreignKey:TransactionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

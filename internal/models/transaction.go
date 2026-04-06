package models

import (
	"time"

	"gorm.io/gorm"
)

type Transaction struct {
	gorm.Model
	UserID      uint      `gorm:"not null" json:"user_id"`
	WorkspaceID uint      `gorm:"not null" json:"workspace_id"`
	CategoryID  uint      `gorm:"not null" json:"category_id"`
	Amount      float64   `gorm:"type:decimal(15,2);not null" json:"amount"`
	Type        string    `gorm:"type:varchar(10);not null" json:"type"` // income/expense
	Date        time.Time `gorm:"type:date;not null" json:"date"`
	Note        string    `gorm:"type:varchar(255)" json:"note"`
	Source      string    `gorm:"type:varchar(20);default:'web'" json:"source"` // web, tele, email
}

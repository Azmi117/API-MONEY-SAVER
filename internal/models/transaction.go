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
	Source      string    `gorm:"type:varchar(20);default:'web'" json:"source"`

	// --- RELATIONSHIPS ---
	User         User          `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Workspace    Workspace     `gorm:"foreignKey:WorkspaceID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Category     Category      `gorm:"foreignKey:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`
	EmailsParsed []EmailParsed `gorm:"foreignKey:TransactionID"`
}

package models

import (
	"gorm.io/gorm"
)

type Debt struct {
	gorm.Model
	WorkspaceID uint    `gorm:"not null" json:"workspace_id"`
	FromUserID  uint    `gorm:"not null" json:"from_user_id"` // Siapa yang berutang
	ToUserID    uint    `gorm:"not null" json:"to_user_id"`   // Berutang ke siapa (Payer)
	Amount      float64 `gorm:"type:decimal(15,2);not null" json:"amount"`
	Note        string  `gorm:"type:varchar(255)" json:"note"`
	IsPaid      bool    `gorm:"default:false" json:"is_paid"`

	// Relationships
	Workspace Workspace `gorm:"foreignKey:WorkspaceID" json:"-"`
	FromUser  User      `gorm:"foreignKey:FromUserID" json:"-"`
	ToUser    User      `gorm:"foreignKey:ToUserID" json:"-"`
}

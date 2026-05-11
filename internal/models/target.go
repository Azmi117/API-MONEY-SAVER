package models

import "gorm.io/gorm"

type Target struct {
	gorm.Model
	WorkspaceID   uint    `gorm:"not null" json:"workspace_id"`
	Period        string  `gorm:"type:varchar(20);not null" json:"period"` // e.g., "2024-04"
	AmountLimit   float64 `gorm:"type:decimal(15,2)" json:"amount_limit"`
	CurrentAmount float64 `json:"current_amount" gorm:"default:0"`
	SavingsTarget float64 `gorm:"type:decimal(15,2)" json:"savings_target"`
	IsActive      bool    `gorm:"default:true" json:"is_active"`

	// --- RELATIONSHIPS ---
	Workspace Workspace `gorm:"foreignKey:WorkspaceID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

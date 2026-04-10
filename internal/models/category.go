package models

import "gorm.io/gorm"

type Category struct {
	gorm.Model
	Name        string `gorm:"type:varchar(50);not null" json:"name"`
	WorkspaceID uint   `gorm:"not null" json:"workspace_id"`
	Type        string `gorm:"type:varchar(10);not null" json:"type"` // income/expense

	// --- RELATIONSHIPS ---
	Workspace    Workspace     `gorm:"foreignKey:WorkspaceID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Transactions []Transaction `gorm:"foreignKey:CategoryID"`
}

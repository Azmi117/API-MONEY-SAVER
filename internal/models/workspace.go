package models

import "gorm.io/gorm"

type Workspace struct {
	gorm.Model
	Name    string `gorm:"type:varchar(100);not null" json:"name"`
	OwnerID uint   `gorm:"not null" json:"owner_id"`

	// --- RELATIONSHIPS ---
	Owner        User                  `gorm:"foreignKey:OwnerID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Members      []WorkspaceMember     `gorm:"foreignKey:WorkspaceID"`
	Categories   []Category            `gorm:"foreignKey:WorkspaceID"`
	Transactions []Transaction         `gorm:"foreignKey:WorkspaceID"`
	Targets      []Target              `gorm:"foreignKey:WorkspaceID"`
	Invitations  []WorkspaceInvitation `gorm:"foreignKey:WorkspaceID"`
}

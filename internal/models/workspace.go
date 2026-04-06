package models

import "gorm.io/gorm"

type Workspace struct {
	gorm.Model
	Name    string `gorm:"type:varchar(100);not null" json:"name"`
	OwnerID uint   `gorm:"not null" json:"owner_id"`

	// Relationships
	Members []WorkspaceMember `gorm:"foreignKey:WorkspaceID" json:"members,omitempty"`
}

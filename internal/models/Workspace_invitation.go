package models

import "gorm.io/gorm"

type WorkspaceInvitation struct {
	gorm.Model
	WorkspaceID uint   `gorm:"not null" json:"workspace_id"`
	InviterID   uint   `gorm:"not null" json:"inviter_id"`
	InvitedID   uint   `gorm:"not null" json:"invited_id"`
	Status      string `gorm:"type:varchar(20);default:'pending'" json:"status"`

	// Belongs To (Child ke Parent)
	Workspace Workspace `gorm:"foreignKey:WorkspaceID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Inviter   User      `gorm:"foreignKey:InviterID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Invited   User      `gorm:"foreignKey:InvitedID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

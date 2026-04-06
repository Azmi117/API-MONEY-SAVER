package models

type WorkspaceMember struct {
	ID          uint `gorm:"primaryKey" json:"id"`
	UserID      uint `gorm:"not null" json:"user_id"`
	WorkspaceID uint `gorm:"not null" json:"workspace_id"`
}

package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Name               string `gorm:"type:varchar(100);not null" json:"name"`
	Email              string `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	PasswordHash       string `gorm:"type:varchar(255);not null" json:"-"`
	Avatar             string `gorm:"type:varchar(255);default:'default-avatar.png'" json:"avatar"`
	TelegramID         int    `gorm:"uniqueIndex" json:"telegram_id"`
	EmailParsingEnable bool   `gorm:"default:false" json:"email_parsing_enable"`
	AccountTier        string `gorm:"type:varchar(20);default:'free'" json:"account_tier"`

	// --- RELATIONSHIPS (Dua Arah) ---
	OwnedWorkspaces     []Workspace           `gorm:"foreignKey:OwnerID"`
	WorkspaceMembers    []WorkspaceMember     `gorm:"foreignKey:UserID"`
	Transactions        []Transaction         `gorm:"foreignKey:UserID"`
	EmailsParsed        []EmailParsed         `gorm:"foreignKey:UserID"`
	RefreshTokens       []RefreshToken        `gorm:"foreignKey:UserID"`
	RevokeTokens        []RevokeToken         `gorm:"foreignKey:UserID"`
	SentInvitations     []WorkspaceInvitation `gorm:"foreignKey:InviterID"`
	ReceivedInvitations []WorkspaceInvitation `gorm:"foreignKey:InvitedID"`
}

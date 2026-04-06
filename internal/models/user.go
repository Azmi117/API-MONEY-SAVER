package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Name               string `gorm:"type:varchar(100);not null" json:"name"`
	Email              string `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	PasswordHash       string `gorm:"type:varchar(255);not null" json:"-"` // Hidden dari JSON
	Avatar             string `gorm:"type:varchar(255);default:'default-avatar.png'" json:"avatar"`
	TelegramID         int    `gorm:"uniqueIndex" json:"telegram_id"`
	EmailParsingEnable bool   `gorm:"default:false" json:"email_parsing_enable"`

	// Relationships
	RefreshTokens []RefreshToken `gorm:"foreignKey:UserID" json:"-"`
	RevokeTokens  []RevokeToken  `gorm:"foreignKey:UserID" json:"-"`
}

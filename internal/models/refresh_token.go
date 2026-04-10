package models

import (
	"time"

	"gorm.io/gorm"
)

type RefreshToken struct {
	gorm.Model
	UserID       uint      `gorm:"not null" json:"user_id"`
	RefreshToken string    `gorm:"type:text;not null" json:"refresh_token"`
	ExpiresAt    time.Time `gorm:"not null" json:"expires_at"`

	// --- RELATIONSHIPS ---
	User User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

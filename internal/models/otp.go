package models

import "time"

type OTP struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"not null"`
	OTPCode   string    `gorm:"not null"`
	Type      string    `gorm:"not null"` // isi: "register", "login", atau "forgot_password"
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
}

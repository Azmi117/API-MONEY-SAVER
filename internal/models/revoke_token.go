package models

import "gorm.io/gorm"

type RevokeToken struct {
	gorm.Model
	UserID uint   `gorm:"not null" json:"user_id"`
	Token  string `gorm:"type:text;not null;index" json:"token"`
}

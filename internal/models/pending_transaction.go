package models

import (
	"time"
)

type PendingTransaction struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null" json:"user_id"`
	WorkspaceID uint      `gorm:"not null" json:"workspace_id"`
	Source      string    `gorm:"type:varchar(50)" json:"source"`                   // e.g., "telegram_alt"
	RawData     string    `gorm:"type:jsonb" json:"raw_data"`                       // Simpan JSON string hasil scan
	Status      string    `gorm:"type:varchar(20);default:'pending'" json:"status"` // pending, approved, rejected
	ImagePath   string    `json:"image_path"`                                       // Buat nyimpen path uploads/pending/xxx.jpg
	RawOCRData  string    `gorm:"type:text" json:"raw_ocr_data"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

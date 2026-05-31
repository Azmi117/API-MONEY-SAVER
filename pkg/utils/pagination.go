package utils

import "gorm.io/gorm"

// Paginate bikin scope GORM buat handle limit & offset otomatis
func Paginate(page, limit int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if page <= 0 {
			page = 1
		}

		// Hard limit biar FE gak ngerequest sejuta data trus bikin server nyedot memori
		switch {
		case limit > 100:
			limit = 100
		case limit <= 0:
			limit = 10 // Default limit
		}

		offset := (page - 1) * limit
		return db.Offset(offset).Limit(limit)
	}
}

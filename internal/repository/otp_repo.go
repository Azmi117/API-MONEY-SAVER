package repository

import (
	"errors"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"gorm.io/gorm"
)

type OTPRepository interface {
	Create(otp *models.OTP) error
	FindLatestByUserID(userID uint, otpType string) (*models.OTP, error)
	DeleteByUserIDAndType(userID uint, otpType string) error
}

type otpRepository struct {
	db *gorm.DB
}

func NewOTPRepository(db *gorm.DB) OTPRepository {
	return &otpRepository{db}
}

func (r *otpRepository) Create(otp *models.OTP) error {
	return r.db.Create(otp).Error
}

func (r *otpRepository) FindLatestByUserID(userID uint, otpType string) (*models.OTP, error) {
	var otp models.OTP
	// Ambil OTP paling baru berdasarkan tipe
	err := r.db.Where("user_id = ? AND type = ?", userID, otpType).Order("created_at DESC").First(&otp).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &otp, nil
}

func (r *otpRepository) DeleteByUserIDAndType(userID uint, otpType string) error {
	return r.db.Where("user_id = ? AND type = ?", userID, otpType).Delete(&models.OTP{}).Error
}

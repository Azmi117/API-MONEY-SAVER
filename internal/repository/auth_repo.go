package repository

import (
	"errors"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"gorm.io/gorm"
)

type AuthRepository interface {
	Create(user *models.User) error
	Update(user *models.User) error
	FindByEmail(email string) (*models.User, error)
	FindByID(id uint) (*models.User, error)
	UpdateTier(id uint, tier string) error
	CreateRefreshToken(token *models.RefreshToken) error
	GetRefreshToken(token string) (*models.RefreshToken, error)
	DeleteRefreshToken(token string) error
	CreateRevokeToken(token *models.RevokeToken) error
	IsTokenRevoked(token string) bool
	FindAllWithGmail() ([]models.User, error)
	GetByTelegramID(tgID int64) (*models.User, error)
	IncrementOCRUsage(userID uint) error
	ResetOCRUsage(userID uint, now time.Time) error
	SetBindingCode(userID uint, code string, expiry time.Time) error
	FindByBindingCode(code string) (*models.User, error)
	FinalizeBinding(userID uint, telegramID int64) error
}

type authRepository struct {
	db *gorm.DB
}

func NewAuthRepository(params *gorm.DB) AuthRepository {
	return &authRepository{params}
}

func (r *authRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *authRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

func (r *authRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *authRepository) FindByID(id uint) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *authRepository) UpdateTier(id uint, tier string) error {
	return r.db.Model(&models.User{}).Where("id = ?", id).Update("account_tier", tier).Error
}

func (r *authRepository) CreateRefreshToken(token *models.RefreshToken) error {
	return r.db.Create(token).Error
}

func (r *authRepository) GetRefreshToken(token string) (*models.RefreshToken, error) {
	var rt models.RefreshToken
	err := r.db.Where("refresh_token = ?", token).First(&rt).Error
	if err != nil {
		return nil, err
	}
	return &rt, nil
}

func (r *authRepository) DeleteRefreshToken(token string) error {
	return r.db.Where("refresh_token = ?", token).Delete(&models.RefreshToken{}).Error
}

func (r *authRepository) CreateRevokeToken(token *models.RevokeToken) error {
	return r.db.Create(token).Error
}

func (r *authRepository) IsTokenRevoked(token string) bool {
	var rt models.RevokeToken
	err := r.db.Where("token = ?", token).First(&rt).Error
	return err == nil
}

func (r *authRepository) FindAllWithGmail() ([]models.User, error) {
	var users []models.User
	err := r.db.Where("gmail_enabled = ?", true).Find(&users).Error
	return users, err
}

func (r *authRepository) GetByTelegramID(tgID int64) (*models.User, error) {
	var user models.User
	err := r.db.Preload("OwnedWorkspaces").Where("telegram_id = ?", tgID).First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

func (r *authRepository) IncrementOCRUsage(userID uint) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).
		UpdateColumn("ocr_usage_count", gorm.Expr("ocr_usage_count + ?", 1)).Error
}

func (r *authRepository) ResetOCRUsage(userID uint, now time.Time) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{
			"ocr_usage_count":  0,
			"last_reset_usage": now,
		}).Error
}

func (r *authRepository) SetBindingCode(userID uint, code string, expiry time.Time) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"binding_code":       code,
		"binding_expires_at": expiry,
	}).Error
}

func (r *authRepository) FindByBindingCode(code string) (*models.User, error) {
	var user models.User
	err := r.db.Where("binding_code = ? AND binding_expires_at > ?", code, time.Now()).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *authRepository) FinalizeBinding(userID uint, telegramID int64) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"telegram_id":        telegramID,
		"binding_code":       nil,
		"binding_expires_at": nil,
	}).Error
}

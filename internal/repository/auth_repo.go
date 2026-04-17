package repository

import (
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
}

type authRepository struct {
	db *gorm.DB
}

func NewAuthRepository(params *gorm.DB) AuthRepository {
	return &authRepository{params}
}

func (r *authRepository) Create(user *models.User) error {
	return r.db.Create(&user).Error
}

func (r *authRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

func (r *authRepository) FindByEmail(email string) (*models.User, error) {
	var input models.User
	err := r.db.Where("email = ?", email).First(&input).Error
	if err != nil {
		return nil, err
	}
	return &input, nil
}

func (r *authRepository) FindByID(id uint) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// 2. Fungsi Tambahan: Update Tier (Free ke Pro/Platinum)
func (r *authRepository) UpdateTier(id uint, tier string) error {
	// Pake .Model().Update() biar GORM tau table mana yang mau di-update field-nya aja
	return r.db.Model(&models.User{}).Where("id = ?", id).Update("account_tier", tier).Error
}

func (r *authRepository) CreateRefreshToken(token *models.RefreshToken) error {
	return r.db.Create(token).Error
}

func (r *authRepository) GetRefreshToken(token string) (*models.RefreshToken, error) {
	var rt models.RefreshToken
	err := r.db.Where("refresh_token = ?", token).First(&rt).Error
	return &rt, err
}

func (r *authRepository) DeleteRefreshToken(token string) error {
	return r.db.Where("refresh_token = ?", token).Delete(&models.RefreshToken{}).Error
}

func (r *authRepository) CreateRevokeToken(token *models.RevokeToken) error {
	return r.db.Create(token).Error
}

func (r *authRepository) IsTokenRevoked(token string) bool {
	var rt models.RevokeToken
	// Cek apakah token ada di tabel blacklist
	err := r.db.Where("token = ?", token).First(&rt).Error
	return err == nil // Kalau ketemu (nil), berarti di-revoke (true)
}

func (r *authRepository) FindAllWithGmail() ([]models.User, error) {
	var users []models.User
	// Cari user yang gmail_enabled nya true
	err := r.db.Where("gmail_enabled = ?", true).Find(&users).Error
	return users, err
}

package repository

import (
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"gorm.io/gorm"
)

type AuthRepository interface {
	Create(user *models.User) error
	FindByEmail(email string) (*models.User, error)
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

func (r *authRepository) FindByEmail(email string) (*models.User, error) {
	var input models.User
	err := r.db.Where("email = ?", email).First(&input).Error
	if err != nil {
		return nil, err
	}
	return &input, nil
}

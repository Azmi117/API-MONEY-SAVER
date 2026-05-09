package repository

import (
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"gorm.io/gorm"
)

type DebtRepository interface {
	CreateInBatch(debts []models.Debt) error
}

type debtRepository struct {
	db *gorm.DB
}

func NewDebtRepository(db *gorm.DB) DebtRepository {
	return &debtRepository{db: db}
}

func (r *debtRepository) CreateInBatch(debts []models.Debt) error {
	// GORM bakal otomatis handle slice struct buat bulk insert
	return r.db.Create(&debts).Error
}

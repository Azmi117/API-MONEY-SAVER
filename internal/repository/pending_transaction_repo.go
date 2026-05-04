package repository

import (
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"gorm.io/gorm"
)

type PendingTransactionRepository interface {
	Create(pending *models.PendingTransaction) error
	FindByID(id uint) (*models.PendingTransaction, error)
	UpdateStatus(id uint, status string) error
}

type pendingTransactionRepository struct {
	db *gorm.DB
}

func NewPendingTransactionRepository(db *gorm.DB) PendingTransactionRepository {
	return &pendingTransactionRepository{db: db}
}

func (r *pendingTransactionRepository) Create(p *models.PendingTransaction) error {
	return r.db.Create(p).Error
}

func (r *pendingTransactionRepository) FindByID(id uint) (*models.PendingTransaction, error) {
	var p models.PendingTransaction
	err := r.db.First(&p, id).Error
	return &p, err
}

func (r *pendingTransactionRepository) UpdateStatus(id uint, status string) error {
	return r.db.Model(&models.PendingTransaction{}).Where("id = ?", id).Update("status", status).Error
}

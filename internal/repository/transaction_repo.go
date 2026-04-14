package repository

import (
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"gorm.io/gorm"
)

type TransactionRepository interface {
	Create(transaction *models.Transaction) error
	// IsDuplicate bakal ngecek kombinasi Amount, Merchant, dan rentang waktu Date
	IsDuplicate(workspaceID uint, amount float64, merchant string, date time.Time) (bool, error)
	UpdateStatus(id uint, status string) error
	GetByWorkspaceID(workspaceID uint) ([]models.Transaction, error)
	Delete(id uint) error
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db}
}

func (r *transactionRepository) Create(transaction *models.Transaction) error {
	return r.db.Create(transaction).Error
}

func (r *transactionRepository) IsDuplicate(workspaceID uint, amount float64, merchant string, date time.Time) (bool, error) {
	var count int64

	// Tentukan rentang waktu toleransi (5 menit sebelum & sesudah)
	startTime := date.Add(-5 * time.Minute)
	endTime := date.Add(5 * time.Minute)

	err := r.db.Model(&models.Transaction{}).
		Where("workspace_id = ?", workspaceID).
		Where("amount = ?", amount).
		Where("merchant = ?", merchant).
		Where("date BETWEEN ? AND ?", startTime, endTime).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *transactionRepository) UpdateStatus(id uint, status string) error {
	return r.db.Model(&models.Transaction{}).Where("id = ?", id).Update("status", status).Error
}

func (r *transactionRepository) GetByWorkspaceID(workspaceID uint) ([]models.Transaction, error) {
	var transactions []models.Transaction
	// Kita urutin dari yang paling baru (Descending)
	err := r.db.Where("workspace_id = ?", workspaceID).Order("date desc").Find(&transactions).Error
	return transactions, err
}

func (r *transactionRepository) Delete(id uint) error {
	// Kita pake Unscoped() kalau lu mau bener-bener hapus dari DB (Hard Delete)
	// Kalau di model Transaction lu pake gorm.Model, dia otomatis Soft Delete
	return r.db.Delete(&models.Transaction{}, id).Error
}

package repository

import (
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/utils"
	"gorm.io/gorm"
)

type PendingTransactionRepository interface {
	Create(pending *models.PendingTransaction) error
	FindByID(id uint) (*models.PendingTransaction, error)
	UpdateStatus(id uint, status string) error
	GetPendingList(workspaceID uint, page int, limit int) ([]models.PendingTransaction, int64, error)
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

// TAMBAHIN IMPLEMENTASI INI 👇
func (r *pendingTransactionRepository) GetPendingList(workspaceID uint, page int, limit int) ([]models.PendingTransaction, int64, error) {
	var pendings []models.PendingTransaction
	var totalItems int64

	// 1. Hitung total data yang statusnya masih "pending" buat info Load More di FE
	err := r.db.Model(&models.PendingTransaction{}).
		Where("workspace_id = ? AND status = ?", workspaceID, "pending").
		Count(&totalItems).Error
	if err != nil {
		return nil, 0, err
	}

	// 2. Tarik datanya pake utils.Paginate
	// Jangan lupa import "yourproject/pkg/utils" kalau belum ada di atas
	err = r.db.Scopes(utils.Paginate(page, limit)).
		Where("workspace_id = ? AND status = ?", workspaceID, "pending").
		Order("created_at DESC"). // Tampilin yang paling baru discan di atas
		Find(&pendings).Error

	return pendings, totalItems, err
}

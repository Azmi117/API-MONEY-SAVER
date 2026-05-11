package repository

import (
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"gorm.io/gorm"
)

type DebtRepository interface {
	CreateInBatch(debts []models.Debt) error
	GetDebtsByWorkspace(workspaceID uint) ([]models.Debt, error)
	IsShortCodeExists(workspaceID uint, code string) (bool, error)
	GetDebtByShortCode(workspaceID uint, code string) (*models.Debt, error)
	UpdateIsPaid(debtID uint, status bool) error
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

func (r *debtRepository) GetDebtsByWorkspace(workspaceID uint) ([]models.Debt, error) {
	var debts []models.Debt
	// Kita Preload FromUser dan ToUser supaya dapet nama orangnya di Web
	// Kita filter yang belum lunas (is_paid = false)
	err := r.db.Preload("FromUser").Preload("ToUser").
		Where("workspace_id = ? AND is_paid = ?", workspaceID, false).
		Order("created_at desc").
		Find(&debts).Error
	return debts, err
}

func (r *debtRepository) IsShortCodeExists(workspaceID uint, code string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Debt{}).Where("workspace_id = ? AND short_code = ? AND is_paid = false", workspaceID, code).Count(&count).Error
	return count > 0, err
}

func (r *debtRepository) GetDebtByShortCode(workspaceID uint, code string) (*models.Debt, error) {
	var debt models.Debt
	err := r.db.Preload("FromUser").Where("workspace_id = ? AND short_code = ? AND is_paid = false", workspaceID, code).First(&debt).Error
	return &debt, err
}

func (r *debtRepository) UpdateIsPaid(debtID uint, status bool) error {
	return r.db.Model(&models.Debt{}).Where("id = ?", debtID).Update("is_paid", status).Error
}

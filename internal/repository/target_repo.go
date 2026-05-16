package repository

import (
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"gorm.io/gorm"
)

type TargetRepository interface {
	GetActiveTarget(workspaceID uint, period string) (*models.Target, error)
	GetActiveTargets(workspaceID uint, period string) ([]models.Target, error)
	UpsertTarget(target *models.Target) error
	GetByWorkspaceAndPeriod(wsID uint, period string) (*models.Target, error)
	Update(target *models.Target) error
}

type targetRepository struct {
	db *gorm.DB
}

func NewTargetRepository(db *gorm.DB) TargetRepository {
	return &targetRepository{db: db}
}

func (r *targetRepository) GetByWorkspaceAndPeriod(wsID uint, period string) (*models.Target, error) {
	var target models.Target
	// Ambil target yang aktif untuk workspace dan bulan tersebut
	err := r.db.Where("workspace_id = ? AND period = ? AND is_active = ?", wsID, period, true).First(&target).Error
	if err != nil {
		return nil, err
	}
	return &target, nil
}

func (r *targetRepository) Update(target *models.Target) error {
	// Kita pake Save buat update semua field di model target
	return r.db.Save(target).Error
}

// 4. TARGETING SYSTEM
func (r *targetRepository) GetActiveTarget(workspaceID uint, period string) (*models.Target, error) {
	var target models.Target
	err := r.db.Where("workspace_id = ? AND period = ? AND is_active = ?", workspaceID, period, true).First(&target).Error
	if err != nil {
		return nil, err
	}
	return &target, nil
}

func (r *targetRepository) UpsertTarget(target *models.Target) error {
	return r.db.Where(models.Target{WorkspaceID: target.WorkspaceID, Period: target.Period}).
		Assign(models.Target{AmountLimit: target.AmountLimit, SavingsTarget: target.SavingsTarget, IsActive: true}).
		FirstOrCreate(target).Error
}

func (r *targetRepository) GetActiveTargets(workspaceID uint, period string) ([]models.Target, error) {
	var targets []models.Target
	err := r.db.Where("workspace_id = ? AND period = ?", workspaceID, period).Find(&targets).Error
	return targets, err
}

package repository

import (
	"errors"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"gorm.io/gorm"
)

type WorkspaceRepository interface {
	Create(workspace *models.Workspace) error
	FindByID(id uint) (*models.Workspace, error)
	FindByOwnerID(ownerID uint) ([]models.Workspace, error)
	FindAllByUserID(userID uint) ([]models.Workspace, error)
	Update(workspace *models.Workspace) error
	Delete(id uint) error
	CreateInvitation(invitation *models.WorkspaceInvitation) error
	FindInvitationByID(id uint) (*models.WorkspaceInvitation, error)
	FindPendingInvitationsByUserID(userID uint) ([]models.WorkspaceInvitation, error)
	UpdateInvitationStatus(invitation *models.WorkspaceInvitation) error
	AcceptInvitation(invitation *models.WorkspaceInvitation) error
	GetMembersCount(workspaceID uint) (int, error)
	ConnectToTelegramGroup(workspaceID uint, chatID int64) error
	GetByIDAndOwner(id uint, ownerID uint) (*models.Workspace, error)
	GetWorkspacesByOwner(ownerID uint) ([]models.Workspace, error)
	GetByTelegramChatID(chatID int64) (*models.Workspace, error)
	IsMember(workspaceID uint, userID uint) (bool, error)
	GetMembersByWorkspaceID(workspaceID uint) ([]models.WorkspaceMember, error)
	GetActiveTarget(workspaceID uint, period string) (*models.Target, error)
	GetActiveTargets(workspaceID uint, period string) ([]models.Target, error)
	UpsertTarget(target *models.Target) error
}

type workspaceRepository struct {
	db *gorm.DB
}

func NewWorkspaceRepository(db *gorm.DB) WorkspaceRepository {
	return &workspaceRepository{db}
}

// 1. CREATE WORKSPACE (Owner otomatis jadi Member pertama)
func (r *workspaceRepository) Create(workspace *models.Workspace) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(workspace).Error; err != nil {
			return err
		}
		member := models.WorkspaceMember{
			UserID:      workspace.OwnerID,
			WorkspaceID: workspace.ID,
		}
		return tx.Create(&member).Error
	})
}

// 2. FINDER & MANAGEMENT
func (r *workspaceRepository) FindAllByUserID(userID uint) ([]models.Workspace, error) {
	var workspaces []models.Workspace
	err := r.db.Joins("JOIN workspace_members on workspace_members.workspace_id = workspaces.id").
		Where("workspace_members.user_id = ?", userID).
		Find(&workspaces).Error
	return workspaces, err
}

func (r *workspaceRepository) FindByID(id uint) (*models.Workspace, error) {
	var workspace models.Workspace
	err := r.db.First(&workspace, id).Error
	return &workspace, err
}

func (r *workspaceRepository) FindByOwnerID(ownerID uint) ([]models.Workspace, error) {
	var workspaces []models.Workspace
	err := r.db.Where("owner_id = ?", ownerID).Find(&workspaces).Error
	return workspaces, err
}

func (r *workspaceRepository) Update(workspace *models.Workspace) error {
	return r.db.Save(workspace).Error
}

func (r *workspaceRepository) Delete(id uint) error {
	return r.db.Delete(&models.Workspace{}, id).Error
}

// --- 3. INVITATION SYSTEM LOGIC ---

func (r *workspaceRepository) CreateInvitation(invitation *models.WorkspaceInvitation) error {
	return r.db.Create(invitation).Error
}

func (r *workspaceRepository) FindInvitationByID(id uint) (*models.WorkspaceInvitation, error) {
	var inv models.WorkspaceInvitation
	err := r.db.First(&inv, id).Error
	return &inv, err
}

func (r *workspaceRepository) FindPendingInvitationsByUserID(userID uint) ([]models.WorkspaceInvitation, error) {
	var invs []models.WorkspaceInvitation
	err := r.db.Where("invited_id = ? AND status = ?", userID, "pending").
		Preload("Workspace"). // Biar user tau dia diundang ke workspace mana
		Preload("Inviter").   // Biar user tau siapa yang ngajak
		Find(&invs).Error
	return invs, err
}

func (r *workspaceRepository) UpdateInvitationStatus(inv *models.WorkspaceInvitation) error {
	return r.db.Save(inv).Error
}

// FUNGSI SAKTI: Terima undangan & masukin ke member dalam satu transaksi
func (r *workspaceRepository) AcceptInvitation(inv *models.WorkspaceInvitation) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// A. Update status jadi accepted
		inv.Status = "accepted"
		if err := tx.Save(inv).Error; err != nil {
			return err
		}

		// B. Insert ke tabel member beneran
		member := models.WorkspaceMember{
			UserID:      inv.InvitedID,
			WorkspaceID: inv.WorkspaceID,
		}
		return tx.Create(&member).Error
	})
}

func (r *workspaceRepository) GetMembersCount(workspaceID uint) (int, error) {
	var count int64
	err := r.db.Model(&models.WorkspaceMember{}).Where("workspace_id = ?", workspaceID).Count(&count).Error
	return int(count), err
}

func (r *workspaceRepository) ConnectToTelegramGroup(workspaceID uint, chatID int64) error {
	return r.db.Model(&models.Workspace{}).Where("id = ?", workspaceID).Update("telegram_chat_id", chatID).Error
}

func (r *workspaceRepository) GetByIDAndOwner(id uint, ownerID uint) (*models.Workspace, error) {
	var ws models.Workspace
	err := r.db.Where("id = ? AND owner_id = ?", id, ownerID).First(&ws).Error
	if err != nil {
		return nil, err
	}
	return &ws, nil
}

func (r *workspaceRepository) GetWorkspacesByOwner(ownerID uint) ([]models.Workspace, error) {
	var workspaces []models.Workspace
	// Tarik semua workspace yang owner_id-nya cocok
	err := r.db.Where("owner_id = ?", ownerID).Find(&workspaces).Error
	return workspaces, err
}

func (r *workspaceRepository) GetByTelegramChatID(chatID int64) (*models.Workspace, error) {
	var ws models.Workspace

	// Kita cari workspace yang punya telegram_chat_id cocok
	err := r.db.Where("telegram_chat_id = ?", chatID).First(&ws).Error

	if err != nil {
		// Jika grup ini belum pernah di-init (/init), balikin nil biar handler tau
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &ws, nil
}

func (r *workspaceRepository) IsMember(workspaceID uint, userID uint) (bool, error) {
	var count int64
	// Hapus pengecekan deleted_at karena kolomnya emang gak ada di tabel ini
	err := r.db.Table("workspace_members").
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		Count(&count).Error

	return count > 0, err
}

func (r *workspaceRepository) GetMembersByWorkspaceID(workspaceID uint) ([]models.WorkspaceMember, error) {
	var members []models.WorkspaceMember
	// Preload("User") ini kuncinya biar dapet data detail usernya
	err := r.db.Preload("User").Where("workspace_id = ?", workspaceID).Find(&members).Error
	return members, err
}

func (r *workspaceRepository) GetActiveTarget(workspaceID uint, period string) (*models.Target, error) {
	var target models.Target
	err := r.db.Where("workspace_id = ? AND period = ? AND is_active = ?", workspaceID, period, true).First(&target).Error
	if err != nil {
		return nil, err
	}
	return &target, nil
}

func (r *workspaceRepository) UpsertTarget(target *models.Target) error {
	return r.db.Where(models.Target{WorkspaceID: target.WorkspaceID, Period: target.Period}).
		Assign(models.Target{AmountLimit: target.AmountLimit, SavingsTarget: target.SavingsTarget, IsActive: true}).
		FirstOrCreate(target).Error
}

func (r *workspaceRepository) GetActiveTargets(workspaceID uint, period string) ([]models.Target, error) {
	var targets []models.Target // Gunakan Slice

	// Pake .Find() bukan .First() supaya dapet semua baris yang cocok
	err := r.db.Where("workspace_id = ? AND period = ?", workspaceID, period).Find(&targets).Error

	return targets, err
}

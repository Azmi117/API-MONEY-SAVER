package repository

import (
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"gorm.io/gorm"
)

type WorkspaceRepository interface {
	// --- Workspace Core ---
	Create(workspace *models.Workspace) error
	FindByID(id uint) (*models.Workspace, error)
	FindByOwnerID(ownerID uint) ([]models.Workspace, error)
	FindAllByUserID(userID uint) ([]models.Workspace, error)
	Update(workspace *models.Workspace) error
	Delete(id uint) error

	// --- Invitation Logic (PENGGANTI AddMember) ---
	CreateInvitation(invitation *models.WorkspaceInvitation) error
	FindInvitationByID(id uint) (*models.WorkspaceInvitation, error)
	FindPendingInvitationsByUserID(userID uint) ([]models.WorkspaceInvitation, error)
	UpdateInvitationStatus(invitation *models.WorkspaceInvitation) error
	AcceptInvitation(invitation *models.WorkspaceInvitation) error // TRANSACTION: Invite + Member

	// --- Stats ---
	GetMembersCount(workspaceID uint) (int, error)
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

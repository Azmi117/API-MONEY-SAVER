package usecase

import (
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
)

type WorkspaceUsecase interface {
	CreateWorkspace(name string, ownerID uint) (*models.Workspace, error)
	GetUserWorkspaces(userID uint) ([]models.Workspace, error)
	UpdateWorkspace(workspaceID uint, userID uint, newName string) error
	DeleteWorkspace(workspaceID uint, userID uint) error

	InviteMember(workspaceID uint, ownerID uint, invitedUserID uint) error
	GetPendingInvitations(userID uint) ([]models.WorkspaceInvitation, error)
	RespondToInvitation(invitationID uint, userID uint, accept bool) error

	UpgradeTier(userID uint, newTier string) error
}

type workspaceUsecase struct {
	workspaceRepo repository.WorkspaceRepository
	authRepo      repository.AuthRepository
}

func NewWorkspaceUsecase(wr repository.WorkspaceRepository, ar repository.AuthRepository) WorkspaceUsecase {
	return &workspaceUsecase{workspaceRepo: wr, authRepo: ar}
}

// 1. CREATE WORKSPACE
func (u *workspaceUsecase) CreateWorkspace(name string, ownerID uint) (*models.Workspace, error) {
	user, _ := u.authRepo.FindByID(ownerID)
	existingWorkspaces, _ := u.workspaceRepo.FindByOwnerID(ownerID)

	count := len(existingWorkspaces)
	if user.AccountTier == "free" && count >= 2 {
		return nil, apperror.UnprocessableEntity("Workspace limit reached: FREE tier is limited to 2 workspaces!")
	}
	if user.AccountTier == "pro" && count >= 10 {
		return nil, apperror.UnprocessableEntity("Workspace limit reached: PRO tier is limited to 10 workspaces!")
	}

	workspace := &models.Workspace{Name: name, OwnerID: ownerID}
	if err := u.workspaceRepo.Create(workspace); err != nil {
		return nil, err
	}
	return workspace, nil
}

// 2. MANAGEMENT FEATURES (Lengkap)
func (u *workspaceUsecase) GetUserWorkspaces(userID uint) ([]models.Workspace, error) {
	workspace, err := u.workspaceRepo.FindAllByUserID(userID)

	if err != nil {
		return nil, apperror.Internal("Failed to retrieve workspaces.")
	}

	return workspace, nil
}

func (u *workspaceUsecase) UpdateWorkspace(workspaceID uint, userID uint, newName string) error {
	ws, err := u.workspaceRepo.FindByID(workspaceID)
	if err != nil {
		return apperror.NotFound("Workspace not found!")
	}
	if ws.OwnerID != userID {
		return apperror.Forbidden("Access denied: You are not the owner of this workspace!")
	}
	ws.Name = newName
	return u.workspaceRepo.Update(ws)
}

func (u *workspaceUsecase) DeleteWorkspace(workspaceID uint, userID uint) error {
	ws, err := u.workspaceRepo.FindByID(workspaceID)
	if err != nil {
		return apperror.NotFound("Workspace not found!")
	}
	if ws.OwnerID != userID {
		return apperror.Forbidden("Access denied: You are not the owner of this workspace!")
	}
	return u.workspaceRepo.Delete(workspaceID)
}

// 3. INVITATION LOGIC (With Safety Checks)
func (u *workspaceUsecase) InviteMember(workspaceID uint, ownerID uint, invitedUserID uint) error {
	if ownerID == invitedUserID {
		return apperror.BadRequest("Invalid action: You cannot invite yourself!")
	}

	ws, _ := u.workspaceRepo.FindByID(workspaceID)
	if ws.OwnerID != ownerID {
		return apperror.Forbidden("Access denied: You are not the owner of this workspace!")
	}

	// Cek limitasi Tiering
	user, _ := u.authRepo.FindByID(ownerID)
	currentMembers, _ := u.workspaceRepo.GetMembersCount(workspaceID)
	if user.AccountTier == "free" && currentMembers >= 2 {
		return apperror.UnprocessableEntity("Member limit reached: FREE tier allows a maximum of 2 members!")
	}

	// VALIDASI TAMBAHAN: Cek apa dia udah jadi member atau belum
	workspaces, _ := u.workspaceRepo.FindAllByUserID(invitedUserID)
	for _, w := range workspaces {
		if w.ID == workspaceID {
			return apperror.UnprocessableEntity("This user is already a member of this workspace.")
		}
	}

	invitation := &models.WorkspaceInvitation{
		WorkspaceID: workspaceID,
		InviterID:   ownerID,
		InvitedID:   invitedUserID,
	}

	if err := u.workspaceRepo.CreateInvitation(invitation); err != nil {
		// 500: Error database/sistem
		return apperror.Internal("Failed to create workspace invitation.")
	}

	return nil
}

func (u *workspaceUsecase) GetPendingInvitations(userID uint) ([]models.WorkspaceInvitation, error) {
	return u.workspaceRepo.FindPendingInvitationsByUserID(userID)
}

func (u *workspaceUsecase) RespondToInvitation(invitationID uint, userID uint, accept bool) error {
	inv, err := u.workspaceRepo.FindInvitationByID(invitationID)
	if err != nil || inv.InvitedID != userID {
		return apperror.NotFound("Invitation not found!")
	}

	if inv.Status != "pending" {
		return apperror.UnprocessableEntity("This invitation has already been processed!")
	}

	if accept {
		return u.workspaceRepo.AcceptInvitation(inv)
	}

	inv.Status = "rejected"
	return u.workspaceRepo.UpdateInvitationStatus(inv)
}

// 4. UPGRADE SIMULATION
func (u *workspaceUsecase) UpgradeTier(userID uint, newTier string) error {
	// Karena lo butuh update field di User, pastikan AuthRepo punya fungsi UpdateTier
	return u.authRepo.UpdateTier(userID, newTier)
}

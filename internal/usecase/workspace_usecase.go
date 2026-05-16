package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
)

type WorkspaceUsecase interface {
	CreateWorkspace(name string, ownerID uint) (*models.Workspace, error)
	GetUserWorkspaces(userID uint) ([]models.Workspace, error)
	UpdateWorkspace(workspaceID uint, userID uint, newName string) error
	DeleteWorkspace(workspaceID uint, userID uint) error
	InviteMember(workspaceID uint, ownerID uint, email string) error
	GetPendingInvitations(userID uint) ([]models.WorkspaceInvitation, error)
	AcceptInvitation(invitationID uint, userID uint) error
	RejectInvitation(invitationID uint, userID uint) error
	UpgradeTier(userID uint, newTier string) error
	InitGroupConnection(telegramUserID int64, workspaceID uint, telegramChatID int64) error
	GetUserWorkspaceList(telegramUserID int64) (string, error)
	CreateFromTelegram(ctx context.Context, telegramID int64, chatTitle string, chatID int64, wsType string) (*models.Workspace, error)
	GetMembers(workspaceID uint) ([]models.WorkspaceMember, error)
}

type workspaceUsecase struct {
	workspaceRepo repository.WorkspaceRepository
	authRepo      repository.AuthRepository
	categoryRepo  repository.CategoryRepository
	targetRepo    repository.TargetRepository
}

func NewWorkspaceUsecase(wr repository.WorkspaceRepository, ar repository.AuthRepository, cr repository.CategoryRepository, tr repository.TargetRepository) WorkspaceUsecase {
	return &workspaceUsecase{
		workspaceRepo: wr,
		authRepo:      ar,
		categoryRepo:  cr,
		targetRepo:    tr,
	}
}

func (u *workspaceUsecase) checkWorkspaceLimit(userID uint) error {
	user, err := u.authRepo.FindByID(userID)
	if err != nil {
		return apperror.NotFound("User not found")
	}

	existingWorkspaces, _ := u.workspaceRepo.FindByOwnerID(userID)
	count := len(existingWorkspaces)

	if user.AccountTier == "free" && count >= 2 {
		return apperror.UnprocessableEntity("Workspace limit reached: FREE tier is limited to 2 workspaces")
	}
	if user.AccountTier == "pro" && count >= 10 {
		return apperror.UnprocessableEntity("Workspace limit reached: PRO tier is limited to 10 workspaces")
	}

	return nil
}

// 1. CREATE WORKSPACE
func (u *workspaceUsecase) CreateWorkspace(name string, ownerID uint) (*models.Workspace, error) {
	// FIX: Panggil helper method biar rapi
	if err := u.checkWorkspaceLimit(ownerID); err != nil {
		return nil, err
	}

	workspace := &models.Workspace{Name: name, OwnerID: ownerID}
	if err := u.workspaceRepo.Create(workspace); err != nil {
		return nil, apperror.Internal("Failed to create workspace")
	}

	u.seedDefaultCategories(workspace.ID)

	return workspace, nil
}

func (u *workspaceUsecase) seedDefaultCategories(wsID uint) {
	defaults := []models.Category{
		{Name: "Snacks", Type: "expense", Icon: "snack", WorkspaceID: wsID},
		{Name: "Meals", Type: "expense", Icon: "rice", WorkspaceID: wsID},
		{Name: "Salary", Type: "income", Icon: "money", WorkspaceID: wsID},
		{Name: "Project", Type: "income", Icon: "code", WorkspaceID: wsID},
		{Name: "Self Reward", Type: "expense", Icon: "star", WorkspaceID: wsID},
	}

	for _, cat := range defaults {
		_ = u.categoryRepo.Create(&cat)
	}
}

// 2. MANAGEMENT FEATURES
func (u *workspaceUsecase) GetUserWorkspaces(userID uint) ([]models.Workspace, error) {
	workspaces, err := u.workspaceRepo.FindAllByUserID(userID)
	if err != nil {
		return nil, apperror.Internal("Failed to retrieve workspaces")
	}
	return workspaces, nil
}

func (u *workspaceUsecase) UpdateWorkspace(workspaceID uint, userID uint, newName string) error {
	ws, err := u.workspaceRepo.FindByID(workspaceID)
	if err != nil {
		return apperror.NotFound("Workspace not found")
	}
	if ws.OwnerID != userID {
		return apperror.Forbidden("Access denied: You are not the owner of this workspace")
	}
	ws.Name = newName
	return u.workspaceRepo.Update(ws)
}

func (u *workspaceUsecase) DeleteWorkspace(workspaceID uint, userID uint) error {
	ws, err := u.workspaceRepo.FindByID(workspaceID)
	if err != nil {
		return apperror.NotFound("Workspace not found")
	}
	if ws.OwnerID != userID {
		return apperror.Forbidden("Access denied: You are not the owner of this workspace")
	}
	return u.workspaceRepo.Delete(workspaceID)
}

// 3. INVITATION LOGIC
func (u *workspaceUsecase) InviteMember(workspaceID uint, ownerID uint, email string) error {
	invitedUser, err := u.authRepo.FindByEmail(email)
	if err != nil {
		return apperror.NotFound("User with the specified email was not found")
	}

	if ownerID == invitedUser.ID {
		return apperror.BadRequest("Invalid action: You cannot invite yourself")
	}

	ws, _ := u.workspaceRepo.FindByID(workspaceID)
	if ws.OwnerID != ownerID {
		return apperror.Forbidden("Access denied: You are not the owner of this workspace")
	}

	user, _ := u.authRepo.FindByID(ownerID)
	currentMembers, _ := u.workspaceRepo.GetMembersCount(workspaceID)
	if user.AccountTier == "free" && currentMembers >= 2 {
		return apperror.UnprocessableEntity("Member limit reached: FREE tier allows a maximum of 2 members")
	}

	isAlreadyMember, _ := u.workspaceRepo.IsMember(workspaceID, invitedUser.ID)
	if isAlreadyMember {
		return apperror.Conflict("This user is already a member of this workspace")
	}

	invitation := &models.WorkspaceInvitation{
		WorkspaceID: workspaceID,
		InviterID:   ownerID,
		InvitedID:   invitedUser.ID,
		Status:      "pending",
	}

	if err := u.workspaceRepo.CreateInvitation(invitation); err != nil {
		return apperror.Internal("Failed to create workspace invitation")
	}

	return nil
}

func (u *workspaceUsecase) GetPendingInvitations(userID uint) ([]models.WorkspaceInvitation, error) {
	invitations, err := u.workspaceRepo.FindPendingInvitationsByUserID(userID)
	if err != nil {
		return nil, apperror.Internal("Failed to retrieve invitations")
	}
	return invitations, nil
}

func (u *workspaceUsecase) AcceptInvitation(invitationID uint, userID uint) error {
	inv, err := u.workspaceRepo.FindInvitationByID(invitationID)
	if err != nil || inv.InvitedID != userID {
		return apperror.NotFound("Invitation not found")
	}
	if inv.Status != "pending" {
		return apperror.UnprocessableEntity("This invitation has already been processed")
	}

	return u.workspaceRepo.AcceptInvitation(inv)
}

func (u *workspaceUsecase) RejectInvitation(invitationID uint, userID uint) error {
	inv, err := u.workspaceRepo.FindInvitationByID(invitationID)
	if err != nil || inv.InvitedID != userID {
		return apperror.NotFound("Invitation not found")
	}
	if inv.Status != "pending" {
		return apperror.UnprocessableEntity("This invitation has already been processed")
	}

	inv.Status = "rejected"
	return u.workspaceRepo.UpdateInvitationStatus(inv)
}

// 4. MANAGEMENT & INTEGRATION
func (u *workspaceUsecase) UpgradeTier(userID uint, newTier string) error {
	return u.authRepo.UpdateTier(userID, newTier)
}

func (u *workspaceUsecase) InitGroupConnection(telegramUserID int64, workspaceID uint, telegramChatID int64) error {
	user, err := u.authRepo.GetByTelegramID(telegramUserID)
	if err != nil || user == nil {
		return apperror.Unauthorized("Telegram account not linked. Please use /bind in private chat first")
	}

	ws, err := u.workspaceRepo.GetByIDAndOwner(workspaceID, user.ID)
	if err != nil {
		return apperror.NotFound("Workspace not found or unauthorized access")
	}

	return u.workspaceRepo.ConnectToTelegramGroup(ws.ID, telegramChatID)
}

func (u *workspaceUsecase) GetUserWorkspaceList(telegramUserID int64) (string, error) {
	user, err := u.authRepo.GetByTelegramID(telegramUserID)
	if err != nil || user == nil {
		return "", apperror.Unauthorized("Telegram account not linked. Please use /bind first")
	}

	workspaces, err := u.workspaceRepo.GetWorkspacesByOwner(user.ID)
	if err != nil {
		return "", apperror.Internal("Failed to retrieve workspace list")
	}

	if len(workspaces) == 0 {
		return "You do not have any workspaces registered yet.", nil
	}

	var sb strings.Builder
	sb.WriteString("📂 **Your Workspace List:**\n\n")
	for _, ws := range workspaces {
		sb.WriteString(fmt.Sprintf("🔹 **%s**\n   └ ID: `%d`\n", ws.Name, ws.ID))
	}
	sb.WriteString("\nType `/init [ID]` in your group to connect.")

	return sb.String(), nil
}

func (u *workspaceUsecase) CreateFromTelegram(ctx context.Context, telegramID int64, chatTitle string, chatID int64, wsType string) (*models.Workspace, error) {
	user, err := u.authRepo.GetByTelegramID(telegramID)
	if err != nil || user == nil {
		return nil, apperror.Unauthorized("Telegram account not linked. Please bind your account in private chat first")
	}

	// FIX: Panggil helper method yang sama! Gak perlu copy-paste logic tier lagi!
	if err := u.checkWorkspaceLimit(user.ID); err != nil {
		return nil, err
	}

	workspace := &models.Workspace{
		Name:           chatTitle,
		OwnerID:        user.ID,
		TelegramChatID: &chatID,
		Type:           wsType,
	}

	if err := u.workspaceRepo.Create(workspace); err != nil {
		return nil, apperror.Internal("Failed to create workspace via Telegram")
	}

	// FIX: Wajib panggil seed category biar workspace dari Telegram gak rusak!
	u.seedDefaultCategories(workspace.ID)

	return workspace, nil
}

func (u *workspaceUsecase) GetMembers(workspaceID uint) ([]models.WorkspaceMember, error) {
	return u.workspaceRepo.GetMembersByWorkspaceID(workspaceID)
}

package usecase

import (
	"errors"
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
// 3. INVITATION LOGIC (Updated to Email)
func (u *workspaceUsecase) InviteMember(workspaceID uint, ownerID uint, email string) error {
	// 1. Cari User ID berdasarkan Email pake method yang udah lu buat
	invitedUser, err := u.authRepo.FindByEmail(email)
	if err != nil {
		return apperror.NotFound("User dengan email tersebut tidak ditemukan!")
	}

	if ownerID == invitedUser.ID {
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
	isAlreadyMember, _ := u.workspaceRepo.IsMember(workspaceID, invitedUser.ID)
	if isAlreadyMember {
		return apperror.UnprocessableEntity("This user is already a member of this workspace.")
	}

	invitation := &models.WorkspaceInvitation{
		WorkspaceID: workspaceID,
		InviterID:   ownerID,
		InvitedID:   invitedUser.ID, // Tetap simpan ID ke DB
		Status:      "pending",
	}

	if err := u.workspaceRepo.CreateInvitation(invitation); err != nil {
		return apperror.Internal("Failed to create workspace invitation.")
	}

	return nil
}

func (u *workspaceUsecase) GetPendingInvitations(userID uint) ([]models.WorkspaceInvitation, error) {
	return u.workspaceRepo.FindPendingInvitationsByUserID(userID)
}

// Pecah jadi dua fungsi terpisah
func (u *workspaceUsecase) AcceptInvitation(invitationID uint, userID uint) error {
	inv, err := u.workspaceRepo.FindInvitationByID(invitationID)
	if err != nil || inv.InvitedID != userID {
		return apperror.NotFound("Invitation not found!")
	}
	if inv.Status != "pending" {
		return apperror.UnprocessableEntity("This invitation has already been processed!")
	}

	return u.workspaceRepo.AcceptInvitation(inv)
}

func (u *workspaceUsecase) RejectInvitation(invitationID uint, userID uint) error {
	inv, err := u.workspaceRepo.FindInvitationByID(invitationID)
	if err != nil || inv.InvitedID != userID {
		return apperror.NotFound("Invitation not found!")
	}
	if inv.Status != "pending" {
		return apperror.UnprocessableEntity("This invitation has already been processed!")
	}

	inv.Status = "rejected"
	return u.workspaceRepo.UpdateInvitationStatus(inv)
}

// 4. UPGRADE SIMULATION
func (u *workspaceUsecase) UpgradeTier(userID uint, newTier string) error {
	// Karena lo butuh update field di User, pastikan AuthRepo punya fungsi UpdateTier
	return u.authRepo.UpdateTier(userID, newTier)
}

func (u *workspaceUsecase) InitGroupConnection(telegramUserID int64, workspaceID uint, telegramChatID int64) error {
	// 1. Cari user berdasarkan Telegram ID
	user, err := u.authRepo.GetByTelegramID(telegramUserID)
	if err != nil || user == nil {
		return errors.New("lu belum binding akun, Mi! Ketik /bind dulu di private chat bot")
	}

	// 2. Pastiin workspace itu milik si user
	ws, err := u.workspaceRepo.GetByIDAndOwner(workspaceID, user.ID)
	if err != nil {
		return errors.New("workspace gak ketemu atau lu bukan owner-nya!")
	}

	// 3. Update telegram_chat_id di workspace tersebut
	return u.workspaceRepo.ConnectToTelegramGroup(ws.ID, telegramChatID)
}

func (u *workspaceUsecase) GetUserWorkspaceList(telegramUserID int64) (string, error) {
	// 1. Cari usernya dulu
	user, err := u.authRepo.GetByTelegramID(telegramUserID)
	if err != nil || user == nil {
		return "", errors.New("lu belum binding akun, Mi! Ketik /bind dulu.")
	}

	// 2. Ambil semua workspacenya
	workspaces, err := u.workspaceRepo.GetWorkspacesByOwner(user.ID)
	if err != nil {
		return "", err
	}

	if len(workspaces) == 0 {
		return "Lu belum punya workspace apa-apa di Web, Mi.", nil
	}

	// 3. Susun jadi teks yang rapi
	var sb strings.Builder
	sb.WriteString("📂 **Daftar Workspace Lu:**\n\n")
	for _, ws := range workspaces {
		sb.WriteString(fmt.Sprintf("🔹 **%s**\n   └ ID: `%d`\n", ws.Name, ws.ID))
	}
	sb.WriteString("\nKetik `/init [ID]` di Grup buat hubungin.")

	return sb.String(), nil
}

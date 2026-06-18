package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ==========================================
// MOCK WORKSPACE REPOSITORY
// ==========================================
type MockWorkspaceRepository struct {
	mock.Mock
}

func (m *MockWorkspaceRepository) FindByOwnerID(ownerID uint) ([]models.Workspace, error) {
	args := m.Called(ownerID)
	if args.Get(0) != nil {
		return args.Get(0).([]models.Workspace), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWorkspaceRepository) GetByTelegramChatID(chatID int64) (*models.Workspace, error) {
	return nil, nil
}

func (m *MockWorkspaceRepository) Create(workspace *models.Workspace) error {
	args := m.Called(workspace)
	return args.Error(0)
}

// Tambahkan sisa method interface WorkspaceRepository dengan implementasi kosong agar tidak error
func (m *MockWorkspaceRepository) FindAllByUserID(userID uint) ([]models.Workspace, error) {
	args := m.Called(userID)
	if args.Get(0) != nil {
		return args.Get(0).([]models.Workspace), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWorkspaceRepository) FindByID(id uint) (*models.Workspace, error) {
	args := m.Called(id)
	if args.Get(0) != nil {
		return args.Get(0).(*models.Workspace), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWorkspaceRepository) Update(workspace *models.Workspace) error {
	args := m.Called(workspace)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) Delete(id uint) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) CreateInvitation(invitation *models.WorkspaceInvitation) error {
	args := m.Called(invitation)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) FindPendingInvitationsByUserID(userID uint) ([]models.WorkspaceInvitation, error) {
	args := m.Called(userID)
	if args.Get(0) != nil {
		return args.Get(0).([]models.WorkspaceInvitation), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWorkspaceRepository) FindInvitationByID(id uint) (*models.WorkspaceInvitation, error) {
	args := m.Called(id)
	if args.Get(0) != nil {
		return args.Get(0).(*models.WorkspaceInvitation), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWorkspaceRepository) AcceptInvitation(invitation *models.WorkspaceInvitation) error {
	args := m.Called(invitation)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) UpdateInvitationStatus(invitation *models.WorkspaceInvitation) error {
	args := m.Called(invitation)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) GetMembersCount(workspaceID uint) (int, error) {
	args := m.Called(workspaceID)
	return args.Int(0), args.Error(1)
}

func (m *MockWorkspaceRepository) IsMember(workspaceID uint, userID uint) (bool, error) {
	args := m.Called(workspaceID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockWorkspaceRepository) GetByIDAndOwner(id uint, ownerID uint) (*models.Workspace, error) {
	args := m.Called(id, ownerID)
	if args.Get(0) != nil {
		return args.Get(0).(*models.Workspace), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWorkspaceRepository) ConnectToTelegramGroup(workspaceID uint, chatID int64) error {
	args := m.Called(workspaceID, chatID)
	return args.Error(0)
}

func (m *MockWorkspaceRepository) GetWorkspacesByOwner(ownerID uint) ([]models.Workspace, error) {
	args := m.Called(ownerID)
	if args.Get(0) != nil {
		return args.Get(0).([]models.Workspace), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWorkspaceRepository) GetMembersByWorkspaceID(workspaceID uint) ([]models.WorkspaceMember, error) {
	args := m.Called(workspaceID)
	if args.Get(0) != nil {
		return args.Get(0).([]models.WorkspaceMember), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWorkspaceRepository) CalculateSummary(workspaceID int) (map[string]float64, error) {
	args := m.Called(workspaceID)
	if args.Get(0) != nil {
		return args.Get(0).(map[string]float64), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWorkspaceRepository) GetBudgetByWorkspace(workspaceID int, period string) (float64, float64, error) {
	args := m.Called(workspaceID, period)
	return args.Get(0).(float64), args.Get(1).(float64), args.Error(2)
}

// ==========================================
// MOCK CATEGORY REPOSITORY
// ==========================================
type MockCategoryRepository struct {
	mock.Mock
}

func (m *MockCategoryRepository) Create(category *models.Category) error {
	args := m.Called(category)
	return args.Error(0)
}
func (m *MockCategoryRepository) FindByID(id uint) (*models.Category, error) { return nil, nil }
func (m *MockCategoryRepository) GetByWorkspace(workspaceID uint) ([]models.Category, error) {
	return nil, nil
}

// ==========================================
// TEST SCENARIOS
// ==========================================

func TestCreateWorkspace(t *testing.T) {
	t.Run("Should return not found error when user does not exist", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		mockAuthRepo.On("FindByID", uint(1)).Return(nil, errors.New("not found"))

		wsUC := NewWorkspaceUsecase(nil, mockAuthRepo, nil, nil)

		ws, err := wsUC.CreateWorkspace("My Workspace", "budgeting", 1)

		assert.NotNil(t, err)
		assert.Nil(t, ws)
		assert.Contains(t, err.Error(), "User not found")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return limit reached error when free user already has 2 workspaces", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		user := &models.User{AccountTier: "free"}
		user.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(user, nil)

		// FIX GORM MODEL: Deklarasiin dulu, baru tembak ID-nya satu per satu
		ws1 := models.Workspace{}
		ws1.ID = 10
		ws2 := models.Workspace{}
		ws2.ID = 11
		existingWorkspaces := []models.Workspace{ws1, ws2}

		mockWsRepo.On("FindByOwnerID", uint(1)).Return(existingWorkspaces, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)

		ws, err := wsUC.CreateWorkspace("Third Workspace", "budgeting", 1)

		assert.NotNil(t, err)
		assert.Nil(t, ws)
		assert.Contains(t, err.Error(), "Workspace limit reached")

		mockAuthRepo.AssertExpectations(t)
		mockWsRepo.AssertExpectations(t)
	})

	t.Run("Should return bad request error for invalid workspace type", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		user := &models.User{AccountTier: "pro"}
		user.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(user, nil)
		mockWsRepo.On("FindByOwnerID", uint(1)).Return([]models.Workspace{}, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)

		ws, err := wsUC.CreateWorkspace("Invalid WS", "gaming", 1)

		assert.NotNil(t, err)
		assert.Nil(t, ws)
		assert.Contains(t, err.Error(), "harus 'budgeting' atau 'split'")

		mockAuthRepo.AssertExpectations(t)
		mockWsRepo.AssertExpectations(t)
	})

	t.Run("Should return internal error when database fails to create workspace", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		user := &models.User{AccountTier: "free"}
		user.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(user, nil)
		mockWsRepo.On("FindByOwnerID", uint(1)).Return([]models.Workspace{}, nil)

		dbError := errors.New("db insert failed")
		mockWsRepo.On("Create", mock.AnythingOfType("*models.Workspace")).Return(dbError)

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)

		ws, err := wsUC.CreateWorkspace("Fail WS", "budgeting", 1)

		assert.NotNil(t, err)
		assert.Nil(t, ws)
		assert.Contains(t, err.Error(), "Failed to create workspace")

		mockAuthRepo.AssertExpectations(t)
		mockWsRepo.AssertExpectations(t)
	})

	t.Run("Should successfully create workspace and seed default categories", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)
		mockCategoryRepo := new(MockCategoryRepository)

		user := &models.User{AccountTier: "pro"}
		user.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(user, nil)
		mockWsRepo.On("FindByOwnerID", uint(1)).Return([]models.Workspace{}, nil)

		mockWsRepo.On("Create", mock.AnythingOfType("*models.Workspace")).Return(nil).Run(func(args mock.Arguments) {
			w := args.Get(0).(*models.Workspace)
			w.ID = 99
		})

		mockCategoryRepo.On("Create", mock.AnythingOfType("*models.Category")).Return(nil).Times(5)

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, mockCategoryRepo, nil)

		ws, err := wsUC.CreateWorkspace("My First WS", "", 1)

		assert.Nil(t, err)
		assert.NotNil(t, ws)
		assert.Equal(t, "My First WS", ws.Name)
		assert.Equal(t, "budgeting", ws.Type)

		mockAuthRepo.AssertExpectations(t)
		mockWsRepo.AssertExpectations(t)
		mockCategoryRepo.AssertExpectations(t)
	})
	t.Run("Should return limit reached error when pro user already has 10 workspaces", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		user := &models.User{AccountTier: "pro"}
		user.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(user, nil)

		// Simulate user already having 10 workspaces
		existingWorkspaces := make([]models.Workspace, 10)
		mockWsRepo.On("FindByOwnerID", uint(1)).Return(existingWorkspaces, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)
		ws, err := wsUC.CreateWorkspace("Eleventh Workspace", "budgeting", 1)

		assert.NotNil(t, err)
		assert.Nil(t, ws)
		assert.Contains(t, err.Error(), "PRO tier is limited to 10 workspaces")

		mockAuthRepo.AssertExpectations(t)
		mockWsRepo.AssertExpectations(t)
	})
}

func TestGetUserWorkspaces(t *testing.T) {
	t.Run("Should return internal error when database fails to retrieve workspaces", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		mockWsRepo.On("FindAllByUserID", uint(1)).Return(nil, errors.New("db error"))

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)

		workspaces, err := wsUC.GetUserWorkspaces(1)

		assert.NotNil(t, err)
		assert.Nil(t, workspaces)
		assert.Contains(t, err.Error(), "Failed to retrieve workspaces")

		mockWsRepo.AssertExpectations(t)
	})

	t.Run("Should successfully return user workspaces", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		expectedWorkspaces := []models.Workspace{
			{Name: "Family Budget"},
			{Name: "Trip Split"},
		}
		mockWsRepo.On("FindAllByUserID", uint(1)).Return(expectedWorkspaces, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)

		workspaces, err := wsUC.GetUserWorkspaces(1)

		assert.Nil(t, err)
		assert.Len(t, workspaces, 2)
		assert.Equal(t, "Family Budget", workspaces[0].Name)

		mockWsRepo.AssertExpectations(t)
	})
}

func TestUpdateWorkspace(t *testing.T) {
	t.Run("Should return not found error when workspace does not exist", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		mockWsRepo.On("FindByID", uint(1)).Return(nil, errors.New("not found"))

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)

		err := wsUC.UpdateWorkspace(1, 1, "New Name")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Workspace not found")

		mockWsRepo.AssertExpectations(t)
	})

	t.Run("Should return forbidden error when user is not the owner", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		// Simulate finding the workspace, but the OwnerID (2) doesn't match the requester ID (1)
		existingWS := &models.Workspace{OwnerID: 2}
		mockWsRepo.On("FindByID", uint(1)).Return(existingWS, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)

		err := wsUC.UpdateWorkspace(1, 1, "New Name")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Access denied")

		mockWsRepo.AssertExpectations(t)
	})

	t.Run("Should successfully update workspace name when user is the owner", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		existingWS := &models.Workspace{OwnerID: 1, Name: "Old Name"}
		mockWsRepo.On("FindByID", uint(1)).Return(existingWS, nil)

		// Use .Run to verify the usecase actually changed the name inside the object before saving
		mockWsRepo.On("Update", mock.AnythingOfType("*models.Workspace")).Return(nil).Run(func(args mock.Arguments) {
			w := args.Get(0).(*models.Workspace)
			assert.Equal(t, "New Name", w.Name)
		})

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)

		err := wsUC.UpdateWorkspace(1, 1, "New Name")

		assert.Nil(t, err)

		mockWsRepo.AssertExpectations(t)
	})
}

func TestDeleteWorkspace(t *testing.T) {
	t.Run("Should return not found error when workspace does not exist", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		mockWsRepo.On("FindByID", uint(1)).Return(nil, errors.New("not found"))

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)

		err := wsUC.DeleteWorkspace(1, 1)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Workspace not found")

		mockWsRepo.AssertExpectations(t)
	})

	t.Run("Should return forbidden error when user is not the owner", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		existingWS := &models.Workspace{OwnerID: 2}
		mockWsRepo.On("FindByID", uint(1)).Return(existingWS, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)

		err := wsUC.DeleteWorkspace(1, 1)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Access denied")

		mockWsRepo.AssertExpectations(t)
	})

	t.Run("Should successfully delete workspace when user is the owner", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		existingWS := &models.Workspace{OwnerID: 1}
		mockWsRepo.On("FindByID", uint(1)).Return(existingWS, nil)
		mockWsRepo.On("Delete", uint(1)).Return(nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)

		err := wsUC.DeleteWorkspace(1, 1)

		assert.Nil(t, err)

		mockWsRepo.AssertExpectations(t)
	})
}

func TestInviteMember(t *testing.T) {
	t.Run("Should return not found when invited email does not exist", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockAuthRepo.On("FindByEmail", "ghost@example.com").Return(nil, errors.New("not found"))

		wsUC := NewWorkspaceUsecase(nil, mockAuthRepo, nil, nil)
		err := wsUC.InviteMember(1, 1, "ghost@example.com")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "email was not found")
	})

	t.Run("Should return bad request when trying to invite oneself", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		myself := &models.User{}
		myself.ID = 1
		mockAuthRepo.On("FindByEmail", "me@example.com").Return(myself, nil)

		wsUC := NewWorkspaceUsecase(nil, mockAuthRepo, nil, nil)
		err := wsUC.InviteMember(1, 1, "me@example.com") // Inviter is 1, Invited is 1

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "cannot invite yourself")
	})

	t.Run("Should return forbidden when inviter is not the owner", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		invitedUser := &models.User{}
		invitedUser.ID = 2
		mockAuthRepo.On("FindByEmail", "friend@example.com").Return(invitedUser, nil)

		// Workspace owner is user 99, but inviter is user 1
		ws := &models.Workspace{OwnerID: 99}
		mockWsRepo.On("FindByID", uint(1)).Return(ws, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)
		err := wsUC.InviteMember(1, 1, "friend@example.com")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "not the owner")
	})

	t.Run("Should return error when free tier limit is reached", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		invitedUser := &models.User{}
		invitedUser.ID = 2
		mockAuthRepo.On("FindByEmail", "friend@example.com").Return(invitedUser, nil)

		ws := &models.Workspace{OwnerID: 1}
		mockWsRepo.On("FindByID", uint(1)).Return(ws, nil)

		inviterUser := &models.User{AccountTier: "free"}
		inviterUser.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(inviterUser, nil)

		// Simulate workspace already has 2 members
		mockWsRepo.On("GetMembersCount", uint(1)).Return(2, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)
		err := wsUC.InviteMember(1, 1, "friend@example.com")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Member limit reached")
	})

	t.Run("Should successfully invite member", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		invitedUser := &models.User{}
		invitedUser.ID = 2
		mockAuthRepo.On("FindByEmail", "friend@example.com").Return(invitedUser, nil)

		ws := &models.Workspace{OwnerID: 1}
		mockWsRepo.On("FindByID", uint(1)).Return(ws, nil)

		inviterUser := &models.User{AccountTier: "pro"}
		inviterUser.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(inviterUser, nil)
		mockWsRepo.On("GetMembersCount", uint(1)).Return(5, nil)

		// User is not a member yet
		mockWsRepo.On("IsMember", uint(1), uint(2)).Return(false, nil)
		mockWsRepo.On("CreateInvitation", mock.AnythingOfType("*models.WorkspaceInvitation")).Return(nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)
		err := wsUC.InviteMember(1, 1, "friend@example.com")

		assert.Nil(t, err)
	})
	t.Run("Should return conflict when user is already a member", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		invitedUser := &models.User{}
		invitedUser.ID = 2
		mockAuthRepo.On("FindByEmail", "friend@example.com").Return(invitedUser, nil)

		ws := &models.Workspace{OwnerID: 1}
		mockWsRepo.On("FindByID", uint(1)).Return(ws, nil)

		inviterUser := &models.User{AccountTier: "pro"}
		inviterUser.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(inviterUser, nil)
		mockWsRepo.On("GetMembersCount", uint(1)).Return(5, nil)

		// Simulate the user is ALREADY inside the workspace
		mockWsRepo.On("IsMember", uint(1), uint(2)).Return(true, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)
		err := wsUC.InviteMember(1, 1, "friend@example.com")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "already a member")
	})

	t.Run("Should return internal error when database fails to create invitation", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		invitedUser := &models.User{}
		invitedUser.ID = 2
		mockAuthRepo.On("FindByEmail", "friend@example.com").Return(invitedUser, nil)

		ws := &models.Workspace{OwnerID: 1}
		mockWsRepo.On("FindByID", uint(1)).Return(ws, nil)

		inviterUser := &models.User{AccountTier: "pro"}
		inviterUser.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(inviterUser, nil)
		mockWsRepo.On("GetMembersCount", uint(1)).Return(5, nil)

		mockWsRepo.On("IsMember", uint(1), uint(2)).Return(false, nil)

		// Simulate database crash when saving invitation
		mockWsRepo.On("CreateInvitation", mock.AnythingOfType("*models.WorkspaceInvitation")).Return(errors.New("db insert fail"))

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)
		err := wsUC.InviteMember(1, 1, "friend@example.com")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Failed to create workspace invitation")
	})
}

func TestInvitationActions(t *testing.T) {
	t.Run("GetPendingInvitations - Should return list", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)
		mockWsRepo.On("FindPendingInvitationsByUserID", uint(1)).Return([]models.WorkspaceInvitation{{Status: "pending"}}, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)
		invs, err := wsUC.GetPendingInvitations(1)

		assert.Nil(t, err)
		assert.Len(t, invs, 1)
	})

	t.Run("AcceptInvitation - Should return error if invitation is processed", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		inv := &models.WorkspaceInvitation{InvitedID: 1, Status: "accepted"}
		mockWsRepo.On("FindInvitationByID", uint(1)).Return(inv, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)
		err := wsUC.AcceptInvitation(1, 1)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "already been processed")
	})

	t.Run("AcceptInvitation - Should successfully accept", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		inv := &models.WorkspaceInvitation{InvitedID: 1, Status: "pending"}
		mockWsRepo.On("FindInvitationByID", uint(1)).Return(inv, nil)
		mockWsRepo.On("AcceptInvitation", inv).Return(nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)
		err := wsUC.AcceptInvitation(1, 1)

		assert.Nil(t, err)
	})

	t.Run("RejectInvitation - Should successfully reject", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		inv := &models.WorkspaceInvitation{InvitedID: 1, Status: "pending"}
		mockWsRepo.On("FindInvitationByID", uint(1)).Return(inv, nil)
		mockWsRepo.On("UpdateInvitationStatus", mock.AnythingOfType("*models.WorkspaceInvitation")).Return(nil).Run(func(args mock.Arguments) {
			i := args.Get(0).(*models.WorkspaceInvitation)
			assert.Equal(t, "rejected", i.Status)
		})

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)
		err := wsUC.RejectInvitation(1, 1)

		assert.Nil(t, err)
	})
	t.Run("GetPendingInvitations - Should return internal error on db failure", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		// Simulate DB error
		mockWsRepo.On("FindPendingInvitationsByUserID", uint(1)).Return(nil, errors.New("db error"))

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)
		invs, err := wsUC.GetPendingInvitations(1)

		assert.NotNil(t, err)
		assert.Nil(t, invs)
		assert.Contains(t, err.Error(), "Failed to retrieve invitations")
	})

	t.Run("AcceptInvitation - Should return not found error", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)
		mockWsRepo.On("FindInvitationByID", uint(1)).Return(nil, errors.New("not found"))

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)
		err := wsUC.AcceptInvitation(1, 1)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Invitation not found")
	})

	t.Run("RejectInvitation - Should return not found error", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)
		mockWsRepo.On("FindInvitationByID", uint(1)).Return(nil, errors.New("not found"))

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)
		err := wsUC.RejectInvitation(1, 1)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Invitation not found")
	})

	t.Run("RejectInvitation - Should return error if invitation is already processed", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		// Simulate an invitation that was already accepted
		inv := &models.WorkspaceInvitation{InvitedID: 1, Status: "accepted"}
		mockWsRepo.On("FindInvitationByID", uint(1)).Return(inv, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)
		err := wsUC.RejectInvitation(1, 1)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "already been processed")
	})
}

func TestUpgradeTier(t *testing.T) {
	t.Run("Should successfully upgrade tier", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockAuthRepo.On("UpdateTier", uint(1), "pro").Return(nil)

		wsUC := NewWorkspaceUsecase(nil, mockAuthRepo, nil, nil)
		err := wsUC.UpgradeTier(1, "pro")

		assert.Nil(t, err)
		mockAuthRepo.AssertExpectations(t)
	})
}

func TestInitGroupConnection(t *testing.T) {
	t.Run("Should return unauthorized if telegram not linked", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockAuthRepo.On("GetByTelegramID", int64(999)).Return(nil, errors.New("not found"))

		wsUC := NewWorkspaceUsecase(nil, mockAuthRepo, nil, nil)
		err := wsUC.InitGroupConnection(999, 1, -12345)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "not linked")
	})

	t.Run("Should return not found if workspace not owned by user", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		user := &models.User{}
		user.ID = 1
		mockAuthRepo.On("GetByTelegramID", int64(999)).Return(user, nil)
		mockWsRepo.On("GetByIDAndOwner", uint(1), uint(1)).Return(nil, errors.New("not found"))

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)
		err := wsUC.InitGroupConnection(999, 1, -12345)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "unauthorized access")
	})

	t.Run("Should successfully connect to group", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		user := &models.User{}
		user.ID = 1
		ws := &models.Workspace{}
		ws.ID = 1

		mockAuthRepo.On("GetByTelegramID", int64(999)).Return(user, nil)
		mockWsRepo.On("GetByIDAndOwner", uint(1), uint(1)).Return(ws, nil)
		mockWsRepo.On("ConnectToTelegramGroup", uint(1), int64(-12345)).Return(nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)
		err := wsUC.InitGroupConnection(999, 1, -12345)

		assert.Nil(t, err)
	})
}

func TestGetUserWorkspaceList(t *testing.T) {
	t.Run("Should handle empty workspace list correctly", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		user := &models.User{}
		user.ID = 1
		mockAuthRepo.On("GetByTelegramID", int64(999)).Return(user, nil)
		mockWsRepo.On("GetWorkspacesByOwner", uint(1)).Return([]models.Workspace{}, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)
		msg, err := wsUC.GetUserWorkspaceList(999)

		assert.Nil(t, err)
		assert.Contains(t, msg, "do not have any workspaces")
	})

	t.Run("Should return formatted string of workspaces", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		user := &models.User{}
		user.ID = 1

		ws1 := models.Workspace{Name: "Alpha"}
		ws1.ID = 10
		ws2 := models.Workspace{Name: "Beta"}
		ws2.ID = 20

		mockAuthRepo.On("GetByTelegramID", int64(999)).Return(user, nil)
		mockWsRepo.On("GetWorkspacesByOwner", uint(1)).Return([]models.Workspace{ws1, ws2}, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)
		msg, err := wsUC.GetUserWorkspaceList(999)

		assert.Nil(t, err)
		assert.Contains(t, msg, "Alpha")
		assert.Contains(t, msg, "`10`")
		assert.Contains(t, msg, "Beta")
	})
	t.Run("Should return error when database fails to retrieve workspace list", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		user := &models.User{}
		user.ID = 1
		mockAuthRepo.On("GetByTelegramID", int64(999)).Return(user, nil)
		mockWsRepo.On("GetWorkspacesByOwner", uint(1)).Return(nil, errors.New("db error"))

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)
		msg, err := wsUC.GetUserWorkspaceList(999)

		assert.NotNil(t, err)
		assert.Empty(t, msg)
		assert.Contains(t, err.Error(), "Failed to retrieve workspace list")
	})
	t.Run("Should return unauthorized error when telegram not linked", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockAuthRepo.On("GetByTelegramID", int64(999)).Return(nil, errors.New("not found"))

		wsUC := NewWorkspaceUsecase(nil, mockAuthRepo, nil, nil)
		msg, err := wsUC.GetUserWorkspaceList(999)

		assert.NotNil(t, err)
		assert.Empty(t, msg)
		assert.Contains(t, err.Error(), "Telegram account not linked")
	})
}

func TestCreateFromTelegram(t *testing.T) {
	// Pake context.Background() karena di interface butuh context
	t.Run("Should successfully create workspace from telegram and seed categories", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)
		mockCategoryRepo := new(MockCategoryRepository)

		user := &models.User{AccountTier: "pro"}
		user.ID = 1
		mockAuthRepo.On("GetByTelegramID", int64(999)).Return(user, nil)
		// Fungsi pembantu checkWorkspaceLimit akan manggil ini
		mockAuthRepo.On("FindByID", uint(1)).Return(user, nil)
		mockWsRepo.On("FindByOwnerID", uint(1)).Return([]models.Workspace{}, nil)

		mockWsRepo.On("Create", mock.AnythingOfType("*models.Workspace")).Return(nil).Run(func(args mock.Arguments) {
			w := args.Get(0).(*models.Workspace)
			w.ID = 77
		})

		// 5 Kategori Default
		mockCategoryRepo.On("Create", mock.AnythingOfType("*models.Category")).Return(nil).Times(5)

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, mockCategoryRepo, nil)

		ws, err := wsUC.CreateFromTelegram(context.Background(), 999, "Telegram WS", -12345, "split")

		assert.Nil(t, err)
		assert.NotNil(t, ws)
		assert.Equal(t, "Telegram WS", ws.Name)
		assert.Equal(t, "split", ws.Type)

		mockAuthRepo.AssertExpectations(t)
		mockWsRepo.AssertExpectations(t)
		mockCategoryRepo.AssertExpectations(t)
	})
	t.Run("Should return error when telegram is not linked", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockAuthRepo.On("GetByTelegramID", int64(999)).Return(nil, errors.New("not found"))

		wsUC := NewWorkspaceUsecase(nil, mockAuthRepo, nil, nil)
		ws, err := wsUC.CreateFromTelegram(context.Background(), 999, "Telegram WS", -12345, "split")

		assert.NotNil(t, err)
		assert.Nil(t, ws)
		assert.Contains(t, err.Error(), "Telegram account not linked")
	})

	t.Run("Should return internal error when database fails to create workspace", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		user := &models.User{AccountTier: "pro"}
		user.ID = 1
		mockAuthRepo.On("GetByTelegramID", int64(999)).Return(user, nil)
		mockAuthRepo.On("FindByID", uint(1)).Return(user, nil)
		mockWsRepo.On("FindByOwnerID", uint(1)).Return([]models.Workspace{}, nil)

		mockWsRepo.On("Create", mock.AnythingOfType("*models.Workspace")).Return(errors.New("db insert failed"))

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)
		ws, err := wsUC.CreateFromTelegram(context.Background(), 999, "Telegram WS", -12345, "split")

		assert.NotNil(t, err)
		assert.Nil(t, ws)
		assert.Contains(t, err.Error(), "Failed to create workspace via Telegram")
	})
	t.Run("Should return error when workspace limit is reached via telegram", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockWsRepo := new(MockWorkspaceRepository)

		user := &models.User{AccountTier: "free"}
		user.ID = 1
		mockAuthRepo.On("GetByTelegramID", int64(999)).Return(user, nil)
		mockAuthRepo.On("FindByID", uint(1)).Return(user, nil)

		// Simulate user already having 2 workspaces (Hitting the Free tier limit)
		ws1 := models.Workspace{}
		ws1.ID = 10
		ws2 := models.Workspace{}
		ws2.ID = 11
		existingWorkspaces := []models.Workspace{ws1, ws2}
		mockWsRepo.On("FindByOwnerID", uint(1)).Return(existingWorkspaces, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, mockAuthRepo, nil, nil)
		ws, err := wsUC.CreateFromTelegram(context.Background(), 999, "Telegram WS", -12345, "split")

		assert.NotNil(t, err)
		assert.Nil(t, ws)
		assert.Contains(t, err.Error(), "Workspace limit reached")
	})
}

func TestGetMembers(t *testing.T) {
	t.Run("Should return error when database fails", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)
		mockWsRepo.On("GetMembersByWorkspaceID", uint(1)).Return(nil, errors.New("db error"))

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)
		members, err := wsUC.GetMembers(1)

		assert.NotNil(t, err)
		assert.Nil(t, members)
		mockWsRepo.AssertExpectations(t)
	})

	t.Run("Should return members successfully", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		expected := []models.WorkspaceMember{{UserID: 2}, {UserID: 3}}
		mockWsRepo.On("GetMembersByWorkspaceID", uint(1)).Return(expected, nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)
		members, err := wsUC.GetMembers(1)

		assert.Nil(t, err)
		assert.Len(t, members, 2)
		mockWsRepo.AssertExpectations(t)
	})
}

func TestGetWorkspaceSummary(t *testing.T) {
	t.Run("Should return error when CalculateSummary fails", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)
		mockWsRepo.On("CalculateSummary", 1).Return(nil, errors.New("calculation failed"))

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)
		summary, err := wsUC.GetWorkspaceSummary(1, "2026-05")

		assert.NotNil(t, err)
		assert.Nil(t, summary)
		assert.Contains(t, err.Error(), "calculation failed")
		mockWsRepo.AssertExpectations(t)
	})

	t.Run("Should handle missing budget gracefully and return summary", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		calcData := map[string]float64{"balance": 1000, "total_income": 2000, "total_expense": 1000}
		mockWsRepo.On("CalculateSummary", 1).Return(calcData, nil)

		// Simulate budget not set/found for this period
		mockWsRepo.On("GetBudgetByWorkspace", 1, "2026-05").Return(float64(0), float64(0), errors.New("not found"))

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)
		summary, err := wsUC.GetWorkspaceSummary(1, "2026-05")

		assert.Nil(t, err)
		assert.Equal(t, float64(1000), summary["total_balance"])
		assert.Equal(t, float64(0), summary["budget_limit"])   // default fallback 0
		assert.Equal(t, float64(0), summary["savings_target"]) // default fallback 0

		// Check the automatic calculation logic (balance * 0.3)
		assert.Equal(t, float64(300), summary["savings_current"])

		mockWsRepo.AssertExpectations(t)
	})

	t.Run("Should return full summary with budget data", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)

		calcData := map[string]float64{"balance": 5000, "total_income": 6000, "total_expense": 1000}
		mockWsRepo.On("CalculateSummary", 1).Return(calcData, nil)
		mockWsRepo.On("GetBudgetByWorkspace", 1, "2026-06").Return(float64(2000), float64(1500), nil)

		wsUC := NewWorkspaceUsecase(mockWsRepo, nil, nil, nil)
		summary, err := wsUC.GetWorkspaceSummary(1, "2026-06")

		assert.Nil(t, err)
		assert.Equal(t, float64(5000), summary["total_balance"])
		assert.Equal(t, float64(2000), summary["budget_limit"])
		assert.Equal(t, float64(1500), summary["savings_target"])

		mockWsRepo.AssertExpectations(t)
	})
}

package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ==========================================
// MOCK TRANSACTION REPOSITORY
// ==========================================
type MockTransactionRepository struct {
	mock.Mock
}

// --- Method yang kita Test Beneran ---
func (m *MockTransactionRepository) Create(tx *models.Transaction) error {
	args := m.Called(tx)
	return args.Error(0)
}
func (m *MockTransactionRepository) IsDuplicate(workspaceID uint, amount float64, merchant string, date time.Time) (bool, error) {
	args := m.Called(workspaceID, amount, merchant, date)
	return args.Bool(0), args.Error(1)
}
func (m *MockTransactionRepository) GetByWorkspaceID(workspaceID uint, page int, limit int) ([]models.Transaction, int64, error) {
	args := m.Called(workspaceID, page, limit)
	if args.Get(0) != nil {
		return args.Get(0).([]models.Transaction), args.Get(1).(int64), args.Error(2)
	}
	return nil, 0, args.Error(2)
}
func (m *MockTransactionRepository) FindByID(tx *models.Transaction, id uint) error {
	args := m.Called(tx, id)
	return args.Error(0)
}
func (m *MockTransactionRepository) Delete(id uint) error {
	args := m.Called(id)
	return args.Error(0)
}
func (m *MockTransactionRepository) HardDelete(id uint) error {
	args := m.Called(id)
	return args.Error(0)
}

// --- Dummy Method (Biar Kompiler Mingkem) ---
func (m *MockTransactionRepository) UpdateStatus(id uint, status string) error { return nil }
func (m *MockTransactionRepository) GetByGmailID(gmailID string) (*models.Transaction, error) {
	return nil, nil
}
func (m *MockTransactionRepository) CreateEmailLog(emailLog *models.EmailParsed) error { return nil }
func (m *MockTransactionRepository) GetEmailLogByGmailID(gmailID string) (*models.EmailParsed, error) {
	return nil, nil
}
func (m *MockTransactionRepository) UpdateEmailLogStatus(id uint, status string) error { return nil }
func (m *MockTransactionRepository) GetPendingEmailLogs(userID uint) ([]models.EmailParsed, error) {
	args := m.Called(userID)
	if args.Get(0) != nil {
		return args.Get(0).([]models.EmailParsed), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockTransactionRepository) GetEmailLogByID(id uint) (*models.EmailParsed, error) {
	return nil, nil
}
func (m *MockTransactionRepository) GetTotalAmountByType(workspaceID uint, txType string, period string) (float64, error) {
	return 0, nil
}
func (m *MockTransactionRepository) GetTotalByMonth(workspaceID uint, period string) (float64, error) {
	return 0, nil
}
func (m *MockTransactionRepository) GetTotalSavings(workspaceID uint, period string) (float64, error) {
	return 0, nil
}
func (m *MockTransactionRepository) GetEmailParsedByID(id uint) (*models.EmailParsed, error) {
	return nil, nil
}
func (m *MockTransactionRepository) DeleteEmailParsed(id uint) error { return nil }
func (m *MockTransactionRepository) GetSummaryByWorkspace(workspaceID uint, txType string, month string) ([]dto.UserTransactionSummary, error) {
	return nil, nil
}
func (m *MockTransactionRepository) GetTotalByWorkspace(workspaceID uint, txType string, month string) (float64, error) {
	return 0, nil
}
func (m *MockTransactionRepository) CreateWithItems(transaction *models.Transaction) error {
	args := m.Called(transaction)
	return args.Error(0)
}
func (m *MockTransactionRepository) GetAllByWorkspaceID(workspaceID uint, month string) ([]models.Transaction, error) {
	args := m.Called(workspaceID, month)
	if args.Get(0) != nil {
		return args.Get(0).([]models.Transaction), args.Error(1)
	}
	return nil, args.Error(1)
}

type MockPendingTransactionRepository struct {
	mock.Mock
}

func (m *MockPendingTransactionRepository) FindByID(id uint) (*models.PendingTransaction, error) {
	args := m.Called(id)
	if args.Get(0) != nil {
		return args.Get(0).(*models.PendingTransaction), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPendingTransactionRepository) UpdateStatus(id uint, status string) error {
	args := m.Called(id, status)
	return args.Error(0)
}

func (m *MockPendingTransactionRepository) Create(pending *models.PendingTransaction) error {
	args := m.Called(pending)
	return args.Error(0)
}

func (m *MockPendingTransactionRepository) GetPendingList(workspaceID uint, page int, limit int) ([]models.PendingTransaction, int64, error) {
	return nil, 0, nil
}

// ==========================================
// MOCK TARGET USECASE (Bukan repo, karena tx usecase manggil target usecase)
// ==========================================
type MockTargetUsecase struct {
	mock.Mock
}

func (m *MockTargetUsecase) AddIncomeToTarget(workspaceID uint, amount float64) error {
	args := m.Called(workspaceID, amount)
	return args.Error(0)
}
func (m *MockTargetUsecase) CheckWorkspaceTarget(workspaceID uint) (*dto.BudgetStatusResponse, error) {
	args := m.Called(workspaceID)
	if args.Get(0) != nil {
		return args.Get(0).(*dto.BudgetStatusResponse), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockTargetUsecase) SetTarget(req dto.SetTargetRequest) error { return nil }

// ==========================================
// TEST SCENARIOS
// ==========================================

func TestCreateManual(t *testing.T) {
	req := dto.CreateTransactionRequest{
		WorkspaceID: 1,
		Amount:      50000,
		Merchant:    "Warung Nasi",
		Type:        "expense",
		Date:        time.Now(),
	}

	t.Run("Should return error if user is not a member of the workspace", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository) // Ambil dari mock yg udah lu bikin di file sebelah
		mockWsRepo.On("IsMember", uint(1), uint(1)).Return(false, nil)

		txUC := NewTransactionUsecase(nil, nil, nil, nil, mockWsRepo, nil, nil, nil, nil)
		tx, budget, err := txUC.CreateManual(context.Background(), 1, req)

		assert.NotNil(t, err)
		assert.Nil(t, tx)
		assert.Nil(t, budget)
		assert.Contains(t, err.Error(), "not a member")
	})

	t.Run("Should return conflict error if transaction is a duplicate", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)
		mockTxRepo := new(MockTransactionRepository)

		mockWsRepo.On("IsMember", uint(1), uint(1)).Return(true, nil)
		mockTxRepo.On("IsDuplicate", uint(1), float64(50000), "warung nasi", req.Date).Return(true, nil)

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, mockWsRepo, nil, nil, nil, nil)
		tx, budget, err := txUC.CreateManual(context.Background(), 1, req)

		assert.NotNil(t, err)
		assert.Nil(t, tx)
		assert.Nil(t, budget)
		assert.Contains(t, err.Error(), "similar transaction has already been recorded")
	})

	t.Run("Should successfully create expense transaction", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)
		mockTxRepo := new(MockTransactionRepository)
		mockTargetUC := new(MockTargetUsecase)

		mockWsRepo.On("IsMember", uint(1), uint(1)).Return(true, nil)
		mockTxRepo.On("IsDuplicate", uint(1), float64(50000), "warung nasi", req.Date).Return(false, nil)
		mockTxRepo.On("Create", mock.AnythingOfType("*models.Transaction")).Return(nil)

		// The Guardian akan ngasih laporan budget
		mockTargetUC.On("CheckWorkspaceTarget", uint(1)).Return(&dto.BudgetStatusResponse{RemainingBudget: 100000}, nil)

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, mockWsRepo, nil, nil, nil, mockTargetUC)
		tx, budget, err := txUC.CreateManual(context.Background(), 1, req)

		assert.Nil(t, err)
		assert.NotNil(t, tx)
		assert.Equal(t, "warung nasi", tx.Merchant)
		assert.Equal(t, float64(100000), budget.RemainingBudget)
	})

	t.Run("Should successfully create income transaction and add to target", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)
		mockTxRepo := new(MockTransactionRepository)
		mockTargetUC := new(MockTargetUsecase)

		incomeReq := req
		incomeReq.Type = "income"
		incomeReq.Amount = 5000000

		mockWsRepo.On("IsMember", uint(1), uint(1)).Return(true, nil)
		mockTxRepo.On("IsDuplicate", uint(1), float64(5000000), "warung nasi", incomeReq.Date).Return(false, nil)
		mockTxRepo.On("Create", mock.AnythingOfType("*models.Transaction")).Return(nil)

		// Khusus Income, dia bakal nambahin tabungan
		mockTargetUC.On("AddIncomeToTarget", uint(1), float64(5000000)).Return(nil)
		mockTargetUC.On("CheckWorkspaceTarget", uint(1)).Return(&dto.BudgetStatusResponse{}, nil)

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, mockWsRepo, nil, nil, nil, mockTargetUC)
		tx, _, err := txUC.CreateManual(context.Background(), 1, incomeReq)

		assert.Nil(t, err)
		assert.Equal(t, "income", tx.Type)
		mockTargetUC.AssertCalled(t, "AddIncomeToTarget", uint(1), float64(5000000))
	})
	t.Run("Should return error if category is invalid", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)
		mockCategoryRepo := new(MockCategoryRepository)
		mockTxRepo := new(MockTransactionRepository)

		mockWsRepo.On("IsMember", uint(1), uint(1)).Return(true, nil)
		mockTxRepo.On("IsDuplicate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

		catID := uint(99)
		reqWithCat := req
		reqWithCat.CategoryID = &catID

		// Simulasi kategori nggak ketemu
		mockCategoryRepo.On("FindByID", catID).Return(nil, errors.New("not found"))

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, mockWsRepo, nil, nil, mockCategoryRepo, nil)
		tx, budget, err := txUC.CreateManual(context.Background(), 1, reqWithCat)

		assert.NotNil(t, err)
		assert.Nil(t, tx)
		assert.Nil(t, budget)
		assert.Contains(t, err.Error(), "invalid category")
	})
	t.Run("Should return error when IsMember check fails", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)
		mockWsRepo.On("IsMember", uint(1), uint(1)).Return(false, errors.New("db connection lost"))

		txUC := NewTransactionUsecase(nil, nil, nil, nil, mockWsRepo, nil, nil, nil, nil)
		tx, budget, err := txUC.CreateManual(context.Background(), 1, req)

		assert.NotNil(t, err)
		assert.Nil(t, tx)
		assert.Nil(t, budget)
	})

	t.Run("Should return internal error when IsDuplicate check fails", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)
		mockTxRepo := new(MockTransactionRepository)

		mockWsRepo.On("IsMember", uint(1), uint(1)).Return(true, nil)
		mockTxRepo.On("IsDuplicate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, errors.New("db error"))

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, mockWsRepo, nil, nil, nil, nil)
		tx, budget, err := txUC.CreateManual(context.Background(), 1, req)

		assert.NotNil(t, err)
		assert.Nil(t, tx)
		assert.Nil(t, budget)
		assert.Contains(t, err.Error(), "failed to check transaction duplicates")
	})

	t.Run("Should return error when failing to save manual transaction", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)
		mockTxRepo := new(MockTransactionRepository)

		mockWsRepo.On("IsMember", uint(1), uint(1)).Return(true, nil)
		mockTxRepo.On("IsDuplicate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
		mockTxRepo.On("Create", mock.AnythingOfType("*models.Transaction")).Return(errors.New("db error"))

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, mockWsRepo, nil, nil, nil, nil)
		tx, budget, err := txUC.CreateManual(context.Background(), 1, req)

		assert.NotNil(t, err)
		assert.Nil(t, tx)
		assert.Nil(t, budget)
		assert.Contains(t, err.Error(), "failed to save manual transaction")
	})

	t.Run("Should return transaction but nil budget if CheckWorkspaceTarget fails", func(t *testing.T) {
		mockWsRepo := new(MockWorkspaceRepository)
		mockTxRepo := new(MockTransactionRepository)
		mockTargetUC := new(MockTargetUsecase)

		mockWsRepo.On("IsMember", uint(1), uint(1)).Return(true, nil)
		mockTxRepo.On("IsDuplicate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
		mockTxRepo.On("Create", mock.Anything).Return(nil)

		// Simulasi Guardian meledak
		mockTargetUC.On("CheckWorkspaceTarget", uint(1)).Return(nil, errors.New("target calculation failed"))

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, mockWsRepo, nil, nil, nil, mockTargetUC)
		tx, budget, err := txUC.CreateManual(context.Background(), 1, req)

		assert.Nil(t, err)
		assert.NotNil(t, tx)
		assert.Nil(t, budget) // Budget jadi nil karena error
	})
}

func TestDeleteTransaction(t *testing.T) {
	t.Run("Should return error if transaction not found", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)
		mockTxRepo.On("FindByID", mock.AnythingOfType("*models.Transaction"), uint(1)).Return(errors.New("not found"))

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, nil, nil, nil, nil, nil)
		err := txUC.DeleteTransaction(context.Background(), 1, 1)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Should return forbidden if user is not the creator", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)

		// Simulate finding a transaction created by UserID 2, but requester is UserID 1
		mockTxRepo.On("FindByID", mock.AnythingOfType("*models.Transaction"), uint(1)).Return(nil).Run(func(args mock.Arguments) {
			tx := args.Get(0).(*models.Transaction)
			tx.UserID = 2
		})

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, nil, nil, nil, nil, nil)
		err := txUC.DeleteTransaction(context.Background(), 1, 1) // Requester is 1

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "not authorized to delete")
	})

	t.Run("Should successfully delete transaction", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)

		mockTxRepo.On("FindByID", mock.AnythingOfType("*models.Transaction"), uint(1)).Return(nil).Run(func(args mock.Arguments) {
			tx := args.Get(0).(*models.Transaction)
			tx.UserID = 1 // Creator matches requester
		})
		mockTxRepo.On("Delete", uint(1)).Return(nil)

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, nil, nil, nil, nil, nil)
		err := txUC.DeleteTransaction(context.Background(), 1, 1)

		assert.Nil(t, err)
	})
}

func TestGetHistoryAndHardDelete(t *testing.T) {
	t.Run("Should get transaction history", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)

		expectedData := []models.Transaction{{Merchant: "Steam"}, {Merchant: "Spotify"}}
		mockTxRepo.On("GetByWorkspaceID", uint(1), 1, 10).Return(expectedData, int64(2), nil)

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, nil, nil, nil, nil, nil)
		data, count, err := txUC.GetHistory(1, 1, 10)

		assert.Nil(t, err)
		assert.Len(t, data, 2)
		assert.Equal(t, int64(2), count)
	})

	t.Run("Should hard delete transaction", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)
		mockTxRepo.On("HardDelete", uint(1)).Return(nil)

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, nil, nil, nil, nil, nil)
		err := txUC.HardDeleteTransaction(1)

		assert.Nil(t, err)
	})
}

func TestConfirmTransaction(t *testing.T) {
	t.Run("Should return not found error if pending transaction does not exist", func(t *testing.T) {
		mockPendingRepo := new(MockPendingTransactionRepository)
		mockPendingRepo.On("FindByID", uint(1)).Return(nil, errors.New("not found"))

		txUC := NewTransactionUsecase(nil, nil, nil, nil, nil, nil, mockPendingRepo, nil, nil)
		tx, budget, err := txUC.ConfirmTransaction(context.Background(), 1, dto.ConfirmTransactionRequest{})

		assert.NotNil(t, err)
		assert.Nil(t, tx)
		assert.Nil(t, budget)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Should return error if transaction is already confirmed", func(t *testing.T) {
		mockPendingRepo := new(MockPendingTransactionRepository)
		pending := &models.PendingTransaction{Status: "confirmed"}
		mockPendingRepo.On("FindByID", uint(1)).Return(pending, nil)

		txUC := NewTransactionUsecase(nil, nil, nil, nil, nil, nil, mockPendingRepo, nil, nil)
		tx, budget, err := txUC.ConfirmTransaction(context.Background(), 1, dto.ConfirmTransactionRequest{})

		assert.NotNil(t, err)
		assert.Nil(t, tx)     // FIX: Tambahin assert ini biar gak declared and not used
		assert.Nil(t, budget) // FIX: Tambahin assert ini biar gak declared and not used
		assert.Contains(t, err.Error(), "already confirmed")
	})

	t.Run("Should successfully confirm transaction with items", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)
		mockPendingRepo := new(MockPendingTransactionRepository)
		mockTargetUC := new(MockTargetUsecase)

		rawJSON := `{"workspace_id": 1, "type": "expense", "amount": 10000}`
		pending := &models.PendingTransaction{Status: "pending", RawData: rawJSON}
		mockPendingRepo.On("FindByID", uint(1)).Return(pending, nil)

		req := dto.ConfirmTransactionRequest{
			Date: "2026-05-20T15:00:00Z",
			// FIX: Ganti TransactionItemRequest jadi TransactionItemConfirm sesuai DTO lu
			Items: []dto.TransactionItemConfirm{
				{Description: "Ayam Goreng", Quantity: 1, Price: 15000},
			},
		}

		mockTxRepo.On("CreateWithItems", mock.AnythingOfType("*models.Transaction")).Return(nil)
		mockPendingRepo.On("UpdateStatus", uint(1), "confirmed").Return(nil)
		mockTargetUC.On("CheckWorkspaceTarget", uint(1)).Return(&dto.BudgetStatusResponse{}, nil)

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, nil, nil, mockPendingRepo, nil, mockTargetUC)
		tx, budget, err := txUC.ConfirmTransaction(context.Background(), 1, req)

		assert.Nil(t, err)
		assert.NotNil(t, tx)
		assert.NotNil(t, budget)
		assert.Equal(t, "approved", tx.Status)
		assert.Len(t, tx.TransactionItems, 1)

		mockPendingRepo.AssertExpectations(t)
		mockTxRepo.AssertExpectations(t)
	})
	t.Run("Should return error on invalid JSON", func(t *testing.T) {
		mockPendingRepo := new(MockPendingTransactionRepository)

		// Simulasi raw data yang corrupt/bukan JSON
		pending := &models.PendingTransaction{Status: "pending", RawData: "Bukan JSON"}
		mockPendingRepo.On("FindByID", uint(1)).Return(pending, nil)

		txUC := NewTransactionUsecase(nil, nil, nil, nil, nil, nil, mockPendingRepo, nil, nil)
		tx, budget, err := txUC.ConfirmTransaction(context.Background(), 1, dto.ConfirmTransactionRequest{})

		assert.NotNil(t, err)
		assert.Nil(t, tx)
		assert.Nil(t, budget)
		assert.Contains(t, err.Error(), "Failed to parse pending data")
	})

	t.Run("Should parse short date and return error if DB create without items fails", func(t *testing.T) {
		mockPendingRepo := new(MockPendingTransactionRepository)
		mockTxRepo := new(MockTransactionRepository)

		rawJSON := `{"workspace_id": 1, "type": "expense", "amount": 10000}`
		pending := &models.PendingTransaction{Status: "pending", RawData: rawJSON}
		mockPendingRepo.On("FindByID", uint(1)).Return(pending, nil)

		req := dto.ConfirmTransactionRequest{
			Date:  "2026-05-20",                   // Pakai format non-RFC3339 untuk ngetrigger fallback parse
			Items: []dto.TransactionItemConfirm{}, // 0 Items buat ngetrigger u.repo.Create biasa
		}

		mockTxRepo.On("Create", mock.AnythingOfType("*models.Transaction")).Return(errors.New("db error"))

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, nil, nil, mockPendingRepo, nil, nil)
		tx, budget, err := txUC.ConfirmTransaction(context.Background(), 1, req)

		assert.NotNil(t, err)
		assert.Nil(t, tx)
		assert.Nil(t, budget)
		assert.Contains(t, err.Error(), "Failed to save transaction")
	})
	t.Run("Should return error if CreateWithItems fails during confirm", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)
		mockPendingRepo := new(MockPendingTransactionRepository)

		// Bikin pending transaction bohongan
		rawJSON := `{"workspace_id": 1, "type": "expense", "amount": 10000}`
		pending := &models.PendingTransaction{Status: "pending", RawData: rawJSON}
		mockPendingRepo.On("FindByID", uint(1)).Return(pending, nil)

		// Masukin Items biar dia masuk ke blok CreateWithItems
		req := dto.ConfirmTransactionRequest{
			Date: "2026-05-20T15:00:00Z",
			Items: []dto.TransactionItemConfirm{
				{Description: "Ayam Bakar", Price: 10000, Quantity: 1},
			},
		}

		// Simulasi Database tiba-tiba putus/error
		mockTxRepo.On("CreateWithItems", mock.AnythingOfType("*models.Transaction")).Return(errors.New("db meledak"))

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, nil, nil, mockPendingRepo, nil, nil)
		tx, budget, err := txUC.ConfirmTransaction(context.Background(), 1, req)

		assert.NotNil(t, err)
		assert.Nil(t, tx)
		assert.Nil(t, budget)
		assert.Contains(t, err.Error(), "Failed to save transaction with items")
	})
}

func TestConfirmScanTransaction(t *testing.T) {
	t.Run("Should successfully confirm scanned transaction", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)
		mockTargetUC := new(MockTargetUsecase)

		txData := &models.Transaction{WorkspaceID: 1, Type: "income", Amount: 50000}
		items := []models.TransactionItem{{Description: "Item 1", Quantity: 1}}

		mockTxRepo.On("CreateWithItems", txData).Return(nil)
		mockTargetUC.On("AddIncomeToTarget", uint(1), float64(50000)).Return(nil)
		mockTargetUC.On("CheckWorkspaceTarget", uint(1)).Return(&dto.BudgetStatusResponse{}, nil)

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, nil, nil, nil, nil, mockTargetUC)
		budget, err := txUC.ConfirmScanTransaction(context.Background(), txData, items)

		assert.Nil(t, err)
		assert.NotNil(t, budget)

		mockTxRepo.AssertExpectations(t)
		mockTargetUC.AssertExpectations(t)
	})
	t.Run("Should return error if CreateWithItems fails", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)
		txData := &models.Transaction{WorkspaceID: 1}

		// Simulasi DB ngambek pas nyimpen struk
		mockTxRepo.On("CreateWithItems", txData).Return(errors.New("db error"))

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, nil, nil, nil, nil, nil)
		budget, err := txUC.ConfirmScanTransaction(context.Background(), txData, nil)

		assert.NotNil(t, err)
		assert.Nil(t, budget)
		assert.Contains(t, err.Error(), "failed to confirm scanned transaction")
	})
	t.Run("Should return nil budget if target check fails", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)
		mockTargetUC := new(MockTargetUsecase)

		txData := &models.Transaction{WorkspaceID: 1, Type: "expense"}
		mockTxRepo.On("CreateWithItems", txData).Return(nil)

		// Simulasi check target error
		mockTargetUC.On("CheckWorkspaceTarget", uint(1)).Return(nil, errors.New("target error"))

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, nil, nil, nil, nil, mockTargetUC)
		budget, err := txUC.ConfirmScanTransaction(context.Background(), txData, nil)

		assert.Nil(t, err)
		assert.Nil(t, budget) // Budget baliknya nil karena log error
	})
}

func TestProcessTelegramInput(t *testing.T) {
	txUC := NewTransactionUsecase(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	t.Run("Should process income format", func(t *testing.T) {
		txType, isValid, amount := txUC.ProcessTelegramInput(context.Background(), "+ 50000 gajian")
		assert.True(t, isValid)
		assert.Equal(t, "income", txType)
		assert.Equal(t, float64(50000), amount)
	})

	t.Run("Should process expense format", func(t *testing.T) {
		txType, isValid, amount := txUC.ProcessTelegramInput(context.Background(), "- 15000 makan siang")
		assert.True(t, isValid)
		assert.Equal(t, "expense", txType)
		assert.Equal(t, float64(15000), amount)
	})

	t.Run("Should return invalid for wrong format", func(t *testing.T) {
		txType, isValid, amount := txUC.ProcessTelegramInput(context.Background(), "halo bot")
		assert.False(t, isValid)
		assert.Equal(t, "", txType)
		assert.Equal(t, float64(0), amount)
	})
}

func TestProcessScanHybrid_QuotaValidation(t *testing.T) {
	t.Run("Should return error if user not found", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockAuthRepo.On("FindByID", uint(1)).Return(nil, errors.New("user not found"))

		txUC := NewTransactionUsecase(nil, mockAuthRepo, nil, nil, nil, nil, nil, nil, nil)
		res, id, err := txUC.ProcessScanHybrid2(context.Background(), 1, 1, []byte("image"), "image/jpeg")

		assert.NotNil(t, err)
		assert.Nil(t, res)
		assert.Equal(t, uint(0), id)
		assert.Contains(t, err.Error(), "user not found")
	})

	t.Run("Should return limit reached error for Free Tier", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		// Simulasi user Free dengan kuota udah habis (2/2) dan baru di-reset kemarin
		user := &models.User{AccountTier: "free", OCRUsageCount: 2, LastResetUsage: time.Now().Add(-24 * time.Hour)}
		user.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(user, nil)

		txUC := NewTransactionUsecase(nil, mockAuthRepo, nil, nil, nil, nil, nil, nil, nil)
		res, id, err := txUC.ProcessScanHybrid2(context.Background(), 1, 1, []byte("image"), "image/jpeg")

		assert.NotNil(t, err)
		assert.Nil(t, res)
		assert.Equal(t, uint(0), id)
		assert.Contains(t, err.Error(), "Weekly scan limit reached")
	})

	t.Run("Should return auto reset in hours if days left is 0", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		// Simulasi kuota habis dan akan keriset dalam beberapa jam (udah hampir 7 hari)
		almost7DaysAgo := time.Now().Add(-165 * time.Hour)
		user := &models.User{AccountTier: "pro", OCRUsageCount: 10, LastResetUsage: almost7DaysAgo}
		user.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(user, nil)

		txUC := NewTransactionUsecase(nil, mockAuthRepo, nil, nil, nil, nil, nil, nil, nil)
		res, id, err := txUC.ProcessScanHybrid2(context.Background(), 1, 1, []byte("image"), "image/jpeg")

		assert.NotNil(t, err)
		assert.Nil(t, res)
		assert.Equal(t, uint(0), id)
		assert.Contains(t, err.Error(), "Quota empty! Auto reset in")
	})
	t.Run("Should return limit reached error for Ultimate Tier", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		// Simulasi user Ultimate dengan kuota maksimal (50)
		user := &models.User{AccountTier: "ultimate", OCRUsageCount: 50, LastResetUsage: time.Now().Add(-24 * time.Hour)}
		user.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(user, nil)

		txUC := NewTransactionUsecase(nil, mockAuthRepo, nil, nil, nil, nil, nil, nil, nil)
		res, id, err := txUC.ProcessScanHybrid2(context.Background(), 1, 1, []byte("image"), "image/jpeg")

		assert.NotNil(t, err)
		assert.Nil(t, res)
		assert.Equal(t, uint(0), id)
		assert.Contains(t, err.Error(), "Weekly scan limit reached")
	})
}

func TestProcessScanAlternative_QuotaValidation(t *testing.T) {
	t.Run("Should return error if quota reached for alternative scan", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		// Simulasi limit
		user := &models.User{AccountTier: "free", OCRUsageCount: 5}
		user.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(user, nil)

		txUC := NewTransactionUsecase(nil, mockAuthRepo, nil, nil, nil, nil, nil, nil, nil)
		res, id, err := txUC.ProcessScanAlternative(context.Background(), "path/to/img.jpg", 1, 1)

		assert.NotNil(t, err)
		assert.Nil(t, res)
		assert.Equal(t, uint(0), id)
		assert.Contains(t, err.Error(), "Weekly scan limit reached")
	})
}

func TestExportTransactionsPDF(t *testing.T) {
	t.Run("Should return error when database fails to get transactions", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)
		mockTxRepo.On("GetAllByWorkspaceID", uint(1), "2026-05").Return(nil, errors.New("db error"))

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, nil, nil, nil, nil, nil)
		pdfBuf, err := txUC.ExportTransactionsPDF(context.Background(), 1, "2026-05")

		assert.NotNil(t, err)
		assert.Nil(t, pdfBuf)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("Should successfully generate PDF buffer", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)

		// Data dummy buat dicetak ke PDF
		txs := []models.Transaction{
			{Date: time.Now(), Merchant: "KFC", Type: "expense", Amount: 50000},
			{Date: time.Now(), Merchant: "Gaji Bulan Ini", Type: "income", Amount: 5000000},
		}
		mockTxRepo.On("GetAllByWorkspaceID", uint(1), "2026-05").Return(txs, nil)

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, nil, nil, nil, nil, nil)
		pdfBuf, err := txUC.ExportTransactionsPDF(context.Background(), 1, "2026-05")

		assert.Nil(t, err)
		assert.NotNil(t, pdfBuf)
		assert.True(t, pdfBuf.Len() > 0) // Mastiin buffer-nya beneran ada isinya (PDF kebentuk)
	})
}

func TestGetPendingEmailLogs(t *testing.T) {
	t.Run("Should successfully retrieve pending email logs", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)

		// FIX GORM.MODEL: Deklarasiin dulu, baru set ID-nya
		log1 := models.EmailParsed{Merchant: "Steam"}
		log1.ID = 1
		expectedLogs := []models.EmailParsed{log1}

		mockTxRepo.On("GetPendingEmailLogs", uint(1)).Return(expectedLogs, nil)

		txUC := NewTransactionUsecase(mockTxRepo, nil, nil, nil, nil, nil, nil, nil, nil)
		logs, err := txUC.GetPendingEmailLogs(1)

		assert.Nil(t, err)
		assert.Len(t, logs, 1)
		assert.Equal(t, "Steam", logs[0].Merchant)
	})
}

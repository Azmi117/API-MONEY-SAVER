package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ==========================================
// 1. MOCK KHUSUS UNTUK PENDING USECASE
// Nama sengaja dibikin panjang biar anti-redeclared
// ==========================================

type MockPendingRepoKhususPending struct{ mock.Mock }

func (m *MockPendingRepoKhususPending) Create(p *models.PendingTransaction) error {
	args := m.Called(p)
	return args.Error(0)
}
func (m *MockPendingRepoKhususPending) FindByID(id uint) (*models.PendingTransaction, error) {
	args := m.Called(id)
	if args.Get(0) != nil {
		return args.Get(0).(*models.PendingTransaction), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockPendingRepoKhususPending) UpdateStatus(id uint, status string) error {
	args := m.Called(id, status)
	return args.Error(0)
}
func (m *MockPendingRepoKhususPending) GetPendingList(wsID uint, page int, limit int) ([]models.PendingTransaction, int64, error) {
	args := m.Called(wsID, page, limit)
	if args.Get(0) != nil {
		return args.Get(0).([]models.PendingTransaction), args.Get(1).(int64), args.Error(2)
	}
	return nil, 0, args.Error(2)
}

// Mock Transaction Repo (Isi 22 Method Biar Gak Error Missing Method)
type MockTxRepoKhususPending struct{ mock.Mock }

func (m *MockTxRepoKhususPending) GetPendingEmailLogs(userID uint) ([]models.EmailParsed, error) {
	args := m.Called(userID)
	if args.Get(0) != nil {
		return args.Get(0).([]models.EmailParsed), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockTxRepoKhususPending) GetEmailLogByID(id uint) (*models.EmailParsed, error) {
	args := m.Called(id)
	if args.Get(0) != nil {
		return args.Get(0).(*models.EmailParsed), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockTxRepoKhususPending) Create(tx *models.Transaction) error {
	args := m.Called(tx)
	return args.Error(0)
}
func (m *MockTxRepoKhususPending) UpdateEmailLogStatus(id uint, status string) error {
	args := m.Called(id, status)
	return args.Error(0)
}
func (m *MockTxRepoKhususPending) GetEmailParsedByID(id uint) (*models.EmailParsed, error) {
	args := m.Called(id)
	if args.Get(0) != nil {
		return args.Get(0).(*models.EmailParsed), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockTxRepoKhususPending) DeleteEmailParsed(id uint) error {
	args := m.Called(id)
	return args.Error(0)
}

// Dummy methods
func (m *MockTxRepoKhususPending) IsDuplicate(w uint, a float64, me string, d time.Time) (bool, error) {
	return false, nil
}
func (m *MockTxRepoKhususPending) UpdateStatus(id uint, status string) error { return nil }
func (m *MockTxRepoKhususPending) GetByWorkspaceID(w uint, p int, l int) ([]models.Transaction, int64, error) {
	return nil, 0, nil
}
func (m *MockTxRepoKhususPending) Delete(id uint) error { return nil }
func (m *MockTxRepoKhususPending) GetByGmailID(g string) (*models.Transaction, error) {
	return nil, nil
}
func (m *MockTxRepoKhususPending) HardDelete(id uint) error                   { return nil }
func (m *MockTxRepoKhususPending) CreateEmailLog(e *models.EmailParsed) error { return nil }
func (m *MockTxRepoKhususPending) GetEmailLogByGmailID(g string) (*models.EmailParsed, error) {
	return nil, nil
}
func (m *MockTxRepoKhususPending) GetTotalAmountByType(w uint, t string, p string) (float64, error) {
	return 0, nil
}
func (m *MockTxRepoKhususPending) FindByID(tx *models.Transaction, id uint) error    { return nil }
func (m *MockTxRepoKhususPending) GetTotalByMonth(w uint, p string) (float64, error) { return 0, nil }
func (m *MockTxRepoKhususPending) GetTotalSavings(w uint, p string) (float64, error) { return 0, nil }
func (m *MockTxRepoKhususPending) GetSummaryByWorkspace(w uint, t string, mo string) ([]dto.UserTransactionSummary, error) {
	return nil, nil
}
func (m *MockTxRepoKhususPending) GetTotalByWorkspace(w uint, t string, mo string) (float64, error) {
	return 0, nil
}
func (m *MockTxRepoKhususPending) CreateWithItems(tx *models.Transaction) error { return nil }
func (m *MockTxRepoKhususPending) GetAllByWorkspaceID(w uint, mo string) ([]models.Transaction, error) {
	return nil, nil
}

// Mock Category Repo
type MockCategoryRepoKhususPending struct{ mock.Mock }

func (m *MockCategoryRepoKhususPending) FindByID(id uint) (*models.Category, error) {
	args := m.Called(id)
	if args.Get(0) != nil {
		return args.Get(0).(*models.Category), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockCategoryRepoKhususPending) Create(c *models.Category) error { return nil }
func (m *MockCategoryRepoKhususPending) GetByWorkspace(w uint) ([]models.Category, error) {
	return nil, nil
}

// Mock Target Usecase
type MockTargetUsecaseKhususPending struct{ mock.Mock }

func (m *MockTargetUsecaseKhususPending) AddIncomeToTarget(w uint, a float64) error {
	args := m.Called(w, a)
	return args.Error(0)
}
func (m *MockTargetUsecaseKhususPending) CheckWorkspaceTarget(w uint) (*dto.BudgetStatusResponse, error) {
	args := m.Called(w)
	if args.Get(0) != nil {
		return args.Get(0).(*dto.BudgetStatusResponse), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockTargetUsecaseKhususPending) SetTarget(r dto.SetTargetRequest) error { return nil }

// Mock Transaction Usecase
type MockTxUsecaseKhususPending struct{ mock.Mock }

func (m *MockTxUsecaseKhususPending) ConfirmScanTransaction(ctx context.Context, tx *models.Transaction, items []models.TransactionItem) (*dto.BudgetStatusResponse, error) {
	args := m.Called(ctx, tx, items)
	if args.Get(0) != nil {
		return args.Get(0).(*dto.BudgetStatusResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTxUsecaseKhususPending) GetPendingEmailLogs(userID uint) ([]models.EmailParsed, error) {
	args := m.Called(userID)
	if args.Get(0) != nil {
		return args.Get(0).([]models.EmailParsed), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTxUsecaseKhususPending) CreateManual(c context.Context, u uint, r dto.CreateTransactionRequest) (*models.Transaction, *dto.BudgetStatusResponse, error) {
	return nil, nil, nil
}
func (m *MockTxUsecaseKhususPending) ConfirmTransaction(c context.Context, p uint, r dto.ConfirmTransactionRequest) (*models.Transaction, *dto.BudgetStatusResponse, error) {
	return nil, nil, nil
}
func (m *MockTxUsecaseKhususPending) GetHistory(w uint, p int, l int) ([]models.Transaction, int64, error) {
	return nil, 0, nil
}
func (m *MockTxUsecaseKhususPending) DeleteTransaction(c context.Context, t uint, u uint) error {
	return nil
}
func (m *MockTxUsecaseKhususPending) ProcessScanHybrid2(c context.Context, u uint, w uint, i []byte, mi string) (*dto.ProcessScanHybridResult, uint, error) {
	return nil, 0, nil
}
func (m *MockTxUsecaseKhususPending) HardDeleteTransaction(id uint) error { return nil }
func (m *MockTxUsecaseKhususPending) ProcessScanAlternative(c context.Context, i string, u uint, w uint) (*dto.ProcessScanHybridResult, uint, error) {
	return nil, 0, nil
}
func (m *MockTxUsecaseKhususPending) ProcessTelegramInput(c context.Context, m_str string) (string, bool, float64) {
	return "", false, 0
}
func (m *MockTxUsecaseKhususPending) ExportTransactionsPDF(c context.Context, w uint, mo string) (*bytes.Buffer, error) {
	return nil, nil
}

// ==========================================
// 2. TEST SCENARIOS (100% COVERAGE)
// ==========================================

func TestCreatePendingSplit(t *testing.T) {
	ctx := context.Background()

	t.Run("Should success create pending split", func(t *testing.T) {
		mockPending := new(MockPendingRepoKhususPending)
		mockPending.On("Create", mock.AnythingOfType("*models.PendingTransaction")).Return(nil)

		uc := NewPendingUsecase(mockPending, nil, nil, nil, nil)
		id, err := uc.CreatePendingSplit(ctx, 1, 1, "test.jpg")

		assert.Nil(t, err)
		assert.Equal(t, uint(0), id)
	})

	// COVERAGE BARU: Error pas Create
	t.Run("Should return error if create fails", func(t *testing.T) {
		mockPending := new(MockPendingRepoKhususPending)
		mockPending.On("Create", mock.AnythingOfType("*models.PendingTransaction")).Return(errors.New("db error"))

		uc := NewPendingUsecase(mockPending, nil, nil, nil, nil)
		id, err := uc.CreatePendingSplit(ctx, 1, 1, "test.jpg")

		assert.NotNil(t, err)
		assert.Equal(t, uint(0), id)
	})
}

func TestConfirmPendingTransaction(t *testing.T) {
	ctx := context.Background()
	catID := uint(1)

	t.Run("Should success confirm expense", func(t *testing.T) {
		mockPending := new(MockPendingRepoKhususPending)
		mockCat := new(MockCategoryRepoKhususPending)
		mockTxUC := new(MockTxUsecaseKhususPending)

		txRaw := models.Transaction{WorkspaceID: 1, CategoryID: &catID, Type: "expense"}
		rawBytes, _ := json.Marshal(txRaw)

		pending := &models.PendingTransaction{RawData: string(rawBytes)}
		category := &models.Category{WorkspaceID: 1}
		budgetRes := &dto.BudgetStatusResponse{TotalExpense: 50000}

		mockPending.On("FindByID", uint(1)).Return(pending, nil)
		mockCat.On("FindByID", catID).Return(category, nil)
		mockTxUC.On("ConfirmScanTransaction", ctx, mock.AnythingOfType("*models.Transaction"), mock.Anything).Return(budgetRes, nil)
		mockPending.On("UpdateStatus", uint(1), "approved").Return(nil)

		uc := NewPendingUsecase(mockPending, nil, mockCat, nil, mockTxUC)
		res, err := uc.ConfirmPendingTransaction(ctx, 1)

		assert.Nil(t, err)
		assert.NotNil(t, res)
	})

	t.Run("Should success confirm income and update target", func(t *testing.T) {
		mockPending := new(MockPendingRepoKhususPending)
		mockTxUC := new(MockTxUsecaseKhususPending)
		mockTargetUC := new(MockTargetUsecaseKhususPending)

		txRaw := models.Transaction{WorkspaceID: 1, Type: "income", Amount: 100000}
		rawBytes, _ := json.Marshal(txRaw)
		pending := &models.PendingTransaction{RawData: string(rawBytes)}

		mockPending.On("FindByID", uint(2)).Return(pending, nil)
		mockTxUC.On("ConfirmScanTransaction", ctx, mock.Anything, mock.Anything).Return(&dto.BudgetStatusResponse{}, nil)
		mockPending.On("UpdateStatus", uint(2), "approved").Return(nil)
		mockTargetUC.On("AddIncomeToTarget", uint(1), float64(100000)).Return(nil)

		uc := NewPendingUsecase(mockPending, nil, nil, mockTargetUC, mockTxUC)
		_, err := uc.ConfirmPendingTransaction(ctx, 2)

		assert.Nil(t, err)
	})

	// ==========================================
	// COVERAGE BARU: Skenario Error ConfirmPending
	// ==========================================

	t.Run("Error - FindByID Fails", func(t *testing.T) {
		mockPending := new(MockPendingRepoKhususPending)
		mockPending.On("FindByID", uint(99)).Return(nil, errors.New("not found"))

		uc := NewPendingUsecase(mockPending, nil, nil, nil, nil)
		_, err := uc.ConfirmPendingTransaction(ctx, 99)
		assert.NotNil(t, err)
	})

	t.Run("Error - Invalid JSON", func(t *testing.T) {
		mockPending := new(MockPendingRepoKhususPending)
		pending := &models.PendingTransaction{RawData: "bukan json ini mah"}
		mockPending.On("FindByID", uint(1)).Return(pending, nil)

		uc := NewPendingUsecase(mockPending, nil, nil, nil, nil)
		_, err := uc.ConfirmPendingTransaction(ctx, 1)
		assert.NotNil(t, err)
	})

	t.Run("Error - Invalid Category", func(t *testing.T) {
		mockPending := new(MockPendingRepoKhususPending)
		mockCat := new(MockCategoryRepoKhususPending)

		txRaw := models.Transaction{WorkspaceID: 1, CategoryID: &catID}
		rawBytes, _ := json.Marshal(txRaw)
		pending := &models.PendingTransaction{RawData: string(rawBytes)}

		// Kategori ada, tapi punya workspace orang lain (WorkspaceID = 2)
		category := &models.Category{WorkspaceID: 2}

		mockPending.On("FindByID", uint(1)).Return(pending, nil)
		mockCat.On("FindByID", catID).Return(category, nil)

		uc := NewPendingUsecase(mockPending, nil, mockCat, nil, nil)
		_, err := uc.ConfirmPendingTransaction(ctx, 1)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid category")
	})

	t.Run("Error - ConfirmScanTransaction Fails", func(t *testing.T) {
		mockPending := new(MockPendingRepoKhususPending)
		mockTxUC := new(MockTxUsecaseKhususPending)

		txRaw := models.Transaction{WorkspaceID: 1}
		rawBytes, _ := json.Marshal(txRaw)
		pending := &models.PendingTransaction{RawData: string(rawBytes)}

		mockPending.On("FindByID", uint(1)).Return(pending, nil)
		mockTxUC.On("ConfirmScanTransaction", ctx, mock.Anything, mock.Anything).Return(nil, errors.New("tx error"))

		uc := NewPendingUsecase(mockPending, nil, nil, nil, mockTxUC)
		_, err := uc.ConfirmPendingTransaction(ctx, 1)
		assert.NotNil(t, err)
	})

	t.Run("Error - UpdateStatus Fails", func(t *testing.T) {
		mockPending := new(MockPendingRepoKhususPending)
		mockTxUC := new(MockTxUsecaseKhususPending)

		txRaw := models.Transaction{WorkspaceID: 1}
		rawBytes, _ := json.Marshal(txRaw)
		pending := &models.PendingTransaction{RawData: string(rawBytes)}

		mockPending.On("FindByID", uint(1)).Return(pending, nil)
		mockTxUC.On("ConfirmScanTransaction", ctx, mock.Anything, mock.Anything).Return(&dto.BudgetStatusResponse{}, nil)
		mockPending.On("UpdateStatus", uint(1), "approved").Return(errors.New("db error"))

		uc := NewPendingUsecase(mockPending, nil, nil, nil, mockTxUC)
		_, err := uc.ConfirmPendingTransaction(ctx, 1)
		assert.NotNil(t, err)
	})
}

func TestEmailLogFlow(t *testing.T) {
	ctx := context.Background()

	t.Run("GetPendingEmailLogs", func(t *testing.T) {
		mockTxRepo := new(MockTxRepoKhususPending)
		mockTxRepo.On("GetPendingEmailLogs", uint(1)).Return([]models.EmailParsed{{UserID: 1}}, nil)

		uc := NewPendingUsecase(nil, mockTxRepo, nil, nil, nil)
		res, err := uc.GetPendingEmailLogs(1)

		assert.Nil(t, err)
		assert.Len(t, res, 1)
	})

	t.Run("ApproveEmailLog - Success", func(t *testing.T) {
		mockTxRepo := new(MockTxRepoKhususPending)
		logData := &models.EmailParsed{UserID: 1, Status: "Pending"}

		mockTxRepo.On("GetEmailLogByID", uint(1)).Return(logData, nil)
		mockTxRepo.On("Create", mock.AnythingOfType("*models.Transaction")).Return(nil)
		mockTxRepo.On("UpdateEmailLogStatus", uint(1), "Success").Return(nil)

		uc := NewPendingUsecase(nil, mockTxRepo, nil, nil, nil)
		err := uc.ApproveEmailLog(ctx, 1, 1)

		assert.Nil(t, err)
	})

	// ==========================================
	// COVERAGE BARU: Error ApproveEmailLog
	// ==========================================
	t.Run("ApproveEmailLog - NotFound", func(t *testing.T) {
		mockTxRepo := new(MockTxRepoKhususPending)
		mockTxRepo.On("GetEmailLogByID", uint(1)).Return(nil, errors.New("not found"))

		uc := NewPendingUsecase(nil, mockTxRepo, nil, nil, nil)
		err := uc.ApproveEmailLog(ctx, 1, 1)
		assert.NotNil(t, err)
	})

	t.Run("ApproveEmailLog - Forbidden", func(t *testing.T) {
		mockTxRepo := new(MockTxRepoKhususPending)
		logData := &models.EmailParsed{UserID: 2} // User ID beda

		mockTxRepo.On("GetEmailLogByID", uint(1)).Return(logData, nil)

		uc := NewPendingUsecase(nil, mockTxRepo, nil, nil, nil)
		err := uc.ApproveEmailLog(ctx, 1, 1) // Nyoba akses pake UserID 1
		assert.NotNil(t, err)
	})

	t.Run("ApproveEmailLog - BadRequest", func(t *testing.T) {
		mockTxRepo := new(MockTxRepoKhususPending)
		logData := &models.EmailParsed{UserID: 1, Status: "Success"} // Udah di-proses

		mockTxRepo.On("GetEmailLogByID", uint(1)).Return(logData, nil)

		uc := NewPendingUsecase(nil, mockTxRepo, nil, nil, nil)
		err := uc.ApproveEmailLog(ctx, 1, 1)
		assert.NotNil(t, err)
	})

	t.Run("ApproveEmailLog - Create Fails", func(t *testing.T) {
		mockTxRepo := new(MockTxRepoKhususPending)
		logData := &models.EmailParsed{UserID: 1, Status: "Pending"}

		mockTxRepo.On("GetEmailLogByID", uint(1)).Return(logData, nil)
		mockTxRepo.On("Create", mock.Anything).Return(errors.New("db error"))

		uc := NewPendingUsecase(nil, mockTxRepo, nil, nil, nil)
		err := uc.ApproveEmailLog(ctx, 1, 1)
		assert.NotNil(t, err)
	})

	// ==========================================
	// COVERAGE BARU: Error RejectEmailLog
	// ==========================================
	t.Run("RejectEmailLog - Success", func(t *testing.T) {
		mockTxRepo := new(MockTxRepoKhususPending)
		logData := &models.EmailParsed{UserID: 1}

		mockTxRepo.On("GetEmailLogByID", uint(1)).Return(logData, nil)
		mockTxRepo.On("UpdateEmailLogStatus", uint(1), "Rejected").Return(nil)

		uc := NewPendingUsecase(nil, mockTxRepo, nil, nil, nil)
		err := uc.RejectEmailLog(ctx, 1, 1)

		assert.Nil(t, err)
	})

	t.Run("RejectEmailLog - NotFound", func(t *testing.T) {
		mockTxRepo := new(MockTxRepoKhususPending)
		mockTxRepo.On("GetEmailLogByID", uint(1)).Return(nil, errors.New("not found"))

		uc := NewPendingUsecase(nil, mockTxRepo, nil, nil, nil)
		err := uc.RejectEmailLog(ctx, 1, 1)
		assert.NotNil(t, err)
	})

	t.Run("RejectEmailLog - Forbidden", func(t *testing.T) {
		mockTxRepo := new(MockTxRepoKhususPending)
		logData := &models.EmailParsed{UserID: 2}

		mockTxRepo.On("GetEmailLogByID", uint(1)).Return(logData, nil)

		uc := NewPendingUsecase(nil, mockTxRepo, nil, nil, nil)
		err := uc.RejectEmailLog(ctx, 1, 1) // User 1 nyoba reject punya User 2
		assert.NotNil(t, err)
	})

	// ==========================================
	// COVERAGE BARU: ConfirmEmailTransaction
	// ==========================================
	t.Run("ConfirmEmailTransaction - Success", func(t *testing.T) {
		mockTxRepo := new(MockTxRepoKhususPending)
		mockTargetUC := new(MockTargetUsecaseKhususPending)

		emailData := &models.EmailParsed{Amount: 50000}
		budgetRes := &dto.BudgetStatusResponse{LimitAmount: 100000}

		mockTxRepo.On("GetEmailParsedByID", uint(1)).Return(emailData, nil)
		mockTxRepo.On("Create", mock.Anything).Return(nil)
		mockTxRepo.On("DeleteEmailParsed", uint(1)).Return(nil)
		mockTargetUC.On("CheckWorkspaceTarget", uint(1)).Return(budgetRes, nil)

		uc := NewPendingUsecase(nil, mockTxRepo, nil, mockTargetUC, nil)
		req := dto.ConfirmEmailRequest{EmailParsedID: 1, WorkspaceID: 1}
		res, err := uc.ConfirmEmailTransaction(ctx, 1, req)

		assert.Nil(t, err)
		assert.Equal(t, float64(100000), res.LimitAmount)
	})

	t.Run("ConfirmEmailTransaction - GetParsed Fails", func(t *testing.T) {
		mockTxRepo := new(MockTxRepoKhususPending)
		mockTxRepo.On("GetEmailParsedByID", uint(1)).Return(nil, errors.New("not found"))

		uc := NewPendingUsecase(nil, mockTxRepo, nil, nil, nil)
		req := dto.ConfirmEmailRequest{EmailParsedID: 1, WorkspaceID: 1}
		_, err := uc.ConfirmEmailTransaction(ctx, 1, req)
		assert.NotNil(t, err)
	})

	t.Run("ConfirmEmailTransaction - Create Fails", func(t *testing.T) {
		mockTxRepo := new(MockTxRepoKhususPending)
		emailData := &models.EmailParsed{Amount: 50000}

		mockTxRepo.On("GetEmailParsedByID", uint(1)).Return(emailData, nil)
		mockTxRepo.On("Create", mock.Anything).Return(errors.New("db error"))

		uc := NewPendingUsecase(nil, mockTxRepo, nil, nil, nil)
		req := dto.ConfirmEmailRequest{EmailParsedID: 1, WorkspaceID: 1}
		_, err := uc.ConfirmEmailTransaction(ctx, 1, req)
		assert.NotNil(t, err)
	})

	t.Run("ConfirmEmailTransaction - CheckWorkspaceTarget Fails (But still returns nil)", func(t *testing.T) {
		mockTxRepo := new(MockTxRepoKhususPending)
		mockTargetUC := new(MockTargetUsecaseKhususPending)
		emailData := &models.EmailParsed{Amount: 50000}

		mockTxRepo.On("GetEmailParsedByID", uint(1)).Return(emailData, nil)
		mockTxRepo.On("Create", mock.Anything).Return(nil)
		mockTxRepo.On("DeleteEmailParsed", uint(1)).Return(nil)
		// Kita simulasikan target check error, di kode asli lu dia nge-return nil, nil
		mockTargetUC.On("CheckWorkspaceTarget", uint(1)).Return(nil, errors.New("target error"))

		uc := NewPendingUsecase(nil, mockTxRepo, nil, mockTargetUC, nil)
		req := dto.ConfirmEmailRequest{EmailParsedID: 1, WorkspaceID: 1}
		res, err := uc.ConfirmEmailTransaction(ctx, 1, req)

		assert.Nil(t, err) // Karena err di-ignore di kode asli
		assert.Nil(t, res) // Karena budgetData-nya gagal diambil
	})
}

func TestGetPendingTransactions(t *testing.T) {
	t.Run("Should success get list", func(t *testing.T) {
		mockPending := new(MockPendingRepoKhususPending)
		list := []models.PendingTransaction{{ID: 1}}

		mockPending.On("GetPendingList", uint(1), 1, 10).Return(list, int64(1), nil)

		uc := NewPendingUsecase(mockPending, nil, nil, nil, nil)
		res, total, err := uc.GetPendingTransactions(1, 1, 10)

		assert.Nil(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, res, 1)
	})
}

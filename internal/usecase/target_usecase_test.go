package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ==========================================
// 1. MOCK TARGET REPOSITORY KHUSUS FILE INI
// ==========================================
type MockTargetRepoKhusus struct {
	mock.Mock
}

func (m *MockTargetRepoKhusus) GetActiveTarget(workspaceID uint, period string) (*models.Target, error) {
	return nil, nil
}
func (m *MockTargetRepoKhusus) GetActiveTargets(workspaceID uint, period string) ([]models.Target, error) {
	return nil, nil
}
func (m *MockTargetRepoKhusus) UpsertTarget(target *models.Target) error {
	args := m.Called(target)
	return args.Error(0)
}
func (m *MockTargetRepoKhusus) GetByWorkspaceAndPeriod(wsID uint, period string) (*models.Target, error) {
	args := m.Called(wsID, period)
	if args.Get(0) != nil {
		return args.Get(0).(*models.Target), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockTargetRepoKhusus) Update(target *models.Target) error {
	args := m.Called(target)
	return args.Error(0)
}

// ==========================================
// 2. MOCK TRANSACTION REPOSITORY KHUSUS FILE INI
// Sesuai persis dengan transaction_repo.go yang lu kasih
// ==========================================
type MockTxRepoKhusus struct {
	mock.Mock
}

// Method Utama yang dipakai di Target Usecase
func (m *MockTxRepoKhusus) GetTotalByWorkspace(workspaceID uint, txType string, month string) (float64, error) {
	args := m.Called(workspaceID, txType, month)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockTxRepoKhusus) GetSummaryByWorkspace(workspaceID uint, txType string, month string) ([]dto.UserTransactionSummary, error) {
	args := m.Called(workspaceID, txType, month)
	if args.Get(0) != nil {
		return args.Get(0).([]dto.UserTransactionSummary), args.Error(1)
	}
	return nil, args.Error(1)
}

// Dummy Methods supaya TransactionRepository Interface terpenuhi (20 method lainnya)
func (m *MockTxRepoKhusus) Create(transaction *models.Transaction) error { return nil }
func (m *MockTxRepoKhusus) IsDuplicate(workspaceID uint, amount float64, merchant string, date time.Time) (bool, error) {
	return false, nil
}
func (m *MockTxRepoKhusus) UpdateStatus(id uint, status string) error { return nil }
func (m *MockTxRepoKhusus) GetByWorkspaceID(workspaceID uint, page int, limit int) ([]models.Transaction, int64, error) {
	return nil, 0, nil
}
func (m *MockTxRepoKhusus) Delete(id uint) error                                     { return nil }
func (m *MockTxRepoKhusus) GetByGmailID(gmailID string) (*models.Transaction, error) { return nil, nil }
func (m *MockTxRepoKhusus) HardDelete(id uint) error                                 { return nil }
func (m *MockTxRepoKhusus) CreateEmailLog(emailLog *models.EmailParsed) error        { return nil }
func (m *MockTxRepoKhusus) GetEmailLogByGmailID(gmailID string) (*models.EmailParsed, error) {
	return nil, nil
}
func (m *MockTxRepoKhusus) UpdateEmailLogStatus(id uint, status string) error { return nil }
func (m *MockTxRepoKhusus) GetPendingEmailLogs(userID uint) ([]models.EmailParsed, error) {
	return nil, nil
}
func (m *MockTxRepoKhusus) GetEmailLogByID(id uint) (*models.EmailParsed, error) { return nil, nil }
func (m *MockTxRepoKhusus) GetTotalAmountByType(workspaceID uint, txType string, period string) (float64, error) {
	return 0, nil
}
func (m *MockTxRepoKhusus) FindByID(tx *models.Transaction, id uint) error { return nil }
func (m *MockTxRepoKhusus) GetTotalByMonth(workspaceID uint, period string) (float64, error) {
	return 0, nil
}
func (m *MockTxRepoKhusus) GetTotalSavings(workspaceID uint, period string) (float64, error) {
	return 0, nil
}
func (m *MockTxRepoKhusus) GetEmailParsedByID(id uint) (*models.EmailParsed, error) { return nil, nil }
func (m *MockTxRepoKhusus) DeleteEmailParsed(id uint) error                         { return nil }
func (m *MockTxRepoKhusus) CreateWithItems(transaction *models.Transaction) error   { return nil }
func (m *MockTxRepoKhusus) GetAllByWorkspaceID(workspaceID uint, month string) ([]models.Transaction, error) {
	return nil, nil
}

// ==========================================
// 3. TEST SCENARIOS
// ==========================================

func TestAddIncomeToTarget(t *testing.T) {
	month := time.Now().Format("2006-01")

	t.Run("Should return nil if target not found", func(t *testing.T) {
		mockTargetRepo := new(MockTargetRepoKhusus)

		mockTargetRepo.On("GetByWorkspaceAndPeriod", uint(1), month).Return(nil, errors.New("not found"))

		// Kita gak butuh TxRepo di fungsi ini, jadi lempar mock kosong aja aman
		targetUC := NewTargetUsecase(mockTargetRepo, new(MockTxRepoKhusus))
		err := targetUC.AddIncomeToTarget(1, 50000)

		assert.Nil(t, err)
	})

	t.Run("Should return error if update fails", func(t *testing.T) {
		mockTargetRepo := new(MockTargetRepoKhusus)
		target := &models.Target{CurrentAmount: 10000}

		mockTargetRepo.On("GetByWorkspaceAndPeriod", uint(1), month).Return(target, nil)
		mockTargetRepo.On("Update", mock.AnythingOfType("*models.Target")).Return(errors.New("db error"))

		targetUC := NewTargetUsecase(mockTargetRepo, new(MockTxRepoKhusus))
		err := targetUC.AddIncomeToTarget(1, 50000)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("Should successfully add income and update target", func(t *testing.T) {
		mockTargetRepo := new(MockTargetRepoKhusus)
		target := &models.Target{CurrentAmount: 10000}

		mockTargetRepo.On("GetByWorkspaceAndPeriod", uint(1), month).Return(target, nil)
		mockTargetRepo.On("Update", mock.AnythingOfType("*models.Target")).Return(nil)

		targetUC := NewTargetUsecase(mockTargetRepo, new(MockTxRepoKhusus))
		err := targetUC.AddIncomeToTarget(1, 50000)

		assert.Nil(t, err)
		assert.Equal(t, float64(60000), target.CurrentAmount)
	})
}

func TestCheckWorkspaceTarget(t *testing.T) {
	currentPeriod := time.Now().Format("2006-01")

	t.Run("Should return error if target not set", func(t *testing.T) {
		mockTargetRepo := new(MockTargetRepoKhusus)
		mockTxRepo := new(MockTxRepoKhusus)

		mockTargetRepo.On("GetByWorkspaceAndPeriod", uint(1), currentPeriod).Return(nil, errors.New("not found"))

		targetUC := NewTargetUsecase(mockTargetRepo, mockTxRepo)
		res, err := targetUC.CheckWorkspaceTarget(1)

		assert.NotNil(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "target not set")
	})

	t.Run("Should return budget status successfully", func(t *testing.T) {
		mockTargetRepo := new(MockTargetRepoKhusus)
		mockTxRepo := new(MockTxRepoKhusus)

		target := &models.Target{AmountLimit: 100000}

		// 1. Setup Mock Target
		mockTargetRepo.On("GetByWorkspaceAndPeriod", uint(1), currentPeriod).Return(target, nil)

		// 2. Setup Mock Transaksi (ini yang tadinya 0 sekarang dapet data beneran)
		mockTxRepo.On("GetTotalByWorkspace", uint(1), "expense", currentPeriod).Return(float64(40000), nil)

		summaries := []dto.UserTransactionSummary{
			{UserName: "Azmi", Total: 40000},
		}
		mockTxRepo.On("GetSummaryByWorkspace", uint(1), "expense", currentPeriod).Return(summaries, nil)

		targetUC := NewTargetUsecase(mockTargetRepo, mockTxRepo)

		// EKSEKUSI
		res, err := targetUC.CheckWorkspaceTarget(1)

		assert.Nil(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, float64(40000), res.TotalExpense)
		assert.Equal(t, float64(60000), res.RemainingBudget)
		assert.Len(t, res.ExpenseDetails, 1)
		assert.Equal(t, "Azmi", res.ExpenseDetails[0].UserName)
	})
}

func TestSetTarget(t *testing.T) {
	t.Run("Should return error if upsert fails", func(t *testing.T) {
		mockTargetRepo := new(MockTargetRepoKhusus)
		mockTargetRepo.On("UpsertTarget", mock.AnythingOfType("*models.Target")).Return(errors.New("db error"))

		targetUC := NewTargetUsecase(mockTargetRepo, new(MockTxRepoKhusus))
		err := targetUC.SetTarget(dto.SetTargetRequest{WorkspaceID: 1})

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("Should successfully set target", func(t *testing.T) {
		mockTargetRepo := new(MockTargetRepoKhusus)
		mockTargetRepo.On("UpsertTarget", mock.AnythingOfType("*models.Target")).Return(nil)

		targetUC := NewTargetUsecase(mockTargetRepo, new(MockTxRepoKhusus))
		err := targetUC.SetTarget(dto.SetTargetRequest{
			WorkspaceID:   1,
			Period:        "2026-06",
			AmountLimit:   100000,
			SavingsTarget: 50000,
		})

		assert.Nil(t, err)
	})
}

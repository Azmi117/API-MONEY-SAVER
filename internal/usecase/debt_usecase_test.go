package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ==========================================
// MOCK DEBT REPOSITORY
// ==========================================
type MockDebtRepository struct {
	mock.Mock
}

func (m *MockDebtRepository) CreateInBatch(debts []models.Debt) error {
	args := m.Called(debts)
	return args.Error(0)
}
func (m *MockDebtRepository) GetDebtsByWorkspace(workspaceID uint) ([]models.Debt, error) {
	args := m.Called(workspaceID)
	if args.Get(0) != nil {
		return args.Get(0).([]models.Debt), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockDebtRepository) IsShortCodeExists(workspaceID uint, code string) (bool, error) {
	args := m.Called(workspaceID, code)
	return args.Bool(0), args.Error(1)
}
func (m *MockDebtRepository) GetDebtByShortCode(workspaceID uint, code string) (*models.Debt, error) {
	args := m.Called(workspaceID, code)
	if args.Get(0) != nil {
		return args.Get(0).(*models.Debt), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockDebtRepository) UpdateIsPaid(debtID uint, status bool) error {
	args := m.Called(debtID, status)
	return args.Error(0)
}
func (m *MockDebtRepository) FindByID(id uint) (*models.Debt, error) {
	args := m.Called(id)
	if args.Get(0) != nil {
		return args.Get(0).(*models.Debt), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockDebtRepository) MarkAsPaid(id uint) error {
	args := m.Called(id)
	return args.Error(0)
}

// ==========================================
// TEST SCENARIOS
// ==========================================

func TestGenerateUniqueShortCode(t *testing.T) {
	mockDebtRepo := new(MockDebtRepository)
	// Simulasi kode yang digenerate selalu unik (belum ada di DB)
	mockDebtRepo.On("IsShortCodeExists", uint(1), mock.AnythingOfType("string")).Return(false, nil)

	debtUC := NewDebtUsecase(mockDebtRepo, nil)
	code, err := debtUC.GenerateUniqueShortCode(1)

	assert.Nil(t, err)
	assert.Len(t, code, 4) // Harus selalu 4 karakter
}

func TestGetWorkspaceDebts(t *testing.T) {
	t.Run("Should return error when repo fails", func(t *testing.T) {
		mockDebtRepo := new(MockDebtRepository)
		mockDebtRepo.On("GetDebtsByWorkspace", uint(1)).Return(nil, errors.New("db error"))

		debtUC := NewDebtUsecase(mockDebtRepo, nil)
		res, err := debtUC.GetWorkspaceDebts(context.Background(), 1)

		assert.NotNil(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "failed to retrieve workspace debts")
	})

	t.Run("Should return debts successfully", func(t *testing.T) {
		mockDebtRepo := new(MockDebtRepository)
		expectedDebts := []models.Debt{{Amount: 50000}}
		mockDebtRepo.On("GetDebtsByWorkspace", uint(1)).Return(expectedDebts, nil)

		debtUC := NewDebtUsecase(mockDebtRepo, nil)
		res, err := debtUC.GetWorkspaceDebts(context.Background(), 1)

		assert.Nil(t, err)
		assert.Len(t, res, 1)
	})
}

func TestConfirmPayment(t *testing.T) {
	t.Run("Should return error if debt not found", func(t *testing.T) {
		mockDebtRepo := new(MockDebtRepository)
		mockDebtRepo.On("GetDebtByShortCode", uint(1), "ABCD").Return(nil, errors.New("not found"))

		debtUC := NewDebtUsecase(mockDebtRepo, nil)
		err := debtUC.ConfirmPayment(context.Background(), 1, "ABCD", 12345)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid or already paid")
	})

	t.Run("Should return error if telegram ID is nil or unauthorized", func(t *testing.T) {
		mockDebtRepo := new(MockDebtRepository)

		// Skenario 1: Telegram ID dari FromUser nil
		debtNilID := &models.Debt{FromUser: models.User{Name: "Budi"}}
		mockDebtRepo.On("GetDebtByShortCode", uint(1), "NIL0").Return(debtNilID, nil)

		// Skenario 2: Telegram ID beda sama yang ngetik command
		wrongID := 99999
		debtWrongID := &models.Debt{FromUser: models.User{Name: "Andi", TelegramID: &wrongID}}
		mockDebtRepo.On("GetDebtByShortCode", uint(1), "WRNG").Return(debtWrongID, nil)

		debtUC := NewDebtUsecase(mockDebtRepo, nil)

		err1 := debtUC.ConfirmPayment(context.Background(), 1, "NIL0", 12345)
		assert.NotNil(t, err1)
		assert.Contains(t, err1.Error(), "unauthorized")

		err2 := debtUC.ConfirmPayment(context.Background(), 1, "WRNG", 12345)
		assert.NotNil(t, err2)
		assert.Contains(t, err2.Error(), "unauthorized")
	})

	t.Run("Should successfully confirm payment", func(t *testing.T) {
		mockDebtRepo := new(MockDebtRepository)
		validID := 12345

		// FIX GORM.MODEL: Deklarasiin dulu, baru tembak ID-nya di bawah
		debt := &models.Debt{
			FromUser: models.User{Name: "Azmi", TelegramID: &validID},
		}
		debt.ID = 10 // <--- Tembak ID di sini

		mockDebtRepo.On("GetDebtByShortCode", uint(1), "OKEY").Return(debt, nil)
		mockDebtRepo.On("UpdateIsPaid", uint(10), true).Return(nil)

		debtUC := NewDebtUsecase(mockDebtRepo, nil)
		err := debtUC.ConfirmPayment(context.Background(), 1, "OKEY", 12345)

		assert.Nil(t, err)
	})
}

func TestMarkAsPaid(t *testing.T) {
	t.Run("Should return not found error", func(t *testing.T) {
		mockDebtRepo := new(MockDebtRepository)
		mockDebtRepo.On("FindByID", uint(1)).Return(nil, errors.New("not found"))

		debtUC := NewDebtUsecase(mockDebtRepo, nil)
		err := debtUC.MarkAsPaid(context.Background(), 1, 1)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Debt not found")
	})

	t.Run("Should return forbidden if not debtor", func(t *testing.T) {
		mockDebtRepo := new(MockDebtRepository)
		debt := &models.Debt{FromUserID: 2} // Yang ngutang user 2
		mockDebtRepo.On("FindByID", uint(1)).Return(debt, nil)

		debtUC := NewDebtUsecase(mockDebtRepo, nil)
		err := debtUC.MarkAsPaid(context.Background(), 1, 1) // Yang mau bayar user 1

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Only the debtor can pay")
	})

	t.Run("Should return bad request if already paid", func(t *testing.T) {
		mockDebtRepo := new(MockDebtRepository)
		debt := &models.Debt{FromUserID: 1, IsPaid: true}
		mockDebtRepo.On("FindByID", uint(1)).Return(debt, nil)

		debtUC := NewDebtUsecase(mockDebtRepo, nil)
		err := debtUC.MarkAsPaid(context.Background(), 1, 1)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "already paid")
	})

	t.Run("Should return error if update fails", func(t *testing.T) {
		mockDebtRepo := new(MockDebtRepository)
		debt := &models.Debt{FromUserID: 1, IsPaid: false}
		mockDebtRepo.On("FindByID", uint(1)).Return(debt, nil)
		mockDebtRepo.On("MarkAsPaid", uint(1)).Return(errors.New("db error"))

		debtUC := NewDebtUsecase(mockDebtRepo, nil)
		err := debtUC.MarkAsPaid(context.Background(), 1, 1)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "failed to update debt status")
	})

	t.Run("Should successfully mark as paid", func(t *testing.T) {
		mockDebtRepo := new(MockDebtRepository)
		debt := &models.Debt{FromUserID: 1, IsPaid: false}
		mockDebtRepo.On("FindByID", uint(1)).Return(debt, nil)
		mockDebtRepo.On("MarkAsPaid", uint(1)).Return(nil)

		debtUC := NewDebtUsecase(mockDebtRepo, nil)
		err := debtUC.MarkAsPaid(context.Background(), 1, 1)

		assert.Nil(t, err)
	})
}

func TestAssignSplitBill(t *testing.T) {
	t.Run("Should return error if transaction not found", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository) // Pake mock dari transaction_usecase_test.go
		mockTxRepo.On("FindByID", mock.AnythingOfType("*models.Transaction"), uint(1)).Return(errors.New("not found"))

		debtUC := NewDebtUsecase(nil, mockTxRepo)
		err := debtUC.AssignSplitBill(context.Background(), 1, nil)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Should return error if item not found in original receipt", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)
		mockTxRepo.On("FindByID", mock.AnythingOfType("*models.Transaction"), uint(1)).Return(nil).Run(func(args mock.Arguments) {
			tx := args.Get(0).(*models.Transaction)
			tx.TransactionItems = []models.TransactionItem{{Description: "Ayam", Quantity: 2}}
		})

		items := []dto.SplitItemRequest{{ItemName: "Bebek", Quantity: 1}}

		debtUC := NewDebtUsecase(nil, mockTxRepo)
		err := debtUC.AssignSplitBill(context.Background(), 1, items)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "not found in original receipt")
	})

	t.Run("Should return error if item quantity exceeded", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)
		mockTxRepo.On("FindByID", mock.AnythingOfType("*models.Transaction"), uint(1)).Return(nil).Run(func(args mock.Arguments) {
			tx := args.Get(0).(*models.Transaction)
			tx.TransactionItems = []models.TransactionItem{{Description: "Ayam", Quantity: 1}}
		})

		items := []dto.SplitItemRequest{{ItemName: "Ayam", Quantity: 2}} // Melebihi stok struk

		debtUC := NewDebtUsecase(nil, mockTxRepo)
		err := debtUC.AssignSplitBill(context.Background(), 1, items)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "quantity exceeded")
	})

	t.Run("Should return error if total split exceeds receipt amount", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)
		mockTxRepo.On("FindByID", mock.AnythingOfType("*models.Transaction"), uint(1)).Return(nil).Run(func(args mock.Arguments) {
			tx := args.Get(0).(*models.Transaction)
			tx.Amount = 10000 // Total struk cuma 10rb
			tx.TransactionItems = []models.TransactionItem{{Description: "Ayam", Quantity: 2}}
		})

		// Split bill masukin harga 20rb
		items := []dto.SplitItemRequest{{ItemName: "Ayam", Quantity: 1, Price: 20000}}

		debtUC := NewDebtUsecase(nil, mockTxRepo)
		err := debtUC.AssignSplitBill(context.Background(), 1, items)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "exceeds original receipt amount")
	})

	t.Run("Should successfully create debts for split bill", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)
		mockDebtRepo := new(MockDebtRepository)

		mockTxRepo.On("FindByID", mock.AnythingOfType("*models.Transaction"), uint(1)).Return(nil).Run(func(args mock.Arguments) {
			tx := args.Get(0).(*models.Transaction)
			tx.WorkspaceID = 1
			tx.UserID = 1 // Pemilik struk
			tx.Amount = 50000
			tx.TransactionItems = []models.TransactionItem{{Description: "Ayam", Quantity: 2}}
		})

		// Item di-tag ke user 2 (yang berarti dia ngutang ke user 1)
		items := []dto.SplitItemRequest{{ItemName: "Ayam", UserID: 2, Quantity: 1, Price: 25000}}

		// Ini kepanggil pas AssignSplitBill manggil GenerateUniqueShortCode
		mockDebtRepo.On("IsShortCodeExists", uint(1), mock.AnythingOfType("string")).Return(false, nil)
		mockDebtRepo.On("CreateInBatch", mock.AnythingOfType("[]models.Debt")).Return(nil)

		debtUC := NewDebtUsecase(mockDebtRepo, mockTxRepo)
		err := debtUC.AssignSplitBill(context.Background(), 1, items)

		assert.Nil(t, err)
		mockDebtRepo.AssertCalled(t, "CreateInBatch", mock.AnythingOfType("[]models.Debt"))
	})
	t.Run("Should fallback to ERR1 if generate shortcode fails", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)
		mockDebtRepo := new(MockDebtRepository)

		mockTxRepo.On("FindByID", mock.AnythingOfType("*models.Transaction"), uint(1)).Return(nil).Run(func(args mock.Arguments) {
			tx := args.Get(0).(*models.Transaction)
			tx.WorkspaceID = 1
			tx.UserID = 1 // Pemilik struk
			tx.Amount = 50000
			tx.TransactionItems = []models.TransactionItem{{Description: "Ayam", Quantity: 2}}
		})

		items := []dto.SplitItemRequest{{ItemName: "Ayam", UserID: 2, Quantity: 1, Price: 25000}}

		// Simulasi pas nyari shortcode, database meledak
		mockDebtRepo.On("IsShortCodeExists", uint(1), mock.AnythingOfType("string")).Return(false, errors.New("db meledak"))

		// Karena database meledak, dia bakal pake "ERR1" dan lanjut nyimpen
		mockDebtRepo.On("CreateInBatch", mock.AnythingOfType("[]models.Debt")).Return(nil)

		debtUC := NewDebtUsecase(mockDebtRepo, mockTxRepo)
		err := debtUC.AssignSplitBill(context.Background(), 1, items)

		assert.Nil(t, err)
		mockDebtRepo.AssertCalled(t, "CreateInBatch", mock.AnythingOfType("[]models.Debt"))
	})

	t.Run("Should return nil when splitting to oneself (no debt created)", func(t *testing.T) {
		mockTxRepo := new(MockTransactionRepository)

		mockTxRepo.On("FindByID", mock.AnythingOfType("*models.Transaction"), uint(1)).Return(nil).Run(func(args mock.Arguments) {
			tx := args.Get(0).(*models.Transaction)
			tx.WorkspaceID = 1
			tx.UserID = 1 // Pemilik struk = User 1
			tx.Amount = 50000
			tx.TransactionItems = []models.TransactionItem{{Description: "Ayam", Quantity: 2}}
		})

		// Skenario: User 1 (pemilik struk) nge-tag makanan buat dirinya sendiri
		items := []dto.SplitItemRequest{{ItemName: "Ayam", UserID: 1, Quantity: 1, Price: 25000}}

		debtUC := NewDebtUsecase(nil, mockTxRepo)

		// Bakal nge-hit 'return nil' yang paling bawah karena len(userDebts) == 0
		err := debtUC.AssignSplitBill(context.Background(), 1, items)

		assert.Nil(t, err)
	})
}

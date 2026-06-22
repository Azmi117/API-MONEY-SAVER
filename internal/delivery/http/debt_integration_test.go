package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/testutils"
	"github.com/stretchr/testify/assert"
)

func TestIntegration_DebtAPI(t *testing.T) {
	db := testutils.SetupTestDB()
	defer testutils.CleanTestDB(db)

	txRepo := repository.NewTransactionRepository(db)
	authRepo := repository.NewAuthRepository(db)
	wsRepo := repository.NewWorkspaceRepository(db)
	debtRepo := repository.NewDebtRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	targetRepo := repository.NewTargetRepository(db)
	pendingRepo := repository.NewPendingTransactionRepository(db)

	targetUsecase := usecase.NewTargetUsecase(targetRepo, txRepo)
	txUsecase := usecase.NewTransactionUsecase(txRepo, authRepo, nil, nil, wsRepo, nil, pendingRepo, categoryRepo, targetUsecase)
	debtUsecase := usecase.NewDebtUsecase(debtRepo, txRepo)

	_ = txUsecase
	debtHandler := NewDebtHandler(debtUsecase)

	// ==========================================
	// 1. HAPPY PATHS (Yang udah tercover)
	// ==========================================
	t.Run("Happy Paths: Get, Pay, and Split", func(t *testing.T) {
		testutils.CleanTestDB(db)
		user := models.User{Name: "User", Email: "debt@test.com"}
		db.Create(&user)

		// WAJIB: Bikin akun temen buat ditagih utang
		teman := models.User{Name: "Teman", Email: "teman@test.com"}
		db.Create(&teman)

		ws := seedWorkspaceWithTarget(db, user, "Debt WS")

		debt := models.Debt{
			WorkspaceID: ws.ID,
			FromUserID:  user.ID,
			ToUserID:    user.ID,
			Amount:      50000,
		}
		db.Create(&debt)

		// WAJIB: Masukin TransactionItems (Misal: Nasi Goreng) ke struk transaksi
		trx := models.Transaction{
			WorkspaceID: ws.ID,
			UserID:      user.ID,
			Amount:      50000,
			Type:        "expense",
			Note:        "Makan Siang",
			Date:        time.Now(),
			TransactionItems: []models.TransactionItem{
				// FIX: Ganti Amount jadi Total
				{Description: "Nasi Goreng", Quantity: 2, Price: 25000, Total: 50000},
			},
		}
		db.Create(&trx)

		// Get
		reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/debts/workspace/%d", ws.ID), nil)
		reqGet.SetPathValue("id", fmt.Sprint(ws.ID))
		wGet := httptest.NewRecorder()
		debtHandler.GetWorkspaceDebts(wGet, reqGet)
		assert.Equal(t, http.StatusOK, wGet.Code)

		// Pay
		reqPay := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/debts/%d/pay", debt.ID), nil)
		reqPay.SetPathValue("id", fmt.Sprint(debt.ID))
		ctx := context.WithValue(context.Background(), "user_id", user.ID)
		wPay := httptest.NewRecorder()
		debtHandler.PayDebt(wPay, reqPay.WithContext(ctx))
		assert.Equal(t, http.StatusOK, wPay.Code)

		// AssignSplitBill
		// WAJIB: JSON Payload diubah, panggil "item_name", "quantity", dan "price" sesuai DTO lu.
		// Tagih ke ID temen, bukan ID user sendiri biar validasi userDebts tembus.
		payload := fmt.Sprintf(`{"transaction_id": %d, "items": [{"item_name": "Nasi Goreng", "user_id": %d, "quantity": 1, "price": 25000}]}`, trx.ID, teman.ID)
		reqSplit := httptest.NewRequest(http.MethodPost, "/api/v1/debts/split", strings.NewReader(payload))
		wSplit := httptest.NewRecorder()
		debtHandler.AssignSplitBill(wSplit, reqSplit.WithContext(ctx))
		assert.Equal(t, http.StatusOK, wSplit.Code)
	})

	// ==========================================
	// 2. ERROR & EDGE CASES (Nembak Baris Not Covered)
	// ==========================================
	t.Run("Coverage Boost: Error Bombardment", func(t *testing.T) {
		testutils.CleanTestDB(db)

		// Trigger: Wrong Method in PayDebt
		wMethod := httptest.NewRecorder()
		debtHandler.PayDebt(wMethod, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusMethodNotAllowed, wMethod.Code)

		// Trigger: Invalid Workspace ID in GetWorkspaceDebts
		reqID := httptest.NewRequest(http.MethodGet, "/", nil)
		reqID.SetPathValue("id", "abc") // Not a number
		wID := httptest.NewRecorder()
		debtHandler.GetWorkspaceDebts(wID, reqID)
		assert.Equal(t, http.StatusBadRequest, wID.Code)
	})

	// ==========================================
	// 3. DATABASE CRASH (Nembak baris SendError(w, err))
	// ==========================================
	t.Run("Coverage Boost: Database Connection Crash", func(t *testing.T) {
		sqlDB, _ := db.DB()
		sqlDB.Close() // Force all DB interactions to error

		// GetWorkspaceDebts -> error Usecase -> Internal Error
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("id", "1")
		w := httptest.NewRecorder()
		debtHandler.GetWorkspaceDebts(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)

		// AssignSplitBill -> error Usecase -> SendError(w, err)
		reqS := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"transaction_id": 1}`))
		wS := httptest.NewRecorder()
		debtHandler.AssignSplitBill(wS, reqS)
		assert.NotEqual(t, http.StatusOK, wS.Code)

		// PayDebt -> error Usecase -> SendError(w, err)
		reqP := httptest.NewRequest(http.MethodPatch, "/", nil)
		reqP.SetPathValue("id", "1")
		ctx := context.WithValue(context.Background(), "user_id", uint(1))
		wP := httptest.NewRecorder()
		debtHandler.PayDebt(wP, reqP.WithContext(ctx))
		assert.NotEqual(t, http.StatusOK, wP.Code)
	})
	// ==========================================
	// 4. MISSING COVERAGE: Edge Cases for Handler
	// ==========================================
	t.Run("Coverage Boost: Missing Handler Logic", func(t *testing.T) {
		testutils.CleanTestDB(db)

		// 1. Test invalid JSON payload in AssignSplitBill
		t.Run("Invalid JSON Payload", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{invalid-json`))
			w := httptest.NewRecorder()
			debtHandler.AssignSplitBill(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		// 2. Test invalid debt ID in PayDebt
		t.Run("Invalid Debt ID format", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, "/", nil)
			req.SetPathValue("id", "bukan-angka")
			w := httptest.NewRecorder()
			debtHandler.PayDebt(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		// 3. Test Unauthorized (missing/invalid user_id in context)
		t.Run("Missing User ID in Context", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, "/", nil)
			req.SetPathValue("id", "1")
			// Tanpa context user_id
			w := httptest.NewRecorder()
			debtHandler.PayDebt(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	})
	// ==========================================
	// 5. FINAL COVERAGE: Precise Edge Cases
	// ==========================================
	t.Run("Coverage Boost: Final Cleanup", func(t *testing.T) {
		testutils.CleanTestDB(db)
		user := models.User{Name: "User", Email: "test@test.com"}
		db.Create(&user)
		ctx := context.WithValue(context.Background(), "user_id", user.ID)

		// 1. AssignSplitBill: Test valid payload but trigger validation error inside usecase
		// (Misal: kirim transaction_id yang gak ada di DB)
		t.Run("AssignSplitBill - Trigger Usecase Error", func(t *testing.T) {
			payload := `{"transaction_id": 99999, "items": [{"user_id": 1, "amount": 25000}]}`
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
			w := httptest.NewRecorder()
			debtHandler.AssignSplitBill(w, req.WithContext(ctx))
			assert.NotEqual(t, http.StatusOK, w.Code) // Harus error karena ID gak ada
		})

		// 2. PayDebt: Test dengan User ID yang salah tipe (bukan uint)
		t.Run("PayDebt - Wrong Context Type", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, "/", nil)
			req.SetPathValue("id", "1")
			// Masukin string bukannya uint
			ctxWrong := context.WithValue(context.Background(), "user_id", "bukan-uint")
			w := httptest.NewRecorder()
			debtHandler.PayDebt(w, req.WithContext(ctxWrong))
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	})
}

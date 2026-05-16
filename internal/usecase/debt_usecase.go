package usecase

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
)

type DebtUsecase interface {
	GetWorkspaceDebts(ctx context.Context, workspaceID uint) ([]models.Debt, error)
	ConfirmPayment(ctx context.Context, workspaceID uint, shortCode string, telegramID int64) error
	GenerateUniqueShortCode(workspaceID uint) (string, error)
	AssignSplitBill(ctx context.Context, transactionID uint, items []dto.SplitItemRequest) error
	MarkAsPaid(ctx context.Context, debtID uint, userID uint) error
}

type debtUsecase struct {
	debtRepo repository.DebtRepository
	txRepo   repository.TransactionRepository // <-- Ditambahin biar bisa baca struk asli
}

func NewDebtUsecase(debtRepo repository.DebtRepository, txRepo repository.TransactionRepository) DebtUsecase {
	return &debtUsecase{
		debtRepo: debtRepo,
		txRepo:   txRepo,
	}
}

func (u *debtUsecase) GenerateUniqueShortCode(workspaceID uint) (string, error) {
	chars := "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Tanpa I, O, 0, 1 biar gak bingung
	for {
		code := make([]byte, 4)
		rand.Read(code)
		for i, b := range code {
			code[i] = chars[b%byte(len(chars))]
		}

		finalCode := string(code)
		exists, _ := u.debtRepo.IsShortCodeExists(workspaceID, finalCode)
		if !exists {
			return finalCode, nil
		}
	}
}

func (u *debtUsecase) GetWorkspaceDebts(ctx context.Context, workspaceID uint) ([]models.Debt, error) {
	debts, err := u.debtRepo.GetDebtsByWorkspace(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve workspace debts: %v", err)
	}
	return debts, nil
}

func (u *debtUsecase) ConfirmPayment(ctx context.Context, workspaceID uint, shortCode string, telegramID int64) error {
	// 1. Ambil data utang
	debt, err := u.debtRepo.GetDebtByShortCode(workspaceID, shortCode)
	if err != nil {
		return fmt.Errorf("payment code %s is invalid or already paid", shortCode)
	}

	// 2. Validasi: Apakah yang ngetik command adalah si pengutang?
	if debt.FromUser.TelegramID == nil || int64(*debt.FromUser.TelegramID) != telegramID {
		return fmt.Errorf("unauthorized: this bill belongs to @%s", debt.FromUser.Name)
	}

	// 3. Panggil UpdateIsPaid pake ID utang yang ketemu
	return u.debtRepo.UpdateIsPaid(debt.ID, true)
}

func (u *debtUsecase) AssignSplitBill(ctx context.Context, transactionID uint, items []dto.SplitItemRequest) error {
	var originalTx models.Transaction
	// Perhatikan: Pake txRepo sekarang
	if err := u.txRepo.FindByID(&originalTx, transactionID); err != nil {
		return fmt.Errorf("transaction with ID %d not found", transactionID)
	}

	originalQtyMap := make(map[string]int)
	for _, actualItem := range originalTx.TransactionItems {
		name := strings.TrimSpace(actualItem.Description)
		originalQtyMap[name] = actualItem.Quantity
	}

	var totalInputAmount float64
	inputQtyMap := make(map[string]int)
	userDebts := make(map[uint]float64)

	for _, input := range items {
		itemName := strings.TrimSpace(input.ItemName)

		inputQtyMap[itemName] += input.Quantity
		totalInputAmount += (input.Price * float64(input.Quantity))

		qtyInDB, exists := originalQtyMap[itemName]
		if !exists {
			return fmt.Errorf("item '%s' not found in original receipt", itemName)
		}

		if inputQtyMap[itemName] > qtyInDB {
			return fmt.Errorf("item '%s' quantity exceeded: receipt has %d, but tagged total is %d",
				itemName, qtyInDB, inputQtyMap[itemName])
		}

		if input.UserID != originalTx.UserID {
			userDebts[input.UserID] += (input.Price * float64(input.Quantity))
		}
	}

	if totalInputAmount > (originalTx.Amount + 1) {
		return fmt.Errorf("total split bill amount (Rp%.2f) exceeds original receipt amount (Rp%.2f)",
			totalInputAmount, originalTx.Amount)
	}

	if len(userDebts) > 0 {
		var debtsToSave []models.Debt
		for targetUserID, totalAmount := range userDebts {
			// Perhatikan: Manggil GenerateUniqueShortCode dari object u sendiri
			shortCode, err := u.GenerateUniqueShortCode(originalTx.WorkspaceID)
			if err != nil {
				shortCode = "ERR1"
			}

			debtsToSave = append(debtsToSave, models.Debt{
				WorkspaceID: originalTx.WorkspaceID,
				FromUserID:  targetUserID,
				ToUserID:    originalTx.UserID,
				Amount:      totalAmount,
				ShortCode:   shortCode,
				Note:        "Split bill from merchant: " + originalTx.Merchant,
				IsPaid:      false,
			})
		}
		return u.debtRepo.CreateInBatch(debtsToSave)
	}

	return nil
}

func (u *debtUsecase) MarkAsPaid(ctx context.Context, debtID uint, userID uint) error {
	debt, err := u.debtRepo.FindByID(debtID)
	if err != nil {
		return apperror.NotFound("Debt not found")
	}

	// FIX: Cuma si pengutang yang bisa bayar, pesannya udah English!
	if debt.FromUserID != userID {
		return apperror.Forbidden("Only the debtor can pay this debt")
	}

	if debt.IsPaid {
		return apperror.BadRequest("This debt is already paid")
	}

	err = u.debtRepo.MarkAsPaid(debtID)
	if err != nil {
		return fmt.Errorf("failed to update debt status: %v", err)
	}

	return nil
}

package usecase

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
)

type DebtUsecase interface {
	GetWorkspaceDebts(ctx context.Context, workspaceID uint) ([]models.Debt, error)
	ConfirmPayment(ctx context.Context, workspaceID uint, shortCode string, telegramID int64) error
	GenerateUniqueShortCode(workspaceID uint) (string, error)
}

type debtUsecase struct {
	debtRepo repository.DebtRepository
}

func NewDebtUsecase(debtRepo repository.DebtRepository) DebtUsecase {
	return &debtUsecase{
		debtRepo: debtRepo,
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
		return nil, fmt.Errorf("gagal ambil daftar utang: %v", err)
	}
	return debts, nil
}

func (u *debtUsecase) ConfirmPayment(ctx context.Context, workspaceID uint, shortCode string, telegramID int64) error {
	// 1. Ambil data utang
	debt, err := u.debtRepo.GetDebtByShortCode(workspaceID, shortCode)
	if err != nil {
		return fmt.Errorf("kode bayar %s gak valid atau udah lunas, Mi", shortCode)
	}

	// 2. Validasi: Apakah yang ngetik command adalah si pengutang?
	// Pastikan di repo GetDebtByShortCode lu udah Preload("FromUser") ya!
	if debt.FromUser.TelegramID == nil || int64(*debt.FromUser.TelegramID) != telegramID {
		return fmt.Errorf("ini bukan tagihan lu! Ini tagihannya @%s, Mi", debt.FromUser.Name)
	}

	// 3. Panggil UpdateIsPaid pake ID utang yang ketemu
	return u.debtRepo.UpdateIsPaid(debt.ID, true)
}

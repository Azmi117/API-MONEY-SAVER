package usecase

import (
	"fmt"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
)

type TargetUsecase interface {
	CheckWorkspaceTarget(workspaceID uint) (*dto.BudgetStatusResponse, error)
	AddIncomeToTarget(workspaceID uint, amount float64) error
	SetTarget(req dto.SetTargetRequest) error

	// Nanti fitur Create, Update, Delete Target bulanan lu bisa dimasukin ke sini juga!
}

type targetUsecase struct {
	targetRepo repository.TargetRepository
	txRepo     repository.TransactionRepository
}

func NewTargetUsecase(targetRepo repository.TargetRepository, txRepo repository.TransactionRepository) TargetUsecase {
	return &targetUsecase{
		targetRepo: targetRepo,
		txRepo:     txRepo,
	}
}

// Method buat nambah progress tabungan pas ada income
func (u *targetUsecase) AddIncomeToTarget(workspaceID uint, amount float64) error {
	month := time.Now().Format("2006-01")
	target, err := u.targetRepo.GetByWorkspaceAndPeriod(workspaceID, month)
	if err == nil && target != nil {
		target.CurrentAmount += amount
		return u.targetRepo.Update(target)
	}
	return nil
}

// Method "The Guardian"
func (u *targetUsecase) CheckWorkspaceTarget(workspaceID uint) (*dto.BudgetStatusResponse, error) {
	month := time.Now().Format("2006-01")

	target, err := u.targetRepo.GetByWorkspaceAndPeriod(workspaceID, month)
	if err != nil {
		return nil, fmt.Errorf("target not set")
	}

	totalExpense, _ := u.txRepo.GetTotalByWorkspace(workspaceID, "expense", month)
	expenseSummaries, _ := u.txRepo.GetSummaryByWorkspace(workspaceID, "expense", month)

	var expDetails []dto.MemberSummary
	for _, s := range expenseSummaries {
		expDetails = append(expDetails, dto.MemberSummary{UserName: s.UserName, Total: s.Total})
	}

	return &dto.BudgetStatusResponse{
		Period:          month,
		LimitAmount:     target.AmountLimit,
		TotalExpense:    totalExpense,
		RemainingBudget: target.AmountLimit - totalExpense,
		ExpenseDetails:  expDetails,
	}, nil
}

func (u *targetUsecase) SetTarget(req dto.SetTargetRequest) error {
	target := &models.Target{
		WorkspaceID:   req.WorkspaceID,
		Period:        req.Period,
		AmountLimit:   req.AmountLimit,
		SavingsTarget: req.SavingsTarget,
		IsActive:      true,
	}
	return u.targetRepo.UpsertTarget(target)
}

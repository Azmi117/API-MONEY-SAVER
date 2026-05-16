package dto

type SetTargetRequest struct {
	WorkspaceID   uint    `json:"workspace_id" validate:"required"`
	Period        string  `json:"period" validate:"required"` // Format: 2026-05
	AmountLimit   float64 `json:"amount_limit"`
	SavingsTarget float64 `json:"savings_target"`
}

type MemberSummary struct {
	UserName string  `json:"user_name"`
	Total    float64 `json:"total"`
}

type BudgetStatusResponse struct {
	Period          string          `json:"period"`
	LimitAmount     float64         `json:"limit_amount"`
	TotalExpense    float64         `json:"total_expense"`
	RemainingBudget float64         `json:"remaining_budget"`
	ExpenseDetails  []MemberSummary `json:"expense_details"`
}

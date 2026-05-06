package dto

type SetTargetRequest struct {
	WorkspaceID   uint    `json:"workspace_id" validate:"required"`
	Period        string  `json:"period" validate:"required"` // Format: 2026-05
	AmountLimit   float64 `json:"amount_limit"`
	SavingsTarget float64 `json:"savings_target"`
}

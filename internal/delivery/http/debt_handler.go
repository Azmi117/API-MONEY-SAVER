package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
)

type DebtHandler struct {
	usecase usecase.DebtUsecase
}

func NewDebtHandler(u usecase.DebtUsecase) *DebtHandler {
	return &DebtHandler{u}
}

func (h *DebtHandler) GetWorkspaceDebts(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	workspaceID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		SendError(w, apperror.BadRequest("ID Workspace tidak valid"))
		return
	}

	debts, err := h.usecase.GetWorkspaceDebts(r.Context(), uint(workspaceID))
	if err != nil {
		SendError(w, apperror.Internal(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Daftar tagihan berhasil ditarik",
		"data":    debts,
	})
}

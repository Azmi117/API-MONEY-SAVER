package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/utils"
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

func (h *DebtHandler) AssignSplitBill(w http.ResponseWriter, r *http.Request) {
	var req dto.SplitBillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, apperror.BadRequest("Invalid split bill payload"))
		return
	}

	err := h.usecase.AssignSplitBill(r.Context(), req.TransactionID, req.Items)
	if err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Split bill processed successfully", nil)
}

func (h *DebtHandler) PayDebt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use PATCH"))
		return
	}

	// 1. Ambil ID utang dari URL (pake cara Go 1.22 lu)
	debtIDStr := r.PathValue("id")
	debtID, err := strconv.ParseUint(debtIDStr, 10, 32)
	if err != nil {
		SendError(w, apperror.BadRequest("Invalid debt ID"))
		return
	}

	// 2. Ambil UserID yang lagi login buat validasi (biar orang lain gak bisa sembarangan bayarin/ngubah)
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid Session"))
		return
	}

	// 3. Panggil Usecase (Asumsi nama method-nya MarkAsPaid atau PayDebt)
	err = h.usecase.MarkAsPaid(r.Context(), uint(debtID), userID)
	if err != nil {
		SendError(w, err)
		return
	}

	// 4. Balikin response sukses
	utils.RespondWithJSON(w, http.StatusOK, "success", "Debt successfully marked as paid!", nil)
}

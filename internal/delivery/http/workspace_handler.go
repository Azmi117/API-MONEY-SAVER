package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/utils"
)

type WorkspaceHandler struct {
	usecase       usecase.WorkspaceUsecase
	targetUsecase usecase.TargetUsecase
}

type UpdateWorkspaceRequest struct {
	Name string `json:"name"`
}

func NewWorkspaceHandler(u usecase.WorkspaceUsecase, tU usecase.TargetUsecase) *WorkspaceHandler {
	return &WorkspaceHandler{usecase: u, targetUsecase: tU}
}

// 1. CREATE WORKSPACE
func (h *WorkspaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, please use POST"))
		return
	}

	// 1. UPDATE STRUCT INPUT
	var input struct {
		Name string `json:"name"`
		Type string `json:"type"` // TAMBAHAN BARU CUY
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		SendError(w, apperror.BadRequest("Invalid JSON payload"))
		return
	}

	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	// 2. TAMBAHIN PARAMETER TYPE KE USECASE
	ws, err := h.usecase.CreateWorkspace(input.Name, input.Type, userID)
	if err != nil {
		SendError(w, err)
		return
	}

	// 3. JANGAN LUPA DIBALIKIN JUGA TYPE-NYA KE FRONTEND
	response := dto.WorkspaceResponse{
		ID:        ws.ID,
		Name:      ws.Name,
		Type:      ws.Type, // PASTIKAN ADA DI DTO (liat poin 4)
		OwnerID:   ws.OwnerID,
		CreatedAt: ws.CreatedAt,
	}

	utils.RespondWithJSON(w, http.StatusCreated, "success", "Workspace created successfully", response)
}

// 2. GET MY WORKSPACES
func (h *WorkspaceHandler) GetMyWorkspaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, please use GET"))
		return
	}

	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	workspaces, err := h.usecase.GetUserWorkspaces(userID)
	if err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Workspaces retrieved successfully", workspaces)
}

// 3. UPDATE WORKSPACE
func (h *WorkspaceHandler) UpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, please use PUT"))
		return
	}

	var input UpdateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		SendError(w, apperror.BadRequest("Invalid JSON payload"))
		return
	}

	// FIX: Pindah ke PathValue biar sesuai sama "/workspaces/{id}"
	wsIDStr := r.PathValue("id")
	wsID, _ := strconv.Atoi(wsIDStr)
	userID := r.Context().Value("user_id").(uint)

	err := h.usecase.UpdateWorkspace(uint(wsID), userID, input.Name)
	if err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Workspace updated successfully", nil)
}

// 4. DELETE WORKSPACE
func (h *WorkspaceHandler) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, please use DELETE"))
		return
	}

	// FIX: Pindah ke PathValue juga
	wsIDStr := r.PathValue("id")
	wsID, _ := strconv.Atoi(wsIDStr)
	userID := r.Context().Value("user_id").(uint)

	err := h.usecase.DeleteWorkspace(uint(wsID), userID)
	if err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Workspace deleted successfully", nil)
}

// 5. INVITE MEMBER
func (h *WorkspaceHandler) Invite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, please use POST"))
		return
	}

	wsIDStr := r.PathValue("id")
	wsID, _ := strconv.ParseUint(wsIDStr, 10, 32)

	var input struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		SendError(w, apperror.BadRequest("Invalid JSON payload"))
		return
	}

	ownerID := r.Context().Value("user_id").(uint)

	err := h.usecase.InviteMember(uint(wsID), ownerID, input.Email)
	if err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Invitation sent successfully to "+input.Email, nil)
}

// 6A. ACCEPT INVITATION
func (h *WorkspaceHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	invIDStr := r.PathValue("id")
	invID, _ := strconv.ParseUint(invIDStr, 10, 32)
	userID := r.Context().Value("user_id").(uint)

	err := h.usecase.AcceptInvitation(uint(invID), userID)
	if err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Successfully joined the workspace", nil)
}

// 6B. REJECT INVITATION
func (h *WorkspaceHandler) RejectInvitation(w http.ResponseWriter, r *http.Request) {
	invIDStr := r.PathValue("id")
	invID, _ := strconv.ParseUint(invIDStr, 10, 32)
	userID := r.Context().Value("user_id").(uint)

	err := h.usecase.RejectInvitation(uint(invID), userID)
	if err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Invitation rejected successfully", nil)
}

// 7. SET TARGET
func (h *WorkspaceHandler) SetTarget(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	var req dto.SetTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, apperror.BadRequest("Invalid JSON payload"))
		return
	}

	// --- 🚀 TAMBAHAN VALIDASI TIPE WORKSPACE ---
	ws, err := h.usecase.GetWorkspaceByID(req.WorkspaceID)
	if err != nil {
		SendError(w, err) // Ini otomatis ngeluarin error NotFound dari usecase
		return
	}

	// Cek apakah tipenya bukan budgeting
	if ws.Type != "budgeting" {
		SendError(w, apperror.BadRequest("Fitur Set Target hanya bisa digunakan untuk workspace tipe 'budgeting'"))
		return
	}

	// Opsional: Pastikan yang nge-set target cuma owner atau membernya (Biar gak bisa ditembak dari Postman sembarangan)
	if ws.OwnerID != userID {
		SendError(w, apperror.Forbidden("Access denied: You don't have permission to set target for this workspace"))
		return
	}
	// ------------------------------------------

	if err := h.targetUsecase.SetTarget(req); err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Target for period "+req.Period+" has been set successfully", nil)
}

// 8. GET MEMBERS
func (h *WorkspaceHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	workspaceID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		SendError(w, apperror.BadRequest("Invalid workspace ID format"))
		return
	}

	members, err := h.usecase.GetMembers(uint(workspaceID))
	if err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Workspace members retrieved successfully", members)
}
func (h *WorkspaceHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/workspaces/")
	idStr = strings.TrimSuffix(idStr, "/summary")
	id, _ := strconv.Atoi(idStr)

	// Tangkap query param period dari URL (contoh: ?period=2026-06)
	period := r.URL.Query().Get("period")
	if period == "" {
		// Kasih default value kalau frontend lupa ngirim, misal ke bulan ini
		period = time.Now().Format("2006-01")
	}

	// Lempar period-nya ke usecase
	summary, err := h.usecase.GetWorkspaceSummary(id, period)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Gagal"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status_code": 200,
		"message":     "Summary retrieved",
		"data":        summary,
	})
}

// GET PENDING INVITATIONS
func (h *WorkspaceHandler) GetPendingInvitations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, apperror.MethodNotAllowed("Method not allowed"))
		return
	}

	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	invitations, err := h.usecase.GetPendingInvitations(userID)
	if err != nil {
		SendError(w, err)
		return
	}

	// Mapping ke format response yang siap dimakan Frontend
	var response []map[string]interface{}
	for _, inv := range invitations {
		response = append(response, map[string]interface{}{
			"id":            inv.ID,
			"workspaceName": inv.Workspace.Name, // Asumsi Repo lu udah nge-Preload Workspace
			"sender":        inv.Inviter.Name,   // Asumsi Repo lu udah nge-Preload Inviter (User)
			"status":        inv.Status,
		})
	}

	// Biar gak return null kalau kosong, kita return array kosong []
	if response == nil {
		response = []map[string]interface{}{}
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Pending invitations retrieved", response)
}

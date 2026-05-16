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

	var input struct {
		Name string `json:"name"`
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

	ws, err := h.usecase.CreateWorkspace(input.Name, userID)
	if err != nil {
		SendError(w, err)
		return
	}

	response := dto.WorkspaceResponse{
		ID:        ws.ID,
		Name:      ws.Name,
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
	_, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	var req dto.SetTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, apperror.BadRequest("Invalid JSON payload"))
		return
	}

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

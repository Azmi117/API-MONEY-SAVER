package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
)

type WorkspaceHandler struct {
	usecase usecase.WorkspaceUsecase
}

type UpdateWorkspaceRequest struct {
	Name string `json:"name"`
}

func NewWorkspaceHandler(u usecase.WorkspaceUsecase) *WorkspaceHandler {
	return &WorkspaceHandler{usecase: u}
}

// 1. CREATE WORKSPACE
func (h *WorkspaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use POST"))
		return
	}

	var input struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		// Lo bisa pake sendError di sini kalau mau seragam
		SendError(w, apperror.BadRequest("Invalid JSON format"))
		return
	}

	// Ambil UserID dari Context (hasil Middleware Authenticate)
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		// Handle error session
		SendError(w, apperror.Unauthorized("User session invalid"))
		return
	}

	ws, err := h.usecase.CreateWorkspace(input.Name, userID)
	if err != nil {
		// Kirim error sesuai business logic (misal: limit tier)
		SendError(w, err)
		return
	}

	// 2. Mapping dari 'ws' (model GORM) ke 'response' (DTO)
	response := dto.WorkspaceResponse{
		ID:        ws.ID,
		Name:      ws.Name,
		OwnerID:   ws.OwnerID,
		CreatedAt: ws.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// 2. GET MY WORKSPACES
func (h *WorkspaceHandler) GetMyWorkspaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use GET"))
		return
	}

	userID := r.Context().Value("user_id").(uint)
	workspaces, err := h.usecase.GetUserWorkspaces(userID)
	if err != nil {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": workspaces})
}

// 3. UPDATE WORKSPACE (Owner Only - Di-filter via Middleware)
func (h *WorkspaceHandler) UpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut { // Pastikan method-nya bener (PUT/PATCH)
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use PUT"))
		return
	}

	// Ganti pake DTO yang di atas tadi
	var input UpdateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		return
	}

	// Ambil ID dari query param ?id=1
	wsIDStr := r.URL.Query().Get("id")
	wsID, _ := strconv.Atoi(wsIDStr)
	userID := r.Context().Value("user_id").(uint)

	err := h.usecase.UpdateWorkspace(uint(wsID), userID, input.Name)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// Kalau Usecase lu balikin data workspace terbaru, mapping ke WorkspaceResponse di sini.
	// Tapi kalau cuma mau message sukses, ini udah cukup:
	json.NewEncoder(w).Encode(map[string]string{"message": "Workspace updated successfully"})
}

// 4. DELETE WORKSPACE (Owner Only - Di-filter via Middleware)
func (h *WorkspaceHandler) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete { // Pastikan method-nya bener (PUT/PATCH)
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use DELETE"))
		return
	}

	wsIDStr := r.URL.Query().Get("id")
	wsID, _ := strconv.Atoi(wsIDStr)
	userID := r.Context().Value("user_id").(uint)

	err := h.usecase.DeleteWorkspace(uint(wsID), userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Workspace deleted successfully"})
}

// 5. INVITE MEMBER
// 5. INVITE MEMBER
func (h *WorkspaceHandler) Invite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use POST"))
		return
	}

	// Ambil WorkspaceID dari Path Parameter (Go 1.22+ style)
	wsIDStr := r.PathValue("id")
	wsID, _ := strconv.ParseUint(wsIDStr, 10, 32)

	var input struct {
		Email string `json:"email"` // Sekarang pake Email
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		SendError(w, apperror.BadRequest("Invalid JSON format"))
		return
	}

	ownerID := r.Context().Value("user_id").(uint)

	// Kirim Email ke Usecase
	err := h.usecase.InviteMember(uint(wsID), ownerID, input.Email)
	if err != nil {
		SendError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Invitation sent successfully to " + input.Email})
}

// 6. RESPOND TO INVITATION (Accept/Reject)
// 6A. ACCEPT INVITATION
func (h *WorkspaceHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	// Ambil ID dari URL Path
	invIDStr := r.PathValue("id")
	invID, _ := strconv.ParseUint(invIDStr, 10, 32)
	userID := r.Context().Value("user_id").(uint)

	err := h.usecase.AcceptInvitation(uint(invID), userID)
	if err != nil {
		SendError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Successfully joined the workspace!"})
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Invitation rejected!"})
}

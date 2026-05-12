package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
)

type CategoryHandler struct {
	repo repository.CategoryRepository
}

func NewCategoryHandler(repo repository.CategoryRepository) *CategoryHandler {
	return &CategoryHandler{repo: repo}
}

// 1. CREATE CATEGORY
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use POST"))
		return
	}

	// Ambil WorkspaceID dari Path (Go 1.22+ style)
	wsIDStr := r.PathValue("id")
	wsID, _ := strconv.ParseUint(wsIDStr, 10, 32)

	var input struct {
		Name string `json:"name"`
		Type string `json:"type"` // income/expense
		Icon string `json:"icon"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		SendError(w, apperror.BadRequest("Invalid JSON format"))
		return
	}

	category := models.Category{
		Name:        input.Name,
		WorkspaceID: uint(wsID),
		Type:        input.Type,
		Icon:        input.Icon,
	}

	if err := h.repo.Create(&category); err != nil {
		SendError(w, apperror.Internal("Gagal bikin kategori"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(category)
}

// 2. GET CATEGORIES BY WORKSPACE
func (h *CategoryHandler) GetByWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use GET"))
		return
	}

	wsIDStr := r.PathValue("id")
	wsID, _ := strconv.ParseUint(wsIDStr, 10, 32)

	categories, err := h.repo.GetByWorkspace(uint(wsID))
	if err != nil {
		SendError(w, apperror.Internal("Gagal ambil data kategori"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"data": categories})
}

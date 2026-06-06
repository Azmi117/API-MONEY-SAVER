package utils

import (
	"encoding/json"
	"net/http"
)

type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
type PaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"` // Opsional, tapi bagus buat FE
}

type PaginatedResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    interface{}    `json:"data"`
	Meta    PaginationMeta `json:"meta"`
}

func RespondWithJSON(w http.ResponseWriter, code int, status string, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	json.NewEncoder(w).Encode(APIResponse{
		Status:  status,
		Message: message,
		Data:    data,
	})
}
func SuccessPaginatedResponse(code int, message string, data interface{}, meta PaginationMeta) PaginatedResponse {
	return PaginatedResponse{
		Code:    code,
		Message: message,
		Data:    data,
		Meta:    meta,
	}
}

func NullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

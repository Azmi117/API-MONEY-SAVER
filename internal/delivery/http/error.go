package http

import (
	"encoding/json"
	"net/http"

	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
)

func SendError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")

	if appErr, ok := err.(*apperror.Apperror); ok {
		w.WriteHeader(appErr.Code)
		json.NewEncoder(w).Encode(appErr)
		return
	}

	json.NewEncoder(w).Encode(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{"message": "Internal Server Error"})
}

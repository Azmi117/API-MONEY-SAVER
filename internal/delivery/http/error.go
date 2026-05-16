package http

import (
	"net/http"

	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/utils"
)

func SendError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperror.Apperror); ok {
		utils.RespondWithJSON(w, appErr.Code, "error", appErr.Message, nil)
		return
	}

	// Default error jika bukan dari apperror
	utils.RespondWithJSON(w, http.StatusInternalServerError, "error", "Internal Server Error", nil)
}

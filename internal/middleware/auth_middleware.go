package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

func sendError(w http.ResponseWriter, err error) {
	appErr := err.(*apperror.Apperror)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.Code)
	json.NewEncoder(w).Encode(appErr)
}

func Authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Ambil JWT_SECRET
		jwtSecret := os.Getenv("JWT_SECRET")

		if jwtSecret == "" {
			sendError(w, apperror.Internal("Failed load JWT SECRET from .env!"))
		}

		// 2. Ambil token dari cookie
		cookie, err := r.Cookie("token")
		if err != nil {
			sendError(w, apperror.Unauthorized("No token is exist!"))
		}

		// 3. Parsing token(Bongkar)
		tokenString := cookie.Value
		token, _ := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})

		// 1. Bongkar isi token (Claims) dan pastiin tokennya beneran asli (Valid)
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {

			// 2. Ambil UserID dari claims, ubah dari desimal (JSON) ke angka bulat (uint)
			userID := uint(claims["user_id"].(float64))

			// 3. Titip UserID ke "tas" request (Context) pake label "user_id"
			ctx := context.WithValue(r.Context(), "user_id", userID)

			// 4. Lanjut jalan ke Handler tujuan sambil bawa identitas (UserID) tadi
			next.ServeHTTP(w, r.WithContext(ctx))

		} else {
			// 5. Kalau token palsu atau rusak, usir (Unauthorized)
			sendError(w, apperror.Unauthorized("Invalid Session, please relogin!"))
		}

	}
}

func AuthorizeWorkspaceOwner(db *gorm.DB) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// 1. Ambil UserID dari context
			userID, ok := r.Context().Value("user_id").(uint)
			if !ok {
				sendError(w, apperror.Unauthorized("Invalid Session, please relogin!"))
				return
			}

			// 2. Ambil workspace_id dari query param (misal: /api/ws?id=1)
			wsID := r.URL.Query().Get("id")

			// 3. Cek di DB apakah dia Owner
			var ws models.Workspace
			err := db.Where("id = ? AND owner_id = ?", wsID, userID).First(&ws).Error

			if err != nil {
				// 4. PAKE CUSTOM ERROR LO!
				sendError(w, apperror.Forbidden("Only the owner can change this workspace!"))
				return
			}

			next.ServeHTTP(w, r)
		}
	}
}

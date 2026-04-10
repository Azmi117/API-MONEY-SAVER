package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
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

func Authenticate(repo repository.AuthRepository) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			jwtSecret := os.Getenv("JWT_SECRET")

			// 1. Ambil token dari cookie (Pastiin namanya "access_token" sesuai handler lo)
			cookie, err := r.Cookie("access_token")
			if err != nil {
				sendError(w, apperror.Unauthorized("No token is exist!"))
				return
			}

			tokenString := cookie.Value

			// --- TAMBAHAN LOGIC BLACKLIST ---
			// Cek ke DB: Apakah token ini sudah di-logout (revoked)?
			if repo.IsTokenRevoked(tokenString) {
				sendError(w, apperror.Unauthorized("Token sudah tidak berlaku, silakan login lagi!"))
				return
			}
			// --------------------------------

			// 2. Parsing JWT
			token, _ := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
				return []byte(jwtSecret), nil
			})

			if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
				userID := uint(claims["user_id"].(float64))
				ctx := context.WithValue(r.Context(), "user_id", userID)
				next.ServeHTTP(w, r.WithContext(ctx))
			} else {
				sendError(w, apperror.Unauthorized("Invalid Session, please relogin!"))
			}
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
			wsIDStr := r.URL.Query().Get("id")
			wsID, _ := strconv.Atoi(wsIDStr)

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

func AuthorizeWorkspaceMember(db *gorm.DB) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value("user_id").(uint)
			if !ok {
				sendError(w, apperror.Unauthorized("Invalid Session, please relogin!"))
				return
			}

			// Cek workspace_id dari query param
			wsIDStr := r.URL.Query().Get("workspace_id")
			if wsIDStr == "" {
				wsIDStr = r.URL.Query().Get("id") // fallback ke 'id'
			}
			wsID, _ := strconv.Atoi(wsIDStr)

			var member models.WorkspaceMember
			// Cek apakah UserID ini terdaftar sebagai member di WorkspaceID ini
			err := db.Where("workspace_id = ? AND user_id = ?", wsID, userID).First(&member).Error

			if err != nil {
				sendError(w, apperror.Forbidden("You are not a member of this workspace!"))
				return
			}

			next.ServeHTTP(w, r)
		}
	}
}

package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/service"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
)

type authHandler struct {
	usecase           usecase.AuthUsecase
	googleAuthService service.GoogleAuthService
}

// Update constructor biar nerima googleAuthService juga
func NewAuthHandler(params usecase.AuthUsecase, googleService service.GoogleAuthService) *authHandler {
	return &authHandler{
		usecase:           params,
		googleAuthService: googleService,
	}
}

func sendError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")

	if appErr, ok := err.(*apperror.Apperror); ok {
		w.WriteHeader(appErr.Code)
		json.NewEncoder(w).Encode(appErr)
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{"message": "Internal Server Error"})
}

// --- GOOGLE OAUTH HANDLERS ---

func (h *authHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	// Ambil userID dari context (setelah lewat authMiddleware)
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		sendError(w, apperror.Unauthorized("User tidak terautentikasi"))
		return
	}

	// State kita isi userID buat identifikasi pas callback
	url := h.googleAuthService.GetAuthURL(userID)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *authHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		sendError(w, apperror.BadRequest("Code dari Google tidak ditemukan"))
		return
	}

	// Parsing state balik jadi userID
	var userID uint
	_, err := fmt.Sscanf(state, "%d", &userID)
	if err != nil {
		sendError(w, apperror.BadRequest("State tidak valid"))
		return
	}

	// Tukar code jadi Refresh Token
	err = h.googleAuthService.ExchangeCode(r.Context(), userID, code)
	if err != nil {
		sendError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte("<h1>Berhasil!</h1><p>Gmail Mandiri lo udah kekonek.</p>"))
}

// --- EXISTING HANDLERS ---

func (h *authHandler) Register(w http.ResponseWriter, r *http.Request) {
	// 1. Parse Form (Maks 5MB)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		sendError(w, apperror.BadRequest("File too large. Maximum size allowed is 5MB!"))
		return
	}

	// 2. Mapping sesuai Model User lo
	teleID, _ := strconv.Atoi(r.FormValue("telegram_id"))

	input := models.User{
		Name:               r.FormValue("name"),
		Email:              r.FormValue("email"),
		PasswordHash:       r.FormValue("password"),
		TelegramID:         teleID,
		EmailParsingEnable: r.FormValue("email_parsing_enable") == "true",
	}

	// 3. Logika Avatar
	file, header, err := r.FormFile("avatar")
	if err == nil {
		defer file.Close()

		if _, err := os.Stat("./uploads/avatar"); os.IsNotExist(err) {
			os.MkdirAll("./uploads/avatar", os.ModePerm)
		}

		fileName := time.Now().Format("20060102150405") + "-" + header.Filename
		filePath := "./uploads/avatar/" + fileName

		dst, err := os.Create(filePath)
		if err != nil {
			sendError(w, apperror.Internal("Failed to save uploaded file: "+err.Error()))
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			sendError(w, apperror.Internal("Failed to save uploaded file.: "+err.Error()))
			return
		}

		input.Avatar = fileName
	} else if err != http.ErrMissingFile {
		sendError(w, apperror.BadRequest("Error retrieving the uploaded file: "+err.Error()))
		return
	}

	if err := h.usecase.Register(&input); err != nil {
		sendError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Register success"})
}

func (h *authHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		sendError(w, apperror.BadRequest("JSON Error"))
		return
	}

	accessToken, refreshToken, err := h.usecase.Login(input.Email, input.Password)
	if err != nil {
		sendError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		HttpOnly: true,
		Path:     "/",
		Expires:  time.Now().Add(15 * time.Minute),
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		HttpOnly: true,
		Path:     "/",
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		SameSite: http.SameSiteLaxMode,
	})

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Login Success!",
	})
}

func (h *authHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		sendError(w, apperror.Unauthorized("Session expired, please login again!"))
		return
	}

	newAccessToken, err := h.usecase.RefreshToken(cookie.Value)
	if err != nil {
		sendError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    newAccessToken,
		HttpOnly: true,
		Path:     "/",
		Expires:  time.Now().Add(15 * time.Minute),
	})

	json.NewEncoder(w).Encode(map[string]string{"message": "Token Refreshed!"})
}

func (h *authHandler) Logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	atCookie, errAT := r.Cookie("access_token")
	rtCookie, errRT := r.Cookie("refresh_token")

	if errAT == nil && errRT == nil {
		h.usecase.Logout(atCookie.Value, rtCookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Logout sucess! Session cleared and token revoked.",
	})
}

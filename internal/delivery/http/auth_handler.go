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
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/utils"
)

type authHandler struct {
	usecase           usecase.AuthUsecase
	googleAuthService service.GoogleAuthService
}

func NewAuthHandler(params usecase.AuthUsecase, googleService service.GoogleAuthService) *authHandler {
	return &authHandler{
		usecase:           params,
		googleAuthService: googleService,
	}
}

// --- GOOGLE OAUTH HANDLERS ---

func (h *authHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	url := h.googleAuthService.GetAuthURL(userID)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *authHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		SendError(w, apperror.BadRequest("Google authorization failed: missing code"))
		return
	}

	var userID uint
	_, err := fmt.Sscanf(state, "%d", &userID)
	if err != nil {
		SendError(w, apperror.BadRequest("Invalid state parameter"))
		return
	}

	err = h.googleAuthService.ExchangeCode(r.Context(), userID, code)
	if err != nil {
		SendError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte("<h1>Success!</h1><p>Your Gmail account has been successfully linked.</p>"))
}

// --- EXISTING HANDLERS ---

func (h *authHandler) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		SendError(w, apperror.BadRequest("File size exceeds 5MB limit"))
		return
	}

	teleID, _ := strconv.Atoi(r.FormValue("telegram_id"))

	input := models.User{
		Name:               r.FormValue("name"),
		Email:              r.FormValue("email"),
		PasswordHash:       r.FormValue("password"),
		TelegramID:         &teleID,
		EmailParsingEnable: r.FormValue("email_parsing_enable") == "true",
	}

	// Avatar Processing logic
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
			SendError(w, apperror.Internal("Failed to save profile picture"))
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			SendError(w, apperror.Internal("Failed to process profile picture"))
			return
		}
		input.Avatar = fileName
	} else if err != http.ErrMissingFile {
		SendError(w, apperror.BadRequest("Error processing uploaded file"))
		return
	}

	if err := h.usecase.Register(&input); err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, "success", "Registration successful. Please verify your email.", nil)
}

func (h *authHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		SendError(w, apperror.BadRequest("Invalid JSON format"))
		return
	}

	accessToken, refreshToken, err := h.usecase.Login(input.Email, input.Password)
	if err != nil {
		SendError(w, err)
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

	utils.RespondWithJSON(w, http.StatusOK, "success", "Login successful", nil)
}

func (h *authHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		SendError(w, apperror.Unauthorized("Session expired, please log in again"))
		return
	}

	newAccessToken, err := h.usecase.RefreshToken(cookie.Value)
	if err != nil {
		SendError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    newAccessToken,
		HttpOnly: true,
		Path:     "/",
		Expires:  time.Now().Add(15 * time.Minute),
	})

	utils.RespondWithJSON(w, http.StatusOK, "success", "Token successfully refreshed", nil)
}

func (h *authHandler) Logout(w http.ResponseWriter, r *http.Request) {
	atCookie, errAT := r.Cookie("access_token")
	rtCookie, errRT := r.Cookie("refresh_token")

	if errAT == nil && errRT == nil {
		h.usecase.Logout(atCookie.Value, rtCookie.Value)
	}

	// Clear Cookies
	cookies := []string{"access_token", "refresh_token"}
	for _, name := range cookies {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
		})
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Logout successful and session cleared", nil)
}

func (h *authHandler) GetBindingCode(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("User identification failed"))
		return
	}

	code, err := h.usecase.RequestBindingCode(userID)
	if err != nil {
		SendError(w, err)
		return
	}

	data := map[string]string{
		"binding_code": code,
		"instruction":  "Send this code to the Telegram bot via /bind [code]",
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Binding code generated successfully", data)
}

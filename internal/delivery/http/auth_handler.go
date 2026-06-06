package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

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
	// 1. authMW bekerja sempurna di sini karena dipanggil via Axios
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	// 2. Generate URL dari Service
	url := h.googleAuthService.GetAuthURL(userID)

	// 3. JANGAN REDIRECT DARI SINI. Balikin sebagai JSON ke Frontend!
	utils.RespondWithJSON(w, http.StatusOK, "success", "Google Auth URL generated", map[string]string{
		"url": url,
	})
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

	// 1. Proses penukaran code dan update database
	err = h.googleAuthService.ExchangeCode(r.Context(), userID, code)
	if err != nil {
		SendError(w, err)
		return
	}

	// 2. Redirect balik ke frontend (Profile Page)
	// Asumsi FE lo jalan di port 5173 atau sesuaikan dengan base URL frontend lo
	redirectURL := os.Getenv("FRONTEND_URL") + "/profile?sync=success"
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// --- EXISTING HANDLERS ---

func (h *authHandler) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		// Log error-nya biar lu tau sebenernya kenapa
		log.Printf("Error parsing form: %v", err)
		SendError(w, apperror.BadRequest("Error parsing form data"))
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

// 1. REFACTOR: Handler Login utama (Step 1 - Cek Pass & trigger OTP)
func (h *authHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		SendError(w, apperror.BadRequest("Invalid JSON format"))
		return
	}

	// Panggil usecase login yang mengembalikan error saja
	_, err := h.usecase.Login(input.Email, input.Password)
	if err != nil {
		if apperror.IsAccountNotVerified(err) {
			// Ambil email dari error (atau custom struct error)
			email := apperror.GetEmailFromError(err)
			utils.RespondWithJSON(w, http.StatusForbidden, "error", "Not verified", map[string]interface{}{
				"code":  "USER_NOT_VERIFIED",
				"email": email,
			})
			return
		}
		SendError(w, err)
		return
	}

	// Kirim response status 202 (Accepted) menandakan password benar & OTP dikirim
	utils.RespondWithJSON(w, http.StatusAccepted, "success", "Password verified. An OTP security code has been sent to your email.", nil)
}

// 2. NEW HANDLER: Handler untuk verifikasi OTP Login (Step 2 - Tukar OTP ke JWT)
func (h *authHandler) VerifyLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed. Please use POST."))
		return
	}

	var req struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, apperror.BadRequest("Invalid JSON payload."))
		return
	}

	// Jalankan verifikasi OTP khusus Login
	accessToken, refreshToken, err := h.usecase.VerifyLoginOTP(req.Email, req.Code)
	if err != nil {
		SendError(w, err)
		return
	}

	// Setelah lolos verifikasi OTP, baru pasang Cookies JWT-nya di sini!
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

	utils.RespondWithJSON(w, http.StatusOK, "success", "Login verification successful. Welcome!", nil)
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
		SameSite: http.SameSiteLaxMode,
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

func (h *authHandler) VerifyRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed. Please use POST."))
		return
	}

	var req struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, apperror.BadRequest("Invalid JSON payload."))
		return
	}

	if err := h.usecase.VerifyRegisterOTP(req.Email, req.Code); err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Account successfully activated. Please log in.", nil)
}

func (h *authHandler) ResendOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed. Please use POST."))
		return
	}

	var req struct {
		Email string `json:"email"`
		Type  string `json:"type"` // "register", "login", dll
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, apperror.BadRequest("Invalid JSON payload."))
		return
	}

	// Cari user ID dari email dulu
	user, err := h.usecase.GetUserByEmail(req.Email)
	if err != nil {
		SendError(w, apperror.NotFound("User not found."))
		return
	}

	if err := h.usecase.SendOrResendOTP(user.ID, req.Email, req.Type); err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "OTP code has been successfully resent to your email.", nil)
}

// Helper buat ngambil config khusus SSO
func getSSOConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_SSO_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_SSO_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_SSO_REDIRECT_URL"),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

// Handler 1: Ngarahin user ke halaman Login Google
func (h *authHandler) GoogleSSOLogin(w http.ResponseWriter, r *http.Request) {
	cfg := getSSOConfig()
	// Pake random state, buat keamanan CSRF
	url := cfg.AuthCodeURL("sso-state-random-string")
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// Handler 2: Nerima balasan dari Google (Callback)
func (h *authHandler) GoogleSSOCallback(w http.ResponseWriter, r *http.Request) {
	state := r.FormValue("state")
	if state != "sso-state-random-string" {
		SendError(w, apperror.BadRequest("Invalid OAuth state"))
		return
	}

	code := r.FormValue("code")
	if code == "" {
		SendError(w, apperror.BadRequest("Authorization code not found"))
		return
	}

	cfg := getSSOConfig()
	token, err := cfg.Exchange(context.Background(), code)
	if err != nil {
		SendError(w, apperror.Internal("Failed to exchange authorization code"))
		return
	}

	client := cfg.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		SendError(w, apperror.Internal("Failed to fetch user info from Google"))
		return
	}
	defer resp.Body.Close()

	var userInfo struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		SendError(w, apperror.Internal("Failed to parse user info"))
		return
	}

	// Panggil Usecase SSO lu
	accToken, refToken, err := h.usecase.LoginSSO(userInfo.Email, userInfo.Name)
	if err != nil {
		SendError(w, err)
		return
	}

	// 👇 INI BAGIAN UTAMANYA: Set HttpOnly Cookies biar FE gak usah ribet nyimpen token 👇
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accToken,
		HttpOnly: true,
		Path:     "/",
		Expires:  time.Now().Add(15 * time.Minute),
		SameSite: http.SameSiteLaxMode, // Sesuaikan sama setting backend lu
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refToken,
		HttpOnly: true,
		Path:     "/",
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		SameSite: http.SameSiteLaxMode,
	})

	// LANGSUNG REDIRECT KE FRONTEND DASHBOARD
	http.Redirect(w, r, "http://localhost:5173/dashboard", http.StatusSeeOther)
}

// 1. Handler Minta OTP Lupa Password (POST /auth/forgot-password)
func (h *authHandler) ForgotPasswordRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed"))
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, apperror.BadRequest("Invalid JSON"))
		return
	}

	err := h.usecase.ForgotPasswordRequest(req.Email)
	if err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "OTP code for password reset has been sent to your email.", nil)
}

// 2. Handler Eksekusi Ganti Password (POST /auth/forgot-password/verify)
func (h *authHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed"))
		return
	}

	var req struct {
		Email       string `json:"email"`
		Code        string `json:"code"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, apperror.BadRequest("Invalid JSON payload"))
		return
	}

	if len(req.NewPassword) < 6 {
		SendError(w, apperror.BadRequest("Password must be at least 6 characters long"))
		return
	}

	err := h.usecase.ResetPassword(req.Email, req.Code, req.NewPassword)
	if err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Your password has been successfully reset. Please log in with your new password.", nil)
}

func (h *authHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	// Ambil dari context (sesuai middleware lu)
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	// Panggil usecase, bukan langsung repo!
	user, err := h.usecase.FindByID(uint(userID))
	if err != nil {
		SendError(w, apperror.NotFound("User not found"))
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Profile retrieved", user)
}

func (h *authHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, please use PUT"))
		return
	}

	// 1. Ambil UserID dari middleware
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	// 2. Parse Multipart Form (Max size 5 MB)
	err := r.ParseMultipartForm(5 << 20)
	if err != nil {
		SendError(w, apperror.BadRequest("File terlalu besar atau format form tidak valid"))
		return
	}

	name := r.FormValue("name")
	if name == "" {
		SendError(w, apperror.BadRequest("Nama tidak boleh kosong"))
		return
	}

	// 3. Handle File Upload (Opsional, karena user bisa aja cuma ganti nama)
	var avatarURL string
	file, header, err := r.FormFile("avatar")
	if err == nil { // Kalau nggak ada error, berarti ada file yang diupload
		defer file.Close()

		// Bikin folder "uploads/avatars" kalau belum ada (Permission 0755)
		os.MkdirAll("./uploads/avatars", os.ModePerm)

		// Generate nama file unik biar nggak bentrok (Contoh: 1_1708899.jpg)
		ext := filepath.Ext(header.Filename)
		filename := fmt.Sprintf("%d_%d%s", userID, time.Now().Unix(), ext)
		savePath := filepath.Join(".", "uploads", "avatars", filename)

		// Bikin file kosong di folder tersebut
		dst, err := os.Create(savePath)
		if err != nil {
			SendError(w, apperror.Internal("Gagal menyimpan file gambar"))
			return
		}
		defer dst.Close()

		// Copy isi file dari memori ke file di hardisk
		if _, err := io.Copy(dst, file); err != nil {
			SendError(w, apperror.Internal("Gagal menyalin file gambar"))
			return
		}

		// Simpen path relatifnya ke database. Karena lu udah punya route "/uploads/"
		// di routes.go, frontend nanti bakal bisa akses gambar ini.
		avatarURL = "http://localhost:8080/uploads/avatars/" + filename
	}

	// 4. Panggil Usecase buat simpen ke DB
	err = h.usecase.UpdateProfile(userID, name, avatarURL)
	if err != nil {
		SendError(w, err)
		return
	}

	// 5. Balikin Response
	responseData := map[string]string{"name": name}
	if avatarURL != "" {
		responseData["avatar"] = avatarURL
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Profil berhasil diupdate", responseData)
}

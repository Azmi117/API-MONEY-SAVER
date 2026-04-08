package http

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
)

type authHandler struct {
	usecase usecase.AuthUsecase
}

func NewAuthHandler(params usecase.AuthUsecase) *authHandler {
	return &authHandler{
		usecase: params,
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

func (h *authHandler) Register(w http.ResponseWriter, r *http.Request) {
	// 1. Parse Form (Maks 5MB)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		sendError(w, apperror.BadRequest("File kegedean, Mi!"))
		return
	}

	// 2. Mapping sesuai Model User lo
	// telegram_id harus di-convert dari string ke int
	teleID, _ := strconv.Atoi(r.FormValue("telegram_id"))

	input := models.User{
		Name:               r.FormValue("name"),
		Email:              r.FormValue("email"),
		PasswordHash:       r.FormValue("password"), // Kita pake key "password" di form
		TelegramID:         teleID,
		EmailParsingEnable: r.FormValue("email_parsing_enable") == "true",
	}

	// 3. Logika Avatar
	// 3. Logika Avatar
	file, header, err := r.FormFile("avatar")
	if err == nil {
		defer file.Close()

		// Pastiin folder ada (Safety check tambahan)
		if _, err := os.Stat("./uploads/avatar"); os.IsNotExist(err) {
			os.MkdirAll("./uploads/avatar", os.ModePerm)
		}

		fileName := time.Now().Format("20060102150405") + "-" + header.Filename
		filePath := "./uploads/avatar/" + fileName

		dst, err := os.Create(filePath)
		if err != nil {
			// Kalau gagal bikin file, kirim error ke Bruno, jangan diem aja!
			sendError(w, apperror.Internal("Gagal bikin file di server: "+err.Error()))
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			sendError(w, apperror.Internal("Gagal copy file: "+err.Error()))
			return
		}

		input.Avatar = fileName
	} else if err != http.ErrMissingFile {
		// Kalau error-nya bukan karena file kosong, tapi karena hal lain
		sendError(w, apperror.BadRequest("Error pas ngambil file: "+err.Error()))
		return
	}

	// 4. Gas ke Usecase
	if err := h.usecase.Register(&input); err != nil {
		sendError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Register Berhasil!"})
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

	// UseCase Login sekarang return 2 token sesuai request lo
	accessToken, refreshToken, err := h.usecase.Login(input.Email, input.Password)
	if err != nil {
		sendError(w, err)
		return
	}

	// A. Set Cookie buat Access Token (15 Menit)
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		HttpOnly: true,
		Path:     "/",
		Expires:  time.Now().Add(15 * time.Minute),
		SameSite: http.SameSiteLaxMode,
	})

	// B. Set Cookie buat Refresh Token (7 Hari)
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		HttpOnly: true,
		Path:     "/", // Biar cuma dikirim pas mau refresh doang
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

	// Ambil refresh_token dari cookie
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		sendError(w, apperror.Unauthorized("Sesi habis, login lagi Mi!"))
		return
	}

	// Panggil UseCase buat tuker refresh_token lama jadi access_token baru
	newAccessToken, err := h.usecase.RefreshToken(cookie.Value)
	if err != nil {
		sendError(w, err)
		return
	}

	// Set Access Token baru ke cookie
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

	// 1. Ambil token dari cookie buat di-blacklist/hapus di DB
	atCookie, errAT := r.Cookie("access_token")
	rtCookie, errRT := r.Cookie("refresh_token")

	// Kalau ada tokennya, panggil Usecase buat urusan database
	if errAT == nil && errRT == nil {
		// Panggil Logout Sultan (Hapus RT di DB & Blacklist AT)
		h.usecase.Logout(atCookie.Value, rtCookie.Value)
	}

	// 2. Kirim instruksi ke browser buat hapus cookie (Client-side)
	// Hapus Access Token
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})

	// Hapus Refresh Token
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
		"message": "Logout Berhasil! Session dihapus dan Token di-blacklist.",
	})
}

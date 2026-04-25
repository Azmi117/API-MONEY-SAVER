package usecase

import (
	"errors"
	"math/rand"
	"os"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/utils"
	"github.com/golang-jwt/jwt/v5"
)

type AuthUsecase interface {
	Register(user *models.User) error
	Login(email, password string) (string, string, error)
	RefreshToken(tokenString string) (string, error)
	Logout(accessToken string, refreshToken string) error
	randomString(length int) string
	RequestBindingCode(userID uint) (string, error)
	VerifyAndBindTelegram(telegramID int64, code string) error
}

type authUsecase struct {
	repo repository.AuthRepository
}

func NewAuthUsecase(params repository.AuthRepository) AuthUsecase {
	return &authUsecase{
		repo: params,
	}
}

func (u *authUsecase) Register(user *models.User) error {
	// 1. Cek Existing User
	existing, _ := u.repo.FindByEmail(user.Email)

	if existing != nil {
		return apperror.BadRequest("Name already taken by another user!")
	}

	// 2. Hash password dengan helper function
	hashedPass, err := utils.HashPassword(user.PasswordHash)

	// 3. Jika password gagal return err
	if err != nil {
		return err
	}

	// 4. Reinitialize password request body menjadi password hash
	user.PasswordHash = hashedPass

	// 5. Gunakan function query create user pada repo
	return u.repo.Create(user)
}

func (u *authUsecase) Login(email, password string) (string, string, error) {
	// 1. Cek Existing User
	existing, err := u.repo.FindByEmail(email)

	if err != nil {
		return "", "", apperror.NotFound("No registered user with this email!")
	}

	// 2. Verify password dengan helper function
	if err := utils.VerifyPassword(password, existing.PasswordHash); err != nil {
		return "", "", apperror.Unauthorized("Invalid credentials!")
	}

	jSecret := os.Getenv("JWT_SECRET")
	rSecret := os.Getenv("REFRESH_SECRET")

	// 3. Buat Access Token
	accessTokenClaims := jwt.MapClaims{
		"user_id": existing.ID,
		"exp":     time.Now().Add(time.Minute * 15).Unix(),
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessStr, err := accessToken.SignedString([]byte(jSecret))
	if err != nil {
		return "", "", err
	}

	// 4. Buat Refresh Token
	refreshTokenClaims := jwt.MapClaims{
		"user_id": existing.ID,
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshStr, err := refreshToken.SignedString([]byte(rSecret))
	if err != nil {
		return "", "", err
	}

	rt := models.RefreshToken{
		UserID:       existing.ID,
		RefreshToken: refreshStr, // Simpen string tokennya
		ExpiresAt:    time.Now().Add(time.Hour * 24 * 7),
	}

	// Panggil repo buat simpen ke table refresh_tokens
	if err := u.repo.CreateRefreshToken(&rt); err != nil {
		return "", "", apperror.Internal("Failed saving session to DB!")
	}

	return accessStr, refreshStr, nil
}

// --- INI METHOD BARU BIAR SYNC SAMA HANDLER ---

func (u *authUsecase) RefreshToken(tokenStr string) (string, error) {
	// 1. Validasi: Ada gak token ini di table refresh_tokens?
	// Ini gunanya table lo, Mi! Kalau hacker bawa token tapi di DB udah diapus (logout), dia gagal.
	rt, err := u.repo.GetRefreshToken(tokenStr)
	if err != nil {
		return "", apperror.Unauthorized("Session expired, please login again!")
	}

	// 2. Cek apakah tokennya sudah expired secara waktu di DB
	if time.Now().After(rt.ExpiresAt) {
		u.repo.DeleteRefreshToken(tokenStr) // Bersihin sampah di DB
		return "", apperror.Unauthorized("Session expired, please login again!")
	}

	// 3. Kalau aman, baru generate Access Token baru
	jSecret := os.Getenv("JWT_SECRET")
	claims := jwt.MapClaims{
		"user_id": rt.UserID,
		"exp":     time.Now().Add(time.Minute * 15).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jSecret))
}

func (u *authUsecase) Logout(accessToken string, refreshToken string) error {
	// 1. Hapus Refresh Token (Ini aman karena pake string token)
	_ = u.repo.DeleteRefreshToken(refreshToken)

	// 2. Bongkar Access Token buat dapet UserID
	// Kita perlu secret buat decode
	jSecret := os.Getenv("JWT_SECRET")
	token, _ := jwt.Parse(accessToken, func(t *jwt.Token) (interface{}, error) {
		return []byte(jSecret), nil
	})

	var uid uint
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		uid = uint(claims["user_id"].(float64))
	}

	// 3. Blacklist Access Token dengan UserID yang bener
	revoked := models.RevokeToken{
		Token:  accessToken,
		UserID: uid, // <--- INI KUNCINYA, BIAR GAK 0 LAGI
	}

	return u.repo.CreateRevokeToken(&revoked)
}

// randomString generates a random code for binding
func (u *authUsecase) randomString(length int) string {
	// Karakter yang jelas dibaca (nggak ada 0, O, 1, I, L)
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

	// Seed random biar kodenya gak itu-itu aja
	// (Note: Di Go terbaru seed otomatis, tapi buat amannya pake time)
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seed.Intn(len(charset))]
	}
	return string(b)
}

func (u *authUsecase) RequestBindingCode(userID uint) (string, error) {
	// 1. Generate Kode Unik (Bisa pake random string)
	code := "NSV-" + u.randomString(6)

	// 2. Set Expired (10 menit cukup lah ya)
	expiry := time.Now().Add(10 * time.Minute)

	// 3. Simpan ke Repo
	err := u.repo.SetBindingCode(userID, code, expiry)
	if err != nil {
		return "", err
	}

	return code, nil
}

func (u *authUsecase) VerifyAndBindTelegram(telegramID int64, code string) error {
	// 1. Cari user berdasarkan kode yang belum expired
	user, err := u.repo.FindByBindingCode(code)
	if err != nil {
		return errors.New("kodenya salah atau udah angus, Mi! Generate baru di Web")
	}

	// 2. VALIDASI: Cek apakah Telegram ID ini udah dipake akun lain
	// Kita nggak mau satu Telegram ID dipake buat login di 2 akun web berbeda
	existingUser, _ := u.repo.GetByTelegramID(telegramID)
	if existingUser != nil && existingUser.ID != user.ID {
		return errors.New("akun Telegram lu udah terhubung ke akun Nesav lain!")
	}

	// 3. FINALISASI: Ikat TelegramID dan hapus kodenya (set NULL)
	err = u.repo.FinalizeBinding(user.ID, telegramID)
	if err != nil {
		return errors.New("gagal nge-link akun, coba lagi nanti")
	}

	return nil
}

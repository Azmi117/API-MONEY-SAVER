package usecase

import (
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

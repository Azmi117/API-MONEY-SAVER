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
		"exp":     time.Now().Add(time.Minute * 3).Unix(),
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

	return accessStr, refreshStr, nil
}

// --- INI METHOD BARU BIAR SYNC SAMA HANDLER ---

func (u *authUsecase) RefreshToken(tokenString string) (string, error) {
	rSecret := os.Getenv("REFRESH_SECRET")
	jSecret := os.Getenv("JWT_SECRET")

	// 1. Bongkar & Validasi Refresh Token-nya
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte(rSecret), nil
	})

	if err != nil || !token.Valid {
		return "", apperror.Unauthorized("Refresh token invalid or expired!")
	}

	// 2. Ambil UserID dari claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", apperror.Internal("Failed to parse claims")
	}

	userID := claims["user_id"]

	// 3. Bikin Access Token baru (Umur 15 menit lagi)
	newAccessTokenClaims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Minute * 15).Unix(),
	}

	newAccessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, newAccessTokenClaims)
	return newAccessToken.SignedString([]byte(jSecret))
}

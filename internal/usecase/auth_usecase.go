package usecase

import (
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
	existing, _ := u.repo.FindByEmail(user.Email)

	if existing != nil {
		// Menggunakan Conflict (409) karena email sudah ada
		return apperror.Conflict("Email address is already registered")
	}

	hashedPass, err := utils.HashPassword(user.PasswordHash)
	if err != nil {
		return apperror.Internal("Failed to process password encryption")
	}

	user.PasswordHash = hashedPass
	return u.repo.Create(user)
}

func (u *authUsecase) Login(email, password string) (string, string, error) {
	existing, err := u.repo.FindByEmail(email)
	if err != nil {
		return "", "", apperror.NotFound("No account found with this email address")
	}

	if err := utils.VerifyPassword(password, existing.PasswordHash); err != nil {
		return "", "", apperror.Unauthorized("Invalid email or password")
	}

	jSecret := os.Getenv("JWT_SECRET")
	rSecret := os.Getenv("REFRESH_SECRET")

	accessTokenClaims := jwt.MapClaims{
		"user_id": existing.ID,
		"exp":     time.Now().Add(time.Minute * 15).Unix(),
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessStr, err := accessToken.SignedString([]byte(jSecret))
	if err != nil {
		return "", "", apperror.Internal("Could not generate access token")
	}

	refreshTokenClaims := jwt.MapClaims{
		"user_id": existing.ID,
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshStr, err := refreshToken.SignedString([]byte(rSecret))
	if err != nil {
		return "", "", apperror.Internal("Could not generate refresh token")
	}

	rt := models.RefreshToken{
		UserID:       existing.ID,
		RefreshToken: refreshStr,
		ExpiresAt:    time.Now().Add(time.Hour * 24 * 7),
	}

	if err := u.repo.CreateRefreshToken(&rt); err != nil {
		return "", "", apperror.Internal("Failed to save session data")
	}

	return accessStr, refreshStr, nil
}

func (u *authUsecase) RefreshToken(tokenStr string) (string, error) {
	rt, err := u.repo.GetRefreshToken(tokenStr)
	if err != nil {
		return "", apperror.Unauthorized("Invalid or expired session. Please log in again")
	}

	if time.Now().After(rt.ExpiresAt) {
		u.repo.DeleteRefreshToken(tokenStr)
		return "", apperror.Unauthorized("Session has expired. Please log in again")
	}

	jSecret := os.Getenv("JWT_SECRET")
	claims := jwt.MapClaims{
		"user_id": rt.UserID,
		"exp":     time.Now().Add(time.Minute * 15).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStrNew, err := token.SignedString([]byte(jSecret))
	if err != nil {
		return "", apperror.Internal("Failed to refresh access token")
	}

	return tokenStrNew, nil
}

func (u *authUsecase) Logout(accessToken string, refreshToken string) error {
	_ = u.repo.DeleteRefreshToken(refreshToken)

	jSecret := os.Getenv("JWT_SECRET")
	token, _ := jwt.Parse(accessToken, func(t *jwt.Token) (interface{}, error) {
		return []byte(jSecret), nil
	})

	var uid uint
	if token != nil {
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if val, ok := claims["user_id"].(float64); ok {
				uid = uint(val)
			}
		}
	}

	revoked := models.RevokeToken{
		Token:  accessToken,
		UserID: uid,
	}

	return u.repo.CreateRevokeToken(&revoked)
}

func (u *authUsecase) randomString(length int) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seed.Intn(len(charset))]
	}
	return string(b)
}

func (u *authUsecase) RequestBindingCode(userID uint) (string, error) {
	code := "NSV-" + u.randomString(6)
	expiry := time.Now().Add(10 * time.Minute)

	err := u.repo.SetBindingCode(userID, code, expiry)
	if err != nil {
		return "", apperror.Internal("Failed to generate binding code")
	}

	return code, nil
}

func (u *authUsecase) VerifyAndBindTelegram(telegramID int64, code string) error {
	user, err := u.repo.FindByBindingCode(code)
	if err != nil {
		return apperror.BadRequest("The binding code is invalid or has expired")
	}

	existingUser, _ := u.repo.GetByTelegramID(telegramID)
	if existingUser != nil && existingUser.ID != user.ID {
		return apperror.Conflict("This Telegram account is already linked to another user")
	}

	err = u.repo.FinalizeBinding(user.ID, telegramID)
	if err != nil {
		return apperror.Internal("Failed to link Telegram account. Please try again later")
	}

	return nil
}

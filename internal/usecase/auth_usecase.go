package usecase

import (
	"context"
	cryptoRand "crypto/rand"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"os"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/service"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/utils"
	"github.com/golang-jwt/jwt/v5"
)

type AuthUsecase interface {
	Register(user *models.User) error
	Login(email, password string) (*models.User, error)
	RefreshToken(tokenString string) (string, error)
	Logout(accessToken string, refreshToken string) error
	randomString(length int) string
	RequestBindingCode(userID uint) (string, error)
	VerifyAndBindTelegram(telegramID int64, code string) error
	SendOrResendOTP(userID uint, email string, otpType string) error
	VerifyRegisterOTP(email string, code string) error
	GetUserByEmail(email string) (*models.User, error)
	LoginSSO(email string, name string) (string, string, error)
	VerifyLoginOTP(email string, code string) (string, string, error)
	ForgotPasswordRequest(email string) error
	ResetPassword(email string, code string, newPassword string) error
	FindByID(ctx context.Context, id uint) (*models.User, error)
	UpdateProfile(userID uint, name string, avatar string) error
}

type authUsecase struct {
	repo              repository.AuthRepository
	otpRepo           repository.OTPRepository
	googleAuthService service.GoogleAuthService
}

func NewAuthUsecase(repo repository.AuthRepository, otpRepo repository.OTPRepository, gas service.GoogleAuthService) AuthUsecase {
	return &authUsecase{
		repo:              repo,
		otpRepo:           otpRepo,
		googleAuthService: gas,
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
	user.IsVerified = false // Pastiin default-nya false biar gak lolos login

	// 1. Simpan user ke tabel users dulu
	if err := u.repo.Create(user); err != nil {
		return err // Kalo gagal bikin user, langsung stop
	}

	// 2. KUNCI UTAMANYA DI SINI!
	// Setelah user kebuat (dapet user.ID), langsung cetak & kirim OTP
	err = u.SendOrResendOTP(user.ID, user.Email, "register")
	if err != nil {
		// Walaupun gagal ngirim OTP (misal email nyangkut), user udah terdaftar.
		// Dia tetep bisa pake endpoint /otp/resend nanti buat nyoba lagi.
		return err
	}

	return nil
}

func (u *authUsecase) FindByID(ctx context.Context, id uint) (*models.User, error) {
	user, err := u.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	// --- LOGIC PENGECEKAN TOKEN GOOGLE ---
	if user.GmailEnabled {
		// Periksa apakah token masih valid
		isValid := u.googleAuthService.CheckTokenValidity(ctx, user.GoogleRefreshToken)

		if !isValid {
			// Token sudah tidak valid, matikan GmailEnabled
			user.GmailEnabled = false
			user.GoogleRefreshToken = "" // Opsional: Kosongkan token yang sudah rusak

			// Simpan perubahan ke database
			_ = u.repo.Update(user)
		}
	}

	return user, nil
}

// 1. REFACTOR: Method Login hanya untuk cek password & kirim OTP Login
func (u *authUsecase) Login(email, password string) (*models.User, error) {
	existing, err := u.repo.FindByEmail(email)
	if err != nil {
		return nil, apperror.NotFound("No account found with this email address")
	}

	if err := utils.VerifyPassword(password, existing.PasswordHash); err != nil {
		return nil, apperror.Unauthorized("Invalid email or password")
	}

	// Cek status verifikasi pendaftaran akun
	if !existing.IsVerified {
		// Return error khusus yang nanti bakal ditangkep handler
		return nil, apperror.AccountNotVerified("Account not verified", existing.Email)
	}

	// Memicu pengiriman OTP khusus LOGIN
	err = u.SendOrResendOTP(existing.ID, existing.Email, "login")
	if err != nil {
		return nil, err
	}

	return existing, nil
}

// 2. NEW METHOD: Verifikasi OTP khusus login dan memproduksi JWT Tokens
func (u *authUsecase) VerifyLoginOTP(email string, code string) (string, string, error) {
	user, err := u.repo.FindByEmail(email)
	if err != nil {
		return "", "", apperror.NotFound("User not found.")
	}

	// Ambil OTP terakhir dengan tipe "login"
	latestOTP, _ := u.otpRepo.FindLatestByUserID(user.ID, "login")
	if latestOTP == nil || latestOTP.OTPCode != code {
		return "", "", apperror.BadRequest("Invalid OTP code.")
	}
	if time.Now().After(latestOTP.ExpiresAt) {
		return "", "", apperror.BadRequest("OTP code has expired.")
	}

	// OTP valid, hapus OTP-nya agar tidak bisa dipakai ulang
	_ = u.otpRepo.DeleteByUserIDAndType(user.ID, "login")

	// Pindahkan logic cetak JWT Token lu yang lama ke sini!
	jSecret := os.Getenv("JWT_SECRET")
	rSecret := os.Getenv("REFRESH_SECRET")

	accessTokenClaims := jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Minute * 15).Unix(),
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessStr, err := accessToken.SignedString([]byte(jSecret))
	if err != nil {
		return "", "", apperror.Internal("Could not generate access token")
	}

	refreshTokenClaims := jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Hour * 24 * 30).Unix(), // 30 Hari
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshStr, err := refreshToken.SignedString([]byte(rSecret))
	if err != nil {
		return "", "", apperror.Internal("Could not generate refresh token")
	}

	rt := models.RefreshToken{
		UserID:       user.ID,
		RefreshToken: refreshStr,
		ExpiresAt:    time.Now().Add(time.Hour * 24 * 30),
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

func generateOTP() string {
	max := big.NewInt(1000000)
	n, _ := cryptoRand.Int(cryptoRand.Reader, max)
	return fmt.Sprintf("%06d", n.Int64())
}

// METHOD UTAMA: Generate, Save, & Send Email
func (u *authUsecase) SendOrResendOTP(userID uint, email string, otpType string) error {
	// 1. Rate Limiter: Cek apakah baru minta kurang dari 1 menit yang lalu
	latestOTP, _ := u.otpRepo.FindLatestByUserID(userID, otpType)
	if latestOTP != nil {
		if time.Since(latestOTP.CreatedAt).Seconds() < 60 {
			return apperror.TooManyRequests("Please wait 1 minute before requesting a new OTP code.")
		}
	}

	// 2. Generate kode & Simpan ke DB (Expired 5 menit)
	code := generateOTP()
	otpData := &models.OTP{
		UserID:    userID,
		OTPCode:   code,
		Type:      otpType,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	if err := u.otpRepo.Create(otpData); err != nil {
		return apperror.Internal("Failed to generate OTP code.")
	}

	// 3. Kirim via Email (Pake Goroutine biar gak nge-block response ke user!)
	go func() {
		subject := "Your OTP Code - Money Saver"
		body := fmt.Sprintf("<h1>Your OTP Code is: <b>%s</b></h1><p>It will expire in 5 minutes.</p>", code)
		if err := utils.SendEmail(email, subject, body); err != nil {
			log.Printf("Failed to send OTP email to %s: %v", email, err)
		}
	}()

	return nil
}

// METHOD VERIFIKASI: Cek OTP valid/nggak
func (u *authUsecase) VerifyRegisterOTP(email string, code string) error {
	user, err := u.repo.FindByEmail(email)
	if err != nil {
		return apperror.NotFound("User not found.")
	}

	latestOTP, _ := u.otpRepo.FindLatestByUserID(user.ID, "register")
	if latestOTP == nil || latestOTP.OTPCode != code {
		return apperror.BadRequest("Invalid OTP code.")
	}
	if time.Now().After(latestOTP.ExpiresAt) {
		return apperror.BadRequest("OTP code has expired.")
	}

	// Lolos verifikasi! Ubah status user & buang OTP-nya
	_ = u.repo.UpdateVerificationStatus(user.ID, true) // Asumsi lu punya fungsi ini di authRepo
	_ = u.otpRepo.DeleteByUserIDAndType(user.ID, "register")

	return nil
}

func (u *authUsecase) GetUserByEmail(email string) (*models.User, error) {
	user, err := u.repo.FindByEmail(email)
	if err != nil {
		// Kalo error atau gak ketemu, balikin custom error 404 lu
		return nil, apperror.NotFound("User with the specified email address was not found.")
	}

	return user, nil
}

func (u *authUsecase) LoginSSO(email string, name string) (string, string, error) {
	existing, err := u.repo.FindByEmail(email)

	// 1. Kalo user belum ada, kita daftarin otomatis!
	if err != nil || existing == nil {
		newUser := &models.User{
			Email:        email,
			Name:         name, // <-- SEKARANG NAMANYA SUDAH MASUK DI SINI, GAK AKAN NULL LAGI!
			PasswordHash: "",   // Kosongin aja, dia gak bakal bisa login manual sampe dia bikin password
			IsVerified:   true, // Otomatis true karena Google udah ngeverifikasi emailnya
		}

		if errCreate := u.repo.Create(newUser); errCreate != nil {
			return "", "", apperror.Internal("Failed to create user via SSO")
		}
		existing = newUser
	} else if !existing.IsVerified {
		// 2. Kalo akunnya ada TAPI belum aktif (misal daftar manual tapi males masukin OTP)
		// Karena dia login via Google, otomatis kita verifikasiin aja.
		_ = u.repo.UpdateVerificationStatus(existing.ID, true)
		existing.IsVerified = true
	}

	// 3. GENERATE TOKEN (Sama persis kayak logic login biasa lu)
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
		"exp":     time.Now().Add(time.Hour * 24 * 30).Unix(),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshStr, err := refreshToken.SignedString([]byte(rSecret))
	if err != nil {
		return "", "", apperror.Internal("Could not generate refresh token")
	}

	rt := models.RefreshToken{
		UserID:       existing.ID,
		RefreshToken: refreshStr,
		ExpiresAt:    time.Now().Add(time.Hour * 24 * 30),
	}

	if err := u.repo.CreateRefreshToken(&rt); err != nil {
		return "", "", apperror.Internal("Failed to save session data")
	}

	return accessStr, refreshStr, nil
}

// 1. Minta OTP Lupa Password
func (u *authUsecase) ForgotPasswordRequest(email string) error {
	user, err := u.repo.FindByEmail(email)
	if err != nil {
		// Demi keamanan/privacy, tetep return nil atau kasih pesan generik biar gak di-scan hacker
		return apperror.NotFound("If the email is registered, an OTP code will be sent.")
	}

	// Kirim OTP khusus tipe "forgot_password"
	return u.SendOrResendOTP(user.ID, user.Email, "forgot_password")
}

// 2. Eksekusi Reset Password Baru (Validasi OTP langsung ganti password)
func (u *authUsecase) ResetPassword(email string, code string, newPassword string) error {
	user, err := u.repo.FindByEmail(email)
	if err != nil {
		return apperror.NotFound("User not found.")
	}

	// Validasi OTP tipe "forgot_password"
	latestOTP, _ := u.otpRepo.FindLatestByUserID(user.ID, "forgot_password")
	if latestOTP == nil || latestOTP.OTPCode != code {
		return apperror.BadRequest("Invalid OTP code.")
	}
	if time.Now().After(latestOTP.ExpiresAt) {
		return apperror.BadRequest("OTP code has expired.")
	}

	// Hash password baru lu pake utils bawaan repo lu
	hashedPassword, err := utils.HashPassword(newPassword)
	if err != nil {
		return apperror.Internal("Failed to process new password.")
	}

	// Update password di DB
	user.PasswordHash = hashedPassword
	if err := u.repo.Update(user); err != nil {
		return apperror.Internal("Failed to update password.")
	}

	// Hapus OTP biar gak bisa di-reuse
	_ = u.otpRepo.DeleteByUserIDAndType(user.ID, "forgot_password")

	return nil
}

func (u *authUsecase) UpdateProfile(userID uint, name string, avatar string) error {
	if name == "" {
		return apperror.BadRequest("Nama nggak boleh kosong")
	}

	return u.repo.UpdateProfile(userID, name, avatar)
}

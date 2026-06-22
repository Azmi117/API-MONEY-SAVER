package http

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/testutils"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/utils" // Required import for argon2 hashing
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/gmail/v1"
)

// Mock for GoogleAuthService to cover SSO success paths
type mockGoogleAuthService struct{}

func (m *mockGoogleAuthService) GetAuthURL(userID uint) string {
	return "https://mock-google-auth-url.com"
}

func (m *mockGoogleAuthService) ExchangeCode(ctx context.Context, userID uint, code string) error {
	if code == "mock-error" {
		return context.DeadlineExceeded // Just a generic error to trigger the SendError block
	}
	return nil
}

func (m *mockGoogleAuthService) CheckTokenValidity(ctx context.Context, refreshToken string) bool {
	return true
}

// Added missing method to strictly satisfy the GoogleAuthService interface
func (m *mockGoogleAuthService) GetGmailService(refreshToken string) (*gmail.Service, error) {
	return nil, nil // We don't need real Gmail functionality for Auth Handler tests
}

func TestIntegration_AuthAPI(t *testing.T) {
	// 1. SETUP TEST DATABASE
	db := testutils.SetupTestDB()
	defer testutils.CleanTestDB(db)

	// 2. SETUP DEPENDENCY INJECTION
	authRepo := repository.NewAuthRepository(db)
	otpRepo := repository.NewOTPRepository(db)
	googleService := &mockGoogleAuthService{}

	authUsecase := usecase.NewAuthUsecase(authRepo, otpRepo, googleService)
	authHandler := NewAuthHandler(authUsecase, googleService)

	resetDB := func() {
		testutils.CleanTestDB(db)
	}

	// Generate an authentic argon2 hash to pass the VerifyPassword check in the Usecase
	hashedPass, _ := utils.HashPassword("securepassword123")

	// ==========================================
	// SCENARIO 1: REGISTER (MULTIPART)
	// ==========================================
	t.Run("Should successfully register a new user using multipart form", func(t *testing.T) {
		resetDB()

		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		writer.WriteField("name", "Azmi Register")
		writer.WriteField("email", "azmi.register@test.com")
		writer.WriteField("password", "securepassword123")
		writer.WriteField("telegram_id", "123456789")
		writer.Close() // The writer must be closed before sending the request

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		w := httptest.NewRecorder()
		authHandler.Register(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	// ==========================================
	// SCENARIO 2: LOGIN (PASSWORD CHECK & OTP INITIATION)
	// ==========================================
	t.Run("Should accept valid login credentials and initiate OTP", func(t *testing.T) {
		resetDB()

		user := models.User{
			Name:         "Login User",
			Email:        "login@test.com",
			PasswordHash: hashedPass, // MUST USE ARGON2 HASH
			IsVerified:   true,       // MUST BE TRUE TO AVOID 403 FORBIDDEN
		}
		db.Create(&user)

		payload := `{"email": "login@test.com", "password": "securepassword123"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		authHandler.Login(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
	})

	// ==========================================
	// SCENARIO 3: VERIFY LOGIN (EXCHANGE OTP FOR JWT)
	// ==========================================
	t.Run("Should verify login OTP and set JWT cookies", func(t *testing.T) {
		resetDB()

		user := models.User{Email: "verify@test.com", PasswordHash: hashedPass, IsVerified: true}
		db.Create(&user)

		// Seed OTP data into the database
		otp := models.OTP{UserID: user.ID, OTPCode: "123456", Type: "login", ExpiresAt: time.Now().Add(5 * time.Minute)}
		db.Create(&otp)

		payload := `{"email": "verify@test.com", "code": "123456"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login/verify", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		authHandler.VerifyLogin(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.GreaterOrEqual(t, len(w.Result().Cookies()), 2, "Expected access and refresh tokens in cookies")
	})

	// ==========================================
	// SCENARIO 4: VERIFY REGISTER
	// ==========================================
	t.Run("Should verify registration OTP", func(t *testing.T) {
		resetDB()

		user := models.User{Email: "reg.verify@test.com", PasswordHash: hashedPass, IsVerified: false}
		db.Create(&user)

		otp := models.OTP{UserID: user.ID, OTPCode: "654321", Type: "register", ExpiresAt: time.Now().Add(5 * time.Minute)}
		db.Create(&otp)

		payload := `{"email": "reg.verify@test.com", "code": "654321"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register/verify", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		authHandler.VerifyRegister(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// ==========================================
	// SCENARIO 5: RESEND OTP
	// ==========================================
	t.Run("Should successfully resend OTP", func(t *testing.T) {
		resetDB()
		user := models.User{Email: "resend@test.com"}
		db.Create(&user)

		payload := `{"email": "resend@test.com", "type": "login"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/resend", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		authHandler.ResendOTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// ==========================================
	// SCENARIO 6: FORGOT PASSWORD REQUEST
	// ==========================================
	t.Run("Should trigger forgot password", func(t *testing.T) {
		resetDB()
		user := models.User{Email: "forgot@test.com"}
		db.Create(&user)

		payload := `{"email": "forgot@test.com"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		authHandler.ForgotPasswordRequest(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// ==========================================
	// SCENARIO 7: RESET PASSWORD (VERIFY)
	// ==========================================
	t.Run("Should verify forgot password OTP and update password", func(t *testing.T) {
		resetDB()
		user := models.User{Email: "reset@test.com", PasswordHash: "oldhash"}
		db.Create(&user)

		otp := models.OTP{UserID: user.ID, OTPCode: "111222", Type: "forgot_password", ExpiresAt: time.Now().Add(5 * time.Minute)}
		db.Create(&otp)

		payload := `{"email": "reset@test.com", "code": "111222", "new_password": "newsecurepassword"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password/verify", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		authHandler.ResetPassword(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// ==========================================
	// SCENARIO 8: REFRESH TOKEN
	// ==========================================
	t.Run("Should refresh token successfully", func(t *testing.T) {
		resetDB()
		user := models.User{Email: "refresh@test.com"}
		db.Create(&user)

		rt := models.RefreshToken{UserID: user.ID, RefreshToken: "valid_refresh_token", ExpiresAt: time.Now().Add(24 * time.Hour)}
		db.Create(&rt)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "valid_refresh_token"})

		w := httptest.NewRecorder()
		authHandler.Refresh(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// ==========================================
	// SCENARIO 9: LOGOUT
	// ==========================================
	t.Run("Should logout and clear cookies", func(t *testing.T) {
		resetDB()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: "access"})
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "refresh"})
		w := httptest.NewRecorder()
		authHandler.Logout(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// ==========================================
	// SCENARIO 10: GET PROFILE
	// ==========================================
	t.Run("Should retrieve profile", func(t *testing.T) {
		resetDB()
		user := models.User{Email: "profile@test.com"}
		db.Create(&user)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/profile", nil)
		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		authHandler.GetProfile(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// ==========================================
	// SCENARIO 11: UPDATE PROFILE (MULTIPART)
	// ==========================================
	t.Run("Should update profile name", func(t *testing.T) {
		resetDB()
		user := models.User{Email: "update@test.com", Name: "Old Name"}
		db.Create(&user)

		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		writer.WriteField("name", "New Name")
		writer.Close()

		req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		authHandler.UpdateProfile(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// ==========================================
	// SCENARIO 12: GET BINDING CODE
	// ==========================================
	t.Run("Should get binding code", func(t *testing.T) {
		resetDB()
		user := models.User{Email: "binding@test.com"}
		db.Create(&user)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/telegram/binding-code", nil)
		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		authHandler.GetBindingCode(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// ==========================================
	// SCENARIO 13: SAD PATHS & COVERAGE BOOSTERS
	// ==========================================
	t.Run("Coverage Boost: Invalid Payloads, Wrong Methods, and SSO Errors", func(t *testing.T) {
		// 1. VerifyLogin - Wrong Method (GET instead of POST)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/login/verify", nil)
		w := httptest.NewRecorder()
		authHandler.VerifyLogin(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

		// 2. VerifyLogin - Bad JSON
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login/verify", strings.NewReader("{ bad_json"))
		w = httptest.NewRecorder()
		authHandler.VerifyLogin(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 3. VerifyRegister - Wrong Method
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/register/verify", nil)
		w = httptest.NewRecorder()
		authHandler.VerifyRegister(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

		// 4. ResendOTP - Wrong Method
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/otp/resend", nil)
		w = httptest.NewRecorder()
		authHandler.ResendOTP(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

		// 5. ForgotPassword - Wrong Method
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/forgot-password", nil)
		w = httptest.NewRecorder()
		authHandler.ForgotPasswordRequest(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

		// 6. ResetPassword - Wrong Method & Bad JSON
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/forgot-password/verify", nil)
		w = httptest.NewRecorder()
		authHandler.ResetPassword(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password/verify", strings.NewReader("{ bad_json"))
		w = httptest.NewRecorder()
		authHandler.ResetPassword(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 7. UpdateProfile - Wrong Method
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/profile", nil)
		w = httptest.NewRecorder()
		authHandler.UpdateProfile(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

		// 8. Google Callback - Missing Code (Trigger BadRequest)
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback", nil)
		w = httptest.NewRecorder()
		authHandler.GoogleCallback(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 9. Google SSO Callback - Missing State (Trigger BadRequest)
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/google/callback", nil)
		w = httptest.NewRecorder()
		authHandler.GoogleSSOCallback(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	// ==========================================
	// SCENARIO 14: DEEP VALIDATION & USECASE ERRORS
	// ==========================================
	t.Run("Coverage Boost: Deep Validations and Usecase Errors", func(t *testing.T) {
		resetDB()

		// Setup base user for tests
		hashedPass, _ := utils.HashPassword("password123")
		verifiedUser := models.User{Email: "verified@test.com", PasswordHash: hashedPass, IsVerified: true}
		unverifiedUser := models.User{Email: "unverified@test.com", PasswordHash: hashedPass, IsVerified: false}
		db.Create(&verifiedUser)
		db.Create(&unverifiedUser)

		// 1. Register - Duplicate Email (Expect Conflict 409)
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		writer.WriteField("name", "Duplicate User")
		writer.WriteField("email", "verified@test.com") // Intentionally duplicated email
		writer.WriteField("password", "password123")
		writer.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		authHandler.Register(w, req)
		assert.Equal(t, http.StatusConflict, w.Code)

		// 2. Login - Wrong Password (Expect Unauthorized 401)
		payload := `{"email": "verified@test.com", "password": "wrongpassword"}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		authHandler.Login(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// 3. Login - Unverified Account (Triggers Custom Forbidden Block)
		payload = `{"email": "unverified@test.com", "password": "password123"}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		authHandler.Login(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)

		// 4. VerifyLogin - Wrong OTP
		payload = `{"email": "verified@test.com", "code": "999999"}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login/verify", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		authHandler.VerifyLogin(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 5. ResetPassword - Password too short (< 6 chars)
		payload = `{"email": "verified@test.com", "code": "123456", "new_password": "123"}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password/verify", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		authHandler.ResetPassword(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 6. UpdateProfile - Empty Name
		body = new(bytes.Buffer)
		writer = multipart.NewWriter(body)
		writer.WriteField("name", "") // Intentionally left blank
		writer.Close()
		req = httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		ctx := context.WithValue(req.Context(), "user_id", verifiedUser.ID)
		req = req.WithContext(ctx)
		w = httptest.NewRecorder()
		authHandler.UpdateProfile(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 7. ResendOTP - User Not Found
		payload = `{"email": "ghost@test.com", "type": "login"}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/resend", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		authHandler.ResendOTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	// ==========================================
	// SCENARIO 15: GOOGLE SSO ENDPOINTS VALIDATION
	// ==========================================
	t.Run("Coverage Boost: Google SSO Validations", func(t *testing.T) {
		// 1. GoogleSSOLogin - Should Redirect to Google Consent Page
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/google/login", nil)
		w := httptest.NewRecorder()
		authHandler.GoogleSSOLogin(w, req)
		assert.Equal(t, http.StatusTemporaryRedirect, w.Code)

		// 2. GoogleSSOCallback - Missing State
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/google/callback", nil)
		w = httptest.NewRecorder()
		authHandler.GoogleSSOCallback(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 3. GoogleSSOCallback - Invalid State
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/google/callback?state=wrong", nil)
		w = httptest.NewRecorder()
		authHandler.GoogleSSOCallback(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 4. GoogleSSOCallback - Missing Code
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/google/callback?state=sso-state-random-string", nil)
		w = httptest.NewRecorder()
		authHandler.GoogleSSOCallback(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 5. GoogleLogin (Axios Route) - Unauthorized (No context injected)
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/login", nil)
		w = httptest.NewRecorder()
		authHandler.GoogleLogin(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// 6. GoogleCallback (Axios Route) - Missing Code
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?state=123", nil)
		w = httptest.NewRecorder()
		authHandler.GoogleCallback(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 7. GoogleCallback (Axios Route) - Invalid State Format (Not integer)
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?code=abc&state=abc", nil)
		w = httptest.NewRecorder()
		authHandler.GoogleCallback(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	// ==========================================
	// SCENARIO 16: FILE UPLOADS (AVATAR)
	// ==========================================
	t.Run("Coverage Boost: Register and Update Profile with Avatar", func(t *testing.T) {
		resetDB()

		// 1. Register with Avatar Image
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		writer.WriteField("name", "Avatar User")
		writer.WriteField("email", "avatar@test.com")
		writer.WriteField("password", "securepassword123")

		// Simulate file upload
		part, _ := writer.CreateFormFile("avatar", "dummy.jpg")
		part.Write([]byte("fake image content"))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		w := httptest.NewRecorder()
		authHandler.Register(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		// 2. Setup user for Update Profile
		user := models.User{Email: "update.avatar@test.com", Name: "Old"}
		db.Create(&user)

		// 3. Update Profile with Avatar Image
		body2 := new(bytes.Buffer)
		writer2 := multipart.NewWriter(body2)
		writer2.WriteField("name", "New Name")

		part2, _ := writer2.CreateFormFile("avatar", "new_dummy.jpg")
		part2.Write([]byte("fake image content 2"))
		writer2.Close()

		req2 := httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", body2)
		req2.Header.Set("Content-Type", writer2.FormDataContentType())

		ctx := context.WithValue(req2.Context(), "user_id", user.ID)
		req2 = req2.WithContext(ctx)

		w2 := httptest.NewRecorder()
		authHandler.UpdateProfile(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})

	// ==========================================
	// SCENARIO 17: THE FINAL BOSS (SSO EXCHANGES & DB FAILURES)
	// ==========================================
	t.Run("Coverage Boost: SSO Network Fails & DB Crash Simulation", func(t *testing.T) {
		// 1. Google SSO Callback - Invalid Code (Triggers Exchange Error)
		// By passing a complete request, we force the handler to hit the real Google OAuth2 cfg.Exchange
		// Because the code is fake, it will fail and execute the SendError block.
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/google/callback?state=sso-state-random-string&code=fake-auth-code", nil)
		w := httptest.NewRecorder()
		authHandler.GoogleSSOCallback(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)

		// 2. CRASH THE DATABASE TO TRIGGER ALL REMAINING SendError(w, err) BLOCKS
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close() // Boom!
		}

		ctx := context.WithValue(context.Background(), "user_id", uint(999))

		// GetProfile - DB Down
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/profile", nil)
		req = req.WithContext(ctx)
		w = httptest.NewRecorder()
		authHandler.GetProfile(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)

		// GetBindingCode - DB Down
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/telegram/binding-code", nil)
		req = req.WithContext(ctx)
		w = httptest.NewRecorder()
		authHandler.GetBindingCode(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)

		// ForgotPassword - DB Down
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", strings.NewReader(`{"email":"test@test.com"}`))
		w = httptest.NewRecorder()
		authHandler.ForgotPasswordRequest(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)

		// VerifyRegister - DB Down
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register/verify", strings.NewReader(`{"email":"test@test.com", "code":"123"}`))
		w = httptest.NewRecorder()
		authHandler.VerifyRegister(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)
	})
	// ==========================================
	// SCENARIO 18: GOOGLE SSO MOCK SUCCESS & MISSING CONTEXTS
	// ==========================================
	t.Run("Coverage Boost: Google SSO Success and Missing Contexts", func(t *testing.T) {
		resetDB()
		user := models.User{Email: "sso@test.com"}
		db.Create(&user)

		// 1. GoogleLogin (Axios Route) - Success Path
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/login", nil)
		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		authHandler.GoogleLogin(w, req)
		assert.Equal(t, http.StatusOK, w.Code) // Now it will successfully return the mock URL

		// 2. GoogleCallback (Axios Route) - Success Path
		// State must be a parsable integer (userID) to pass Sscanf
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?state=1&code=valid-code", nil)
		w = httptest.NewRecorder()
		authHandler.GoogleCallback(w, req)
		assert.Equal(t, http.StatusTemporaryRedirect, w.Code) // Successful exchange redirects to profile

		// 3. GoogleCallback (Axios Route) - Exchange Error Path
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/callback?state=1&code=mock-error", nil)
		w = httptest.NewRecorder()
		authHandler.GoogleCallback(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code) // Expected to hit SendError

		// 4. UpdateProfile - Unauthorized (Missing Context)
		req = httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", nil)
		w = httptest.NewRecorder()
		authHandler.UpdateProfile(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// 5. GetBindingCode - Unauthorized (Missing Context)
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/telegram/binding-code", nil)
		w = httptest.NewRecorder()
		authHandler.GetBindingCode(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
	// ==========================================
	// SCENARIO 19: UNCOVERED ERROR BLOCKS (JSON DECODE, MISSING COOKIE, PARSE ERRORS)
	// ==========================================
	t.Run("Coverage Boost: JSON Decode, Missing Cookies, and Parse Errors", func(t *testing.T) {
		// 1. Register - Bad Multipart Form
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader("bad body"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
		w := httptest.NewRecorder()
		authHandler.Register(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 2. Login - Bad JSON
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("{ bad json }"))
		w = httptest.NewRecorder()
		authHandler.Login(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 3. Refresh - Missing Cookie
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		w = httptest.NewRecorder()
		authHandler.Refresh(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// 4. VerifyRegister - Bad JSON
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register/verify", strings.NewReader("{ bad json }"))
		w = httptest.NewRecorder()
		authHandler.VerifyRegister(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 5. ResendOTP - Bad JSON
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/resend", strings.NewReader("{ bad json }"))
		w = httptest.NewRecorder()
		authHandler.ResendOTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 6. ForgotPasswordRequest - Bad JSON
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", strings.NewReader("{ bad json }"))
		w = httptest.NewRecorder()
		authHandler.ForgotPasswordRequest(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 7. GetProfile - Missing Context
		req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/profile", nil)
		w = httptest.NewRecorder()
		authHandler.GetProfile(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// 8. UpdateProfile - Bad Multipart Form
		req = httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", strings.NewReader("bad body"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
		ctx := context.WithValue(req.Context(), "user_id", uint(1))
		req = req.WithContext(ctx)
		w = httptest.NewRecorder()
		authHandler.UpdateProfile(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	// ==========================================
	// SCENARIO 20: OS FILE ERRORS & REMAINING USECASE ERRORS
	// ==========================================
	t.Run("Coverage Boost: OS File Errors and Final Usecase Errors", func(t *testing.T) {
		resetDB()
		user := models.User{Email: "target@test.com", Name: "Target", IsVerified: true}
		db.Create(&user)

		// 1. Refresh - Token not found in DB
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "ghost_token"})
		w := httptest.NewRecorder()
		authHandler.Refresh(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)

		// 2. ResetPassword - Wrong OTP Code
		payload := `{"email": "target@test.com", "code": "wrongcode", "new_password": "validpassword"}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password/verify", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		authHandler.ResetPassword(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)

		// 3. Force OS.Create Error by creating a FILE where a DIRECTORY should be
		os.MkdirAll("./uploads", os.ModePerm)

		// Setup blocker for Register (uses ./uploads/avatar)
		os.RemoveAll("./uploads/avatar")
		os.WriteFile("./uploads/avatar", []byte("blocker"), 0644) // Creating a file named "avatar"

		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		writer.WriteField("name", "Avatar User")
		writer.WriteField("email", "avatar2@test.com")
		writer.WriteField("password", "securepassword123")
		part, _ := writer.CreateFormFile("avatar", "dummy.jpg")
		part.Write([]byte("fake image content"))
		writer.Close()

		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w = httptest.NewRecorder()
		authHandler.Register(w, req)
		assert.NotEqual(t, http.StatusCreated, w.Code)

		os.Remove("./uploads/avatar") // Cleanup blocker

		// Setup blocker for UpdateProfile (uses ./uploads/avatars)
		os.RemoveAll("./uploads/avatars")
		os.WriteFile("./uploads/avatars", []byte("blocker"), 0644) // Creating a file named "avatars"

		body2 := new(bytes.Buffer)
		writer2 := multipart.NewWriter(body2)
		writer2.WriteField("name", "New Name")
		part2, _ := writer2.CreateFormFile("avatar", "new_dummy.jpg")
		part2.Write([]byte("fake image content 2"))
		writer2.Close()

		req2 := httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", body2)
		req2.Header.Set("Content-Type", writer2.FormDataContentType())
		ctx := context.WithValue(req2.Context(), "user_id", user.ID)
		req2 = req2.WithContext(ctx)
		w2 := httptest.NewRecorder()
		authHandler.UpdateProfile(w2, req2)
		assert.NotEqual(t, http.StatusOK, w2.Code)

		os.Remove("./uploads/avatars") // Cleanup blocker

		// 4. Force Usecase errors via DB Crash
		sqlDB, _ := db.DB()
		sqlDB.Close()

		// ResendOTP - DB Error
		payload = `{"email": "target@test.com", "type": "login"}`
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/resend", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		authHandler.ResendOTP(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)

		// UpdateProfile - DB Error (Without file upload)
		body3 := new(bytes.Buffer)
		writer3 := multipart.NewWriter(body3)
		writer3.WriteField("name", "Crash Name")
		writer3.Close()
		req3 := httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", body3)
		req3.Header.Set("Content-Type", writer3.FormDataContentType())
		req3 = req3.WithContext(ctx)
		w3 := httptest.NewRecorder()
		authHandler.UpdateProfile(w3, req3)
		assert.NotEqual(t, http.StatusOK, w3.Code)
	})
}

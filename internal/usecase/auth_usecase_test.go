package usecase

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/service"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/utils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuthRepository is a mock implementation of the AuthRepository interface.
// It is used to isolate the usecase logic from the actual database operations.
type MockAuthRepository struct {
	mock.Mock
	repository.AuthRepository // Fix: Embedding the interface automatically satisfies all other missing methods like Create, FindByEmail, etc.
}

func (m *MockAuthRepository) ResetOCRUsage(userID uint, resetTime time.Time) error {
	args := m.Called(userID, resetTime)
	return args.Error(0)
}

func (m *MockAuthRepository) IncrementOCRUsage(userID uint) error {
	args := m.Called(userID)
	return args.Error(0)
}

func (m *MockAuthRepository) FindByID(id uint) (*models.User, error) {
	args := m.Called(id)
	if args.Get(0) != nil {
		return args.Get(0).(*models.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuthRepository) SetBindingCode(userID uint, code string, expiry time.Time) error {
	args := m.Called(userID, code, expiry)
	return args.Error(0)
}

func (m *MockAuthRepository) FindByBindingCode(code string) (*models.User, error) {
	args := m.Called(code)
	if args.Get(0) != nil {
		return args.Get(0).(*models.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuthRepository) GetByTelegramID(telegramID int64) (*models.User, error) {
	args := m.Called(telegramID)
	if args.Get(0) != nil {
		return args.Get(0).(*models.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuthRepository) FinalizeBinding(userID uint, telegramID int64) error {
	args := m.Called(userID, telegramID)
	return args.Error(0)
}

func (m *MockAuthRepository) UpdateProfile(userID uint, name string, avatar string) error {
	// m.Called records the arguments passed by the usecase
	args := m.Called(userID, name, avatar)

	// Returns the error defined in the specific test scenario
	return args.Error(0)
}

func (m *MockAuthRepository) FindByEmail(email string) (*models.User, error) {
	args := m.Called(email)
	if args.Get(0) != nil {
		return args.Get(0).(*models.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuthRepository) Create(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockAuthRepository) CreateRefreshToken(rt *models.RefreshToken) error {
	args := m.Called(rt)
	return args.Error(0)
}

// Tambahkan di dalam area MOCK REPOSITORIES, di bawah method Create milik MockOTPRepository

func (m *MockAuthRepository) UpdateVerificationStatus(userID uint, status bool) error {
	args := m.Called(userID, status)
	return args.Error(0)
}

func (m *MockAuthRepository) Update(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockAuthRepository) GetRefreshToken(token string) (*models.RefreshToken, error) {
	args := m.Called(token)
	if args.Get(0) != nil {
		return args.Get(0).(*models.RefreshToken), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuthRepository) DeleteRefreshToken(token string) error {
	args := m.Called(token)
	return args.Error(0)
}

func (m *MockAuthRepository) CreateRevokeToken(rt *models.RevokeToken) error {
	args := m.Called(rt)
	return args.Error(0)
}

func (m *MockAuthRepository) UpdateTier(userID uint, newTier string) error {
	args := m.Called(userID, newTier)
	return args.Error(0)
}

type MockOTPRepository struct {
	mock.Mock
	repository.OTPRepository
}

func (m *MockOTPRepository) FindLatestByUserID(userID uint, otpType string) (*models.OTP, error) {
	args := m.Called(userID, otpType)
	if args.Get(0) != nil {
		return args.Get(0).(*models.OTP), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockOTPRepository) Create(otp *models.OTP) error {
	args := m.Called(otp)
	return args.Error(0)
}

func (m *MockOTPRepository) DeleteByUserIDAndType(userID uint, otpType string) error {
	args := m.Called(userID, otpType)
	return args.Error(0)
}

type MockGoogleAuthService struct {
	mock.Mock
	service.GoogleAuthService
}

func (m *MockGoogleAuthService) CheckTokenValidity(ctx context.Context, token string) bool {
	args := m.Called(ctx, token)
	return args.Bool(0)
}

// UpdateProfile mocks the repository method to simulate database interactions.

func TestUpdateProfile(t *testing.T) {

	t.Run("Should return error when name is empty", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockAuthRepository)

		// The OTP repository and Google Auth Service are passed as nil
		// because they are not invoked within the UpdateProfile method.
		authUC := NewAuthUsecase(mockRepo, nil, nil)

		// Act
		err := authUC.UpdateProfile(1, "", "avatar.png")

		// Assert
		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("Should return error when repository fails", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockAuthRepository)

		// Simulate a database failure or connection timeout
		expectedError := errors.New("database connection timeout")

		// Instruct the mock to return the expectedError when called with these exact parameters
		mockRepo.On("UpdateProfile", uint(1), "Azmi", "avatar.png").Return(expectedError)

		authUC := NewAuthUsecase(mockRepo, nil, nil)

		// Act
		err := authUC.UpdateProfile(1, "Azmi", "avatar.png")

		// Assert
		assert.NotNil(t, err)
		assert.Equal(t, expectedError, err)

		// Verify that the mocked method was actually called during the execution
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should return nil on successful profile update", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockAuthRepository)

		// Instruct the mock to return nil (success) when called with these exact parameters
		mockRepo.On("UpdateProfile", uint(1), "Azmi", "avatar.png").Return(nil)

		authUC := NewAuthUsecase(mockRepo, nil, nil)

		// Act
		err := authUC.UpdateProfile(1, "Azmi", "avatar.png")

		// Assert
		assert.Nil(t, err)

		// Verify that the mocked method was actually called during the execution
		mockRepo.AssertExpectations(t)
	})
}

func TestRegister(t *testing.T) {

	t.Run("Should return conflict error when email already exists", func(t *testing.T) {
		// Arrange
		mockAuthRepo := new(MockAuthRepository)

		existingUser := &models.User{Email: "existing@example.com"}
		// Simulate the scenario where the email is already found in the database
		mockAuthRepo.On("FindByEmail", "existing@example.com").Return(existingUser, nil)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		newUser := &models.User{Email: "existing@example.com", PasswordHash: "password123"}

		// Act
		err := authUC.Register(newUser)

		// Assert
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "already registered")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return internal error when repository fails to create user", func(t *testing.T) {
		// Arrange
		mockAuthRepo := new(MockAuthRepository)

		// Simulate email not found (safe to register)
		mockAuthRepo.On("FindByEmail", "new@example.com").Return(nil, errors.New("not found"))

		// Simulate database failure during user creation
		dbError := errors.New("database connection lost")
		mockAuthRepo.On("Create", mock.AnythingOfType("*models.User")).Return(dbError)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		newUser := &models.User{Email: "new@example.com", PasswordHash: "password123"}

		// Act
		err := authUC.Register(newUser)

		// Assert
		assert.NotNil(t, err)
		assert.Equal(t, dbError, err)

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return error when OTP generation fails", func(t *testing.T) {
		// Arrange
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		mockAuthRepo.On("FindByEmail", "new@example.com").Return(nil, errors.New("not found"))

		// Simulate successful user creation and inject a mock ID
		mockAuthRepo.On("Create", mock.AnythingOfType("*models.User")).Return(nil).Run(func(args mock.Arguments) {
			user := args.Get(0).(*models.User)
			user.ID = 99 // Simulate auto-increment ID from DB
		})

		// OTP Rate Limiter check (simulate no recent OTP requests)
		mockOTPRepo.On("FindLatestByUserID", uint(99), "register").Return(nil, errors.New("not found"))

		// Simulate database failure when trying to save the generated OTP
		otpDbError := errors.New("failed to save OTP")
		mockOTPRepo.On("Create", mock.AnythingOfType("*models.OTP")).Return(otpDbError)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		newUser := &models.User{Email: "new@example.com", PasswordHash: "password123"}

		// Act
		err := authUC.Register(newUser)

		// Assert
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Failed to generate OTP code")

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})

	t.Run("Should return nil on successful registration", func(t *testing.T) {
		// Arrange
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		mockAuthRepo.On("FindByEmail", "new@example.com").Return(nil, errors.New("not found"))
		mockAuthRepo.On("Create", mock.AnythingOfType("*models.User")).Return(nil).Run(func(args mock.Arguments) {
			user := args.Get(0).(*models.User)
			user.ID = 100
		})

		mockOTPRepo.On("FindLatestByUserID", uint(100), "register").Return(nil, errors.New("not found"))
		mockOTPRepo.On("Create", mock.AnythingOfType("*models.OTP")).Return(nil)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		newUser := &models.User{Email: "new@example.com", PasswordHash: "password123"}

		// Act
		err := authUC.Register(newUser)

		// Assert
		assert.Nil(t, err)

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})
}

func TestLogin(t *testing.T) {
	// Setup: Generate a valid bcrypt hash to simulate a real password in the database
	validPassword := "securepassword123"
	hashedPassword, err := utils.HashPassword(validPassword)
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	t.Run("Should return not found error when email does not exist", func(t *testing.T) {
		// Arrange
		mockAuthRepo := new(MockAuthRepository)

		// Simulate email not found in the database
		mockAuthRepo.On("FindByEmail", "unknown@example.com").Return(nil, errors.New("not found"))

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		// Act
		user, err := authUC.Login("unknown@example.com", validPassword)

		// Assert
		assert.NotNil(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "No account found")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return unauthorized error when password does not match", func(t *testing.T) {
		// Arrange
		mockAuthRepo := new(MockAuthRepository)

		existingUser := &models.User{Email: "test@example.com", PasswordHash: hashedPassword}
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(existingUser, nil)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		// Act - Provide a wrong password
		user, err := authUC.Login("test@example.com", "wrongpassword456")

		// Assert
		assert.NotNil(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "Invalid email or password")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return account not verified error when user is unverified", func(t *testing.T) {
		// Arrange
		mockAuthRepo := new(MockAuthRepository)

		// Simulate a user whose IsVerified flag is false
		existingUser := &models.User{Email: "test@example.com", PasswordHash: hashedPassword, IsVerified: false}
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(existingUser, nil)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		// Act
		user, err := authUC.Login("test@example.com", validPassword)

		// Assert
		assert.NotNil(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "Account not verified")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return error when OTP generation fails during login", func(t *testing.T) {
		// Arrange
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		// Fix: Assign the ID after the struct initialization due to gorm.Model embedding
		existingUser := &models.User{Email: "test@example.com", PasswordHash: hashedPassword, IsVerified: true}
		existingUser.ID = 1

		mockAuthRepo.On("FindByEmail", "test@example.com").Return(existingUser, nil)

		// Simulate OTP Rate Limiter check
		mockOTPRepo.On("FindLatestByUserID", uint(1), "login").Return(nil, errors.New("not found"))

		// Simulate database failure when trying to save the generated login OTP
		otpDbError := errors.New("failed to save OTP")
		mockOTPRepo.On("Create", mock.AnythingOfType("*models.OTP")).Return(otpDbError)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		// Act
		user, err := authUC.Login("test@example.com", validPassword)

		// Assert
		assert.NotNil(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "Failed to generate OTP code")

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})

	t.Run("Should return user on successful login request", func(t *testing.T) {
		// Arrange
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		// Fix: Assign the ID after the struct initialization due to gorm.Model embedding
		existingUser := &models.User{Email: "test@example.com", PasswordHash: hashedPassword, IsVerified: true}
		existingUser.ID = 1

		mockAuthRepo.On("FindByEmail", "test@example.com").Return(existingUser, nil)

		mockOTPRepo.On("FindLatestByUserID", uint(1), "login").Return(nil, errors.New("not found"))
		mockOTPRepo.On("Create", mock.AnythingOfType("*models.OTP")).Return(nil)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		// Act
		user, err := authUC.Login("test@example.com", validPassword)

		// Assert
		assert.Nil(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, existingUser.Email, user.Email)

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})
}

func TestVerifyLoginOTP(t *testing.T) {
	// Setup: Need to set environment variables for JWT generation
	os.Setenv("JWT_SECRET", "supersecretjwt")
	os.Setenv("REFRESH_SECRET", "supersecretrefresh")

	t.Run("Should return not found error when user email does not exist", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		mockAuthRepo.On("FindByEmail", "unknown@example.com").Return(nil, errors.New("not found"))

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		accessToken, refreshToken, err := authUC.VerifyLoginOTP("unknown@example.com", "123456")

		assert.NotNil(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
		assert.Contains(t, err.Error(), "User not found")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return bad request error when OTP is invalid", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		user := &models.User{Email: "test@example.com"}
		user.ID = 1
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(user, nil)

		// Simulate finding a different OTP code in the database
		savedOTP := &models.OTP{OTPCode: "654321"}
		mockOTPRepo.On("FindLatestByUserID", uint(1), "login").Return(savedOTP, nil)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		accessToken, refreshToken, err := authUC.VerifyLoginOTP("test@example.com", "123456")

		assert.NotNil(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
		assert.Contains(t, err.Error(), "Invalid OTP code")

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})

	t.Run("Should return bad request error when OTP has expired", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		user := &models.User{Email: "test@example.com"}
		user.ID = 1
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(user, nil)

		// Simulate an expired OTP by setting the expiration time in the past
		expiredOTP := &models.OTP{OTPCode: "123456", ExpiresAt: time.Now().Add(-5 * time.Minute)}
		mockOTPRepo.On("FindLatestByUserID", uint(1), "login").Return(expiredOTP, nil)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		accessToken, refreshToken, err := authUC.VerifyLoginOTP("test@example.com", "123456")

		assert.NotNil(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
		assert.Contains(t, err.Error(), "OTP code has expired")

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})

	t.Run("Should return internal error when failing to save refresh token", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		user := &models.User{Email: "test@example.com"}
		user.ID = 1
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(user, nil)

		validOTP := &models.OTP{OTPCode: "123456", ExpiresAt: time.Now().Add(5 * time.Minute)}
		mockOTPRepo.On("FindLatestByUserID", uint(1), "login").Return(validOTP, nil)
		mockOTPRepo.On("DeleteByUserIDAndType", uint(1), "login").Return(nil)

		// Simulate a database failure when saving the new refresh token
		dbError := errors.New("failed to save token")
		mockAuthRepo.On("CreateRefreshToken", mock.AnythingOfType("*models.RefreshToken")).Return(dbError)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		accessToken, refreshToken, err := authUC.VerifyLoginOTP("test@example.com", "123456")

		assert.NotNil(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
		assert.Contains(t, err.Error(), "Failed to save session data")

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})

	t.Run("Should successfully verify OTP and return tokens", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		user := &models.User{Email: "test@example.com"}
		user.ID = 1
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(user, nil)

		validOTP := &models.OTP{OTPCode: "123456", ExpiresAt: time.Now().Add(5 * time.Minute)}
		mockOTPRepo.On("FindLatestByUserID", uint(1), "login").Return(validOTP, nil)
		mockOTPRepo.On("DeleteByUserIDAndType", uint(1), "login").Return(nil)

		mockAuthRepo.On("CreateRefreshToken", mock.AnythingOfType("*models.RefreshToken")).Return(nil)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		accessToken, refreshToken, err := authUC.VerifyLoginOTP("test@example.com", "123456")

		assert.Nil(t, err)
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, refreshToken)

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})
}

func TestVerifyRegisterOTP(t *testing.T) {
	t.Run("Should return not found error when user email does not exist", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		mockAuthRepo.On("FindByEmail", "unknown@example.com").Return(nil, errors.New("not found"))

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		err := authUC.VerifyRegisterOTP("unknown@example.com", "123456")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "User not found")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return bad request error when OTP is invalid", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		user := &models.User{Email: "test@example.com"}
		user.ID = 1
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(user, nil)

		// Simulate finding a different OTP code in the database
		savedOTP := &models.OTP{OTPCode: "654321"}
		mockOTPRepo.On("FindLatestByUserID", uint(1), "register").Return(savedOTP, nil)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		err := authUC.VerifyRegisterOTP("test@example.com", "123456")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Invalid OTP code")

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})

	t.Run("Should return bad request error when OTP has expired", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		user := &models.User{Email: "test@example.com"}
		user.ID = 1
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(user, nil)

		// Simulate an expired OTP
		expiredOTP := &models.OTP{OTPCode: "123456", ExpiresAt: time.Now().Add(-5 * time.Minute)}
		mockOTPRepo.On("FindLatestByUserID", uint(1), "register").Return(expiredOTP, nil)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		err := authUC.VerifyRegisterOTP("test@example.com", "123456")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "OTP code has expired")

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})

	t.Run("Should successfully verify user registration OTP", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		user := &models.User{Email: "test@example.com"}
		user.ID = 1
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(user, nil)

		validOTP := &models.OTP{OTPCode: "123456", ExpiresAt: time.Now().Add(5 * time.Minute)}
		mockOTPRepo.On("FindLatestByUserID", uint(1), "register").Return(validOTP, nil)

		// Expect the status update and OTP deletion to be called
		mockAuthRepo.On("UpdateVerificationStatus", uint(1), true).Return(nil)
		mockOTPRepo.On("DeleteByUserIDAndType", uint(1), "register").Return(nil)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		err := authUC.VerifyRegisterOTP("test@example.com", "123456")

		assert.Nil(t, err)

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})
}

func TestForgotPasswordRequest(t *testing.T) {
	t.Run("Should return generic error when email is not found to prevent user enumeration", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		// Simulate user not found
		mockAuthRepo.On("FindByEmail", "unknown@example.com").Return(nil, errors.New("not found"))

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		err := authUC.ForgotPasswordRequest("unknown@example.com")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "If the email is registered")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return too many requests error if requested within 1 minute", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		user := &models.User{Email: "test@example.com"}
		user.ID = 1
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(user, nil)

		// Simulate a recent OTP request (less than 1 minute ago)
		recentOTP := &models.OTP{CreatedAt: time.Now().Add(-30 * time.Second)}
		mockOTPRepo.On("FindLatestByUserID", uint(1), "forgot_password").Return(recentOTP, nil)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		err := authUC.ForgotPasswordRequest("test@example.com")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Please wait 1 minute")

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})

	t.Run("Should successfully process forgot password request and create OTP", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		user := &models.User{Email: "test@example.com"}
		user.ID = 1
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(user, nil)

		// Simulate no recent OTP
		mockOTPRepo.On("FindLatestByUserID", uint(1), "forgot_password").Return(nil, errors.New("not found"))
		mockOTPRepo.On("Create", mock.AnythingOfType("*models.OTP")).Return(nil)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		err := authUC.ForgotPasswordRequest("test@example.com")

		assert.Nil(t, err)

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})
}

func TestResetPassword(t *testing.T) {
	t.Run("Should return not found error when email does not exist", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		mockAuthRepo.On("FindByEmail", "unknown@example.com").Return(nil, errors.New("not found"))

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		err := authUC.ResetPassword("unknown@example.com", "123456", "newpassword123")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "User not found")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return bad request error when OTP is invalid", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		user := &models.User{Email: "test@example.com"}
		user.ID = 1
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(user, nil)

		savedOTP := &models.OTP{OTPCode: "654321"}
		mockOTPRepo.On("FindLatestByUserID", uint(1), "forgot_password").Return(savedOTP, nil)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		err := authUC.ResetPassword("test@example.com", "123456", "newpassword123")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Invalid OTP code")

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})

	t.Run("Should return bad request error when OTP has expired", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		user := &models.User{Email: "test@example.com"}
		user.ID = 1
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(user, nil)

		expiredOTP := &models.OTP{OTPCode: "123456", ExpiresAt: time.Now().Add(-5 * time.Minute)}
		mockOTPRepo.On("FindLatestByUserID", uint(1), "forgot_password").Return(expiredOTP, nil)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		err := authUC.ResetPassword("test@example.com", "123456", "newpassword123")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "OTP code has expired")

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})

	t.Run("Should return internal error when failing to update password in database", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		user := &models.User{Email: "test@example.com"}
		user.ID = 1
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(user, nil)

		validOTP := &models.OTP{OTPCode: "123456", ExpiresAt: time.Now().Add(5 * time.Minute)}
		mockOTPRepo.On("FindLatestByUserID", uint(1), "forgot_password").Return(validOTP, nil)

		// Simulate database error during user update
		dbError := errors.New("database update failed")
		mockAuthRepo.On("Update", mock.AnythingOfType("*models.User")).Return(dbError)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		err := authUC.ResetPassword("test@example.com", "123456", "newpassword123")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Failed to update password")

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})

	t.Run("Should successfully reset password", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockOTPRepo := new(MockOTPRepository)

		user := &models.User{Email: "test@example.com"}
		user.ID = 1
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(user, nil)

		validOTP := &models.OTP{OTPCode: "123456", ExpiresAt: time.Now().Add(5 * time.Minute)}
		mockOTPRepo.On("FindLatestByUserID", uint(1), "forgot_password").Return(validOTP, nil)

		mockAuthRepo.On("Update", mock.AnythingOfType("*models.User")).Return(nil)
		mockOTPRepo.On("DeleteByUserIDAndType", uint(1), "forgot_password").Return(nil)

		authUC := NewAuthUsecase(mockAuthRepo, mockOTPRepo, nil)

		err := authUC.ResetPassword("test@example.com", "123456", "newpassword123")

		assert.Nil(t, err)

		mockAuthRepo.AssertExpectations(t)
		mockOTPRepo.AssertExpectations(t)
	})
}

func TestRefreshToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "supersecretjwt")

	t.Run("Should return unauthorized error when refresh token is invalid or not found", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		// Simulate token not found in the database
		mockAuthRepo.On("GetRefreshToken", "invalid_token").Return(nil, errors.New("not found"))

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		newToken, err := authUC.RefreshToken("invalid_token")

		assert.NotNil(t, err)
		assert.Empty(t, newToken)
		assert.Contains(t, err.Error(), "Invalid or expired session")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return unauthorized error and delete token when refresh token has expired", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		// Simulate finding an expired token (set expiration time in the past)
		expiredToken := &models.RefreshToken{
			RefreshToken: "expired_token",
			ExpiresAt:    time.Now().Add(-24 * time.Hour),
			UserID:       1,
		}

		mockAuthRepo.On("GetRefreshToken", "expired_token").Return(expiredToken, nil)

		// Expect the system to delete the token from the database due to expiration
		mockAuthRepo.On("DeleteRefreshToken", "expired_token").Return(nil)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		newToken, err := authUC.RefreshToken("expired_token")

		assert.NotNil(t, err)
		assert.Empty(t, newToken)
		assert.Contains(t, err.Error(), "Session has expired")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should successfully generate a new access token", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		// Simulate finding a valid, active token
		validToken := &models.RefreshToken{
			RefreshToken: "valid_token",
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			UserID:       1,
		}

		mockAuthRepo.On("GetRefreshToken", "valid_token").Return(validToken, nil)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		newToken, err := authUC.RefreshToken("valid_token")

		assert.Nil(t, err)
		assert.NotEmpty(t, newToken)

		mockAuthRepo.AssertExpectations(t)
	})
}

func TestLogout(t *testing.T) {
	os.Setenv("JWT_SECRET", "supersecretjwt")

	// Setup: We must generate a REAL valid JWT token so the jwt.Parse logic
	// inside the Logout usecase can actually extract the claims and cover those red lines.
	claims := jwt.MapClaims{
		"user_id": float64(99), // We inject user ID 99 to prove the extraction works
		"exp":     time.Now().Add(time.Minute * 15).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	validAccessToken, _ := token.SignedString([]byte("supersecretjwt"))

	t.Run("Should return internal error when failing to create revoke token", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		mockAuthRepo.On("DeleteRefreshToken", "some_refresh_token").Return(nil)

		dbError := errors.New("database insert failed")
		mockAuthRepo.On("CreateRevokeToken", mock.AnythingOfType("*models.RevokeToken")).Return(dbError)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		err := authUC.Logout(validAccessToken, "some_refresh_token")

		assert.NotNil(t, err)
		assert.Equal(t, dbError, err)

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should successfully logout, extract JWT claims, and blacklist token", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		mockAuthRepo.On("DeleteRefreshToken", "some_refresh_token").Return(nil)

		// We use .Run to peek into the arguments being passed to the mock.
		// If the JWT extraction logic works, it should pass UserID: 99 to the repository.
		mockAuthRepo.On("CreateRevokeToken", mock.AnythingOfType("*models.RevokeToken")).Return(nil).Run(func(args mock.Arguments) {
			revoked := args.Get(0).(*models.RevokeToken)
			assert.Equal(t, uint(99), revoked.UserID)
		})

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		err := authUC.Logout(validAccessToken, "some_refresh_token")

		assert.Nil(t, err)

		mockAuthRepo.AssertExpectations(t)
	})
}

func TestSendOrResendOTP(t *testing.T) {
	t.Run("Should return too many requests error if requested within 1 minute", func(t *testing.T) {
		mockOTPRepo := new(MockOTPRepository)

		// Simulate a recent OTP request made just 30 seconds ago
		recentOTP := &models.OTP{CreatedAt: time.Now().Add(-30 * time.Second)}
		mockOTPRepo.On("FindLatestByUserID", uint(1), "register").Return(recentOTP, nil)

		authUC := NewAuthUsecase(nil, mockOTPRepo, nil)

		err := authUC.SendOrResendOTP(1, "test@example.com", "register")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "wait 1 minute before requesting")

		mockOTPRepo.AssertExpectations(t)
	})

	t.Run("Should return internal error when failing to save new OTP to database", func(t *testing.T) {
		mockOTPRepo := new(MockOTPRepository)

		mockOTPRepo.On("FindLatestByUserID", uint(1), "register").Return(nil, errors.New("not found"))

		dbError := errors.New("db crash")
		mockOTPRepo.On("Create", mock.AnythingOfType("*models.OTP")).Return(dbError)

		authUC := NewAuthUsecase(nil, mockOTPRepo, nil)

		err := authUC.SendOrResendOTP(1, "test@example.com", "register")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Failed to generate OTP")

		mockOTPRepo.AssertExpectations(t)
	})

	t.Run("Should successfully generate and dispatch OTP via email goroutine", func(t *testing.T) {
		mockOTPRepo := new(MockOTPRepository)

		mockOTPRepo.On("FindLatestByUserID", uint(1), "register").Return(nil, errors.New("not found"))
		mockOTPRepo.On("Create", mock.AnythingOfType("*models.OTP")).Return(nil)

		authUC := NewAuthUsecase(nil, mockOTPRepo, nil)

		err := authUC.SendOrResendOTP(1, "test@example.com", "register")

		assert.Nil(t, err)

		mockOTPRepo.AssertExpectations(t)
	})
}

func TestGetUserByEmail(t *testing.T) {
	t.Run("Should return not found error when user email does not exist", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		mockAuthRepo.On("FindByEmail", "unknown@example.com").Return(nil, errors.New("not found"))

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		user, err := authUC.GetUserByEmail("unknown@example.com")

		assert.NotNil(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "was not found")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should successfully return user data when email exists", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		existingUser := &models.User{Email: "test@example.com"}
		mockAuthRepo.On("FindByEmail", "test@example.com").Return(existingUser, nil)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		user, err := authUC.GetUserByEmail("test@example.com")

		assert.Nil(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "test@example.com", user.Email)

		mockAuthRepo.AssertExpectations(t)
	})
}

func TestLoginSSO(t *testing.T) {
	os.Setenv("JWT_SECRET", "supersecretjwt")
	os.Setenv("REFRESH_SECRET", "supersecretrefresh")

	t.Run("Should return internal error when failing to create a new user via SSO", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		// Simulate user not found, prompting SSO to attempt account creation
		mockAuthRepo.On("FindByEmail", "new@example.com").Return(nil, errors.New("not found"))

		dbError := errors.New("database insert failed")
		mockAuthRepo.On("Create", mock.AnythingOfType("*models.User")).Return(dbError)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		accessToken, refreshToken, err := authUC.LoginSSO("new@example.com", "New User")

		assert.NotNil(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
		assert.Contains(t, err.Error(), "Failed to create user via SSO")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should successfully register and login a brand new user via SSO", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		mockAuthRepo.On("FindByEmail", "new@example.com").Return(nil, errors.New("not found"))

		// Simulate successful creation and inject an auto-incremented ID
		mockAuthRepo.On("Create", mock.AnythingOfType("*models.User")).Return(nil).Run(func(args mock.Arguments) {
			u := args.Get(0).(*models.User)
			u.ID = 1
		})

		mockAuthRepo.On("CreateRefreshToken", mock.AnythingOfType("*models.RefreshToken")).Return(nil)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		accessToken, refreshToken, err := authUC.LoginSSO("new@example.com", "New User")

		assert.Nil(t, err)
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, refreshToken)

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should automatically verify and login an existing unverified user", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		existingUser := &models.User{Email: "existing@example.com", IsVerified: false}
		existingUser.ID = 2
		mockAuthRepo.On("FindByEmail", "existing@example.com").Return(existingUser, nil)

		// Expect the system to update the user's verification status
		mockAuthRepo.On("UpdateVerificationStatus", uint(2), true).Return(nil)
		mockAuthRepo.On("CreateRefreshToken", mock.AnythingOfType("*models.RefreshToken")).Return(nil)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		accessToken, refreshToken, err := authUC.LoginSSO("existing@example.com", "Existing User")

		assert.Nil(t, err)
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, refreshToken)

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return internal error when failing to save refresh token for SSO", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		existingUser := &models.User{Email: "existing@example.com", IsVerified: true}
		existingUser.ID = 3
		mockAuthRepo.On("FindByEmail", "existing@example.com").Return(existingUser, nil)

		dbError := errors.New("failed to save token")
		mockAuthRepo.On("CreateRefreshToken", mock.AnythingOfType("*models.RefreshToken")).Return(dbError)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		accessToken, refreshToken, err := authUC.LoginSSO("existing@example.com", "Existing User")

		assert.NotNil(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
		assert.Contains(t, err.Error(), "Failed to save session data")

		mockAuthRepo.AssertExpectations(t)
	})
}

func TestFindByID(t *testing.T) {
	ctx := context.Background()

	t.Run("Should return error when user is not found", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		// Parameter ketiga sekarang kita masukin mockGoogleService
		mockGoogleService := new(MockGoogleAuthService)

		mockAuthRepo.On("FindByID", uint(1)).Return(nil, errors.New("user not found"))

		authUC := NewAuthUsecase(mockAuthRepo, nil, mockGoogleService)

		user, err := authUC.FindByID(ctx, 1)

		assert.NotNil(t, err)
		assert.Nil(t, user)

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return user directly when Gmail is disabled", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockGoogleService := new(MockGoogleAuthService)

		existingUser := &models.User{GmailEnabled: false}
		existingUser.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(existingUser, nil)

		authUC := NewAuthUsecase(mockAuthRepo, nil, mockGoogleService)

		user, err := authUC.FindByID(ctx, 1)

		assert.Nil(t, err)
		assert.NotNil(t, user)
		assert.False(t, user.GmailEnabled)

		mockAuthRepo.AssertExpectations(t)
		// Ensure Google service was NOT called
		mockGoogleService.AssertNotCalled(t, "CheckTokenValidity")
	})

	t.Run("Should return user when Gmail is enabled and token is valid", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockGoogleService := new(MockGoogleAuthService)

		existingUser := &models.User{GmailEnabled: true, GoogleRefreshToken: "valid_google_token"}
		existingUser.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(existingUser, nil)

		// Simulate token being valid from Google's end
		mockGoogleService.On("CheckTokenValidity", ctx, "valid_google_token").Return(true)

		authUC := NewAuthUsecase(mockAuthRepo, nil, mockGoogleService)

		user, err := authUC.FindByID(ctx, 1)

		assert.Nil(t, err)
		assert.NotNil(t, user)
		assert.True(t, user.GmailEnabled)

		mockAuthRepo.AssertExpectations(t)
		mockGoogleService.AssertExpectations(t)
	})

	t.Run("Should disable Gmail and update user when token is invalid", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)
		mockGoogleService := new(MockGoogleAuthService)

		existingUser := &models.User{GmailEnabled: true, GoogleRefreshToken: "invalid_google_token"}
		existingUser.ID = 1
		mockAuthRepo.On("FindByID", uint(1)).Return(existingUser, nil)

		// Simulate token being invalid from Google's end
		mockGoogleService.On("CheckTokenValidity", ctx, "invalid_google_token").Return(false)

		// Expect the system to update the database to turn off Gmail functionality
		mockAuthRepo.On("Update", mock.AnythingOfType("*models.User")).Return(nil)

		authUC := NewAuthUsecase(mockAuthRepo, nil, mockGoogleService)

		user, err := authUC.FindByID(ctx, 1)

		assert.Nil(t, err)
		assert.NotNil(t, user)
		assert.False(t, user.GmailEnabled) // Ensure it was disabled

		mockAuthRepo.AssertExpectations(t)
		mockGoogleService.AssertExpectations(t)
	})
}

func TestRequestBindingCode(t *testing.T) {
	t.Run("Should return internal error when database fails to set binding code", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		dbError := errors.New("failed to save code")
		// We use AnythingOfType for string and time.Time since they are generated dynamically inside the usecase
		mockAuthRepo.On("SetBindingCode", uint(1), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(dbError)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		code, err := authUC.RequestBindingCode(1)

		assert.NotNil(t, err)
		assert.Empty(t, code)
		assert.Contains(t, err.Error(), "Failed to generate binding code")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should successfully generate and return binding code", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		mockAuthRepo.On("SetBindingCode", uint(1), mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		code, err := authUC.RequestBindingCode(1)

		assert.Nil(t, err)
		assert.NotEmpty(t, code)
		assert.Contains(t, code, "NSV-")

		mockAuthRepo.AssertExpectations(t)
	})
}

func TestVerifyAndBindTelegram(t *testing.T) {
	t.Run("Should return bad request error when binding code is invalid or expired", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		mockAuthRepo.On("FindByBindingCode", "INVALID_CODE").Return(nil, errors.New("not found"))

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		err := authUC.VerifyAndBindTelegram(999888, "INVALID_CODE")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid or has expired")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return conflict error when Telegram ID is already linked to another user", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		// The user trying to bind
		bindingUser := &models.User{}
		bindingUser.ID = 1
		mockAuthRepo.On("FindByBindingCode", "VALID_CODE").Return(bindingUser, nil)

		// Simulating that the telegram ID is already used by a DIFFERENT user (ID: 2)
		anotherUser := &models.User{}
		anotherUser.ID = 2
		mockAuthRepo.On("GetByTelegramID", int64(999888)).Return(anotherUser, nil)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		err := authUC.VerifyAndBindTelegram(999888, "VALID_CODE")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "already linked to another user")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should return internal error when database fails to finalize binding", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		bindingUser := &models.User{}
		bindingUser.ID = 1
		mockAuthRepo.On("FindByBindingCode", "VALID_CODE").Return(bindingUser, nil)

		// Simulating the telegram ID is free
		mockAuthRepo.On("GetByTelegramID", int64(999888)).Return(nil, errors.New("not found"))

		dbError := errors.New("db crash")
		mockAuthRepo.On("FinalizeBinding", uint(1), int64(999888)).Return(dbError)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		err := authUC.VerifyAndBindTelegram(999888, "VALID_CODE")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Failed to link Telegram account")

		mockAuthRepo.AssertExpectations(t)
	})

	t.Run("Should successfully verify code and bind Telegram account", func(t *testing.T) {
		mockAuthRepo := new(MockAuthRepository)

		bindingUser := &models.User{}
		bindingUser.ID = 1
		mockAuthRepo.On("FindByBindingCode", "VALID_CODE").Return(bindingUser, nil)

		mockAuthRepo.On("GetByTelegramID", int64(999888)).Return(nil, errors.New("not found"))
		mockAuthRepo.On("FinalizeBinding", uint(1), int64(999888)).Return(nil)

		authUC := NewAuthUsecase(mockAuthRepo, nil, nil)

		err := authUC.VerifyAndBindTelegram(999888, "VALID_CODE")

		assert.Nil(t, err)

		mockAuthRepo.AssertExpectations(t)
	})
}

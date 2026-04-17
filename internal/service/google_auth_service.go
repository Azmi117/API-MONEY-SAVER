package service

import (
	"context"
	"fmt"
	"os"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type GoogleAuthService interface {
	GetAuthURL(userID uint) string
	ExchangeCode(ctx context.Context, userID uint, code string) error
	GetGmailService(refreshToken string) (*gmail.Service, error)
}

type googleAuthService struct {
	config   *oauth2.Config
	authRepo repository.AuthRepository
}

func NewGoogleAuthService(authRepo repository.AuthRepository) GoogleAuthService {
	// Ambil dari Env atau Config lo
	conf := &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		Scopes: []string{
			gmail.GmailReadonlyScope, // Hanya izin baca
		},
		Endpoint: google.Endpoint,
	}

	return &googleAuthService{
		config:   conf,
		authRepo: authRepo,
	}
}

func (s *googleAuthService) GetAuthURL(userID uint) string {
	// Kita selipin userID di parameter state
	state := fmt.Sprintf("%d", userID)
	return s.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

func (s *googleAuthService) ExchangeCode(ctx context.Context, userID uint, code string) error {
	// 1. Tukar code jadi token lewat library Google
	token, err := s.config.Exchange(ctx, code)
	if err != nil {
		return apperror.BadRequest("Gagal tukar kode Google")
	}

	// 2. Cari user (tanpa ctx sesuai repository lo)
	user, err := s.authRepo.FindByID(userID)
	if err != nil {
		return err // FindByID lo udah balikin error kalau nggak ketemu
	}

	// 3. Update field OAuth
	user.GoogleRefreshToken = token.RefreshToken
	user.GmailEnabled = true
	user.GoogleTokenExpires = token.Expiry

	// 4. Simpan perubahan ke DB
	return s.authRepo.Update(user)
}

func (s *googleAuthService) GetGmailService(refreshToken string) (*gmail.Service, error) {
	ctx := context.Background()
	// Pakai config oauth yang udah kita buat sebelumnya
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	// client ini otomatis bakal nge-refresh access_token kalau expired
	client := s.config.Client(ctx, token)

	return gmail.NewService(ctx, option.WithHTTPClient(client))
}

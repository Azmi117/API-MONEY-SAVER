package http

import (
	"net/http"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/middleware"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
)

func MapRoutes(mux *http.ServeMux, aH *authHandler, aR repository.AuthRepository) {
	registerV1Routes(mux, aH, aR)

	fs := http.FileServer(http.Dir("./uploads"))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads", fs))
}

func registerV1Routes(mux *http.ServeMux, aH *authHandler, authRepo repository.AuthRepository) {
	prefix := "/api/v1/"
	authMW := middleware.Authenticate(authRepo)

	mux.HandleFunc("POST "+prefix+"auth/login", aH.Login)
	mux.HandleFunc("POST "+prefix+"auth/register", aH.Register)
	mux.HandleFunc("POST "+prefix+"auth/refresh", aH.Refresh)
	mux.HandleFunc("POST "+prefix+"auth/logout", authMW(aH.Logout))
}

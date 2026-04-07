package http

import "net/http"

func MapRoutes(mux *http.ServeMux, aH *authHandler) {
	registerV1Routes(mux, aH)

	fs := http.FileServer(http.Dir("./uploads"))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads", fs))
}

func registerV1Routes(mux *http.ServeMux, aH *authHandler) {
	prefix := "/api/v1/"

	mux.HandleFunc("POST "+prefix+"auth/login", aH.Login)
	mux.HandleFunc("POST "+prefix+"auth/register", aH.Register)
	mux.HandleFunc("POST "+prefix+"auth/refresh", aH.Refresh)
}

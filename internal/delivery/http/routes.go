package http

import (
	"net/http"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/middleware"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"gorm.io/gorm"
)

func MapRoutes(mux *http.ServeMux, aH *authHandler, wH *WorkspaceHandler, aR repository.AuthRepository, db *gorm.DB) {
	registerV1Routes(mux, aH, wH, aR, db)

	fs := http.FileServer(http.Dir("./uploads"))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads", fs))
}

func registerV1Routes(mux *http.ServeMux, aH *authHandler, wH *WorkspaceHandler, authRepo repository.AuthRepository, db *gorm.DB) {
	prefix := "/api/v1/"
	authMW := middleware.Authenticate(authRepo)
	ownerMW := middleware.AuthorizeWorkspaceOwner(db)

	// --- AUTH ROUTES ---
	mux.HandleFunc("POST "+prefix+"auth/login", aH.Login)
	mux.HandleFunc("POST "+prefix+"auth/register", aH.Register)
	mux.HandleFunc("POST "+prefix+"auth/refresh", aH.Refresh)
	mux.HandleFunc("POST "+prefix+"auth/logout", authMW(aH.Logout))

	// --- WORKSPACE ROUTES ---
	// Create & Get List
	mux.HandleFunc("POST "+prefix+"workspaces", authMW(wH.Create))
	mux.HandleFunc("GET "+prefix+"workspaces", authMW(wH.GetMyWorkspaces))

	// Update & Delete (Hanya Owner)
	// Update pake PUT, URL format: /api/v1/workspaces?id=1
	mux.HandleFunc("PUT "+prefix+"workspaces", authMW(ownerMW(wH.UpdateWorkspace)))
	// Delete pake DELETE, URL format: /api/v1/workspaces?id=1
	mux.HandleFunc("DELETE "+prefix+"workspaces", authMW(ownerMW(wH.DeleteWorkspace)))

	// --- INVITATION ROUTES ---
	mux.HandleFunc("POST "+prefix+"workspaces/invite", authMW(wH.Invite))
	// Respond pake PATCH karena mengupdate sebagian data (status)
	mux.HandleFunc("POST "+prefix+"workspaces/invitations/respond", authMW(wH.RespondInvitation))
}

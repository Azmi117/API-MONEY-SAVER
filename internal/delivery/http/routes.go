package http

import (
	"net/http"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/middleware"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"gorm.io/gorm"
)

func MapRoutes(mux *http.ServeMux, aH *authHandler, wH *WorkspaceHandler, tH *TransactionHandler, aR repository.AuthRepository, db *gorm.DB) {
	registerV1Routes(mux, aH, wH, tH, aR, db)

	fs := http.FileServer(http.Dir("./uploads"))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads", fs))
}

func registerV1Routes(mux *http.ServeMux, aH *authHandler, wH *WorkspaceHandler, tH *TransactionHandler, authRepo repository.AuthRepository, db *gorm.DB) {
	prefix := "/api/v1/"
	authMW := middleware.Authenticate(authRepo)
	ownerMW := middleware.AuthorizeWorkspaceOwner(db)

	// --- AUTH ROUTES ---
	mux.HandleFunc("POST "+prefix+"auth/login", aH.Login)
	mux.HandleFunc("POST "+prefix+"auth/register", aH.Register)
	mux.HandleFunc("POST "+prefix+"auth/refresh", aH.Refresh)
	mux.HandleFunc("POST "+prefix+"auth/logout", authMW(aH.Logout))

	// --- TELEGRAM BINDING ROUTE (TAMBAHAN) ---
	// User harus login dulu buat dapet kode binding
	mux.HandleFunc("GET "+prefix+"auth/telegram/binding-code", authMW(aH.GetBindingCode))

	// --- GOOGLE OAUTH ROUTES ---
	// User harus login apps dulu buat "Link Gmail"
	mux.HandleFunc("GET "+prefix+"auth/google/login", authMW(aH.GoogleLogin))
	// Callback dari Google (tanpa middleware auth)
	mux.HandleFunc("GET "+prefix+"auth/google/callback", aH.GoogleCallback)

	// --- WORKSPACE ROUTES ---
	mux.HandleFunc("POST "+prefix+"workspaces", authMW(wH.Create))
	mux.HandleFunc("GET "+prefix+"workspaces", authMW(wH.GetMyWorkspaces))
	mux.HandleFunc("PUT "+prefix+"workspaces", authMW(ownerMW(wH.UpdateWorkspace)))
	mux.HandleFunc("DELETE "+prefix+"workspaces", authMW(ownerMW(wH.DeleteWorkspace)))

	// --- INVITATION ROUTES ---
	mux.HandleFunc("POST "+prefix+"workspaces/{id}/invite", authMW(wH.Invite))
	mux.HandleFunc("POST "+prefix+"workspaces/invitations/{id}/accept", authMW(wH.AcceptInvitation))
	mux.HandleFunc("POST "+prefix+"workspaces/invitations/{id}/reject", authMW(wH.RejectInvitation))

	// --- TRANSACTION ROUTES ---
	mux.HandleFunc("POST "+prefix+"transactions/manual", authMW(tH.CreateManual))
	mux.HandleFunc("GET "+prefix+"transactions", authMW(ownerMW(tH.GetHistory)))
	mux.HandleFunc("DELETE "+prefix+"transactions", authMW(tH.Delete))
	mux.HandleFunc("POST "+prefix+"transactions/scan", authMW(tH.ScanReceipt))
	mux.HandleFunc("PATCH "+prefix+"transactions/confirm", authMW(tH.Confirm))
	mux.HandleFunc("POST "+prefix+"transactions/scan-hybrid2", authMW(tH.ScanReceiptHybrid))
	mux.HandleFunc("GET "+prefix+"emails/pending", authMW(tH.GetPendingEmails))
	mux.HandleFunc("POST "+prefix+"emails/{id}/approve", authMW(tH.ApproveEmail))
	mux.HandleFunc("POST "+prefix+"emails/{id}/reject", authMW(tH.RejectEmail))
}

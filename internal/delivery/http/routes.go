package http

import (
	"net/http"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/middleware"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"gorm.io/gorm"
)

func MapRoutes(mux *http.ServeMux, aH *authHandler, wH *WorkspaceHandler, tH *TransactionHandler, dH *DebtHandler, cH *CategoryHandler, aR repository.AuthRepository, db *gorm.DB) {
	registerV1Routes(mux, aH, wH, tH, dH, cH, aR, db)

	fs := http.FileServer(http.Dir("./uploads"))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads", fs))
}

func registerV1Routes(mux *http.ServeMux, aH *authHandler, wH *WorkspaceHandler, tH *TransactionHandler, dH *DebtHandler, cH *CategoryHandler, authRepo repository.AuthRepository, db *gorm.DB) {
	prefix := "/api/v1/"
	authMW := middleware.Authenticate(authRepo)
	ownerMW := middleware.AuthorizeWorkspaceOwner(db)
	memberMW := middleware.AuthorizeWorkspaceMember(db)

	// --- AUTH ROUTES ---
	mux.HandleFunc("POST "+prefix+"auth/login", aH.Login)           //done
	mux.HandleFunc("POST "+prefix+"auth/register", aH.Register)     //done
	mux.HandleFunc("POST "+prefix+"auth/refresh", aH.Refresh)       //done
	mux.HandleFunc("POST "+prefix+"auth/logout", authMW(aH.Logout)) //done

	// --- TELEGRAM BINDING ROUTE ---
	mux.HandleFunc("GET "+prefix+"auth/telegram/binding-code", authMW(aH.GetBindingCode)) //done

	// --- GOOGLE OAUTH ROUTES ---
	mux.HandleFunc("GET "+prefix+"auth/google/login", authMW(aH.GoogleLogin)) //done
	mux.HandleFunc("GET "+prefix+"auth/google/callback", aH.GoogleCallback)   //done

	// --- WORKSPACE ROUTES ---
	mux.HandleFunc("POST "+prefix+"workspaces", authMW(wH.Create))                          //done
	mux.HandleFunc("GET "+prefix+"workspaces", authMW(wH.GetMyWorkspaces))                  //done
	mux.HandleFunc("PUT "+prefix+"workspaces/{id}", authMW(ownerMW(wH.UpdateWorkspace)))    //done
	mux.HandleFunc("DELETE "+prefix+"workspaces/{id}", authMW(ownerMW(wH.DeleteWorkspace))) //done
	mux.HandleFunc("POST "+prefix+"workspaces/target", authMW(wH.SetTarget))                //done
	mux.HandleFunc("GET "+prefix+"workspaces/{id}/members", authMW(wH.GetMembers))          //done

	// --- INVITATION ROUTES ---
	mux.HandleFunc("POST "+prefix+"workspaces/{id}/invite", authMW(wH.Invite))                       //done
	mux.HandleFunc("POST "+prefix+"workspaces/invitations/{id}/accept", authMW(wH.AcceptInvitation)) //done
	mux.HandleFunc("POST "+prefix+"workspaces/invitations/{id}/reject", authMW(wH.RejectInvitation)) //done

	// --- TRANSACTION ROUTES ---
	mux.HandleFunc("POST "+prefix+"transactions/manual", authMW(tH.CreateManual))                 //done
	mux.HandleFunc("GET "+prefix+"workspaces/{id}/transactions", authMW(memberMW(tH.GetHistory))) //done
	mux.HandleFunc("DELETE "+prefix+"transactions/{id}", authMW(tH.Delete))                       //done
	mux.HandleFunc("PATCH "+prefix+"transactions/{id}/confirm", authMW(tH.Confirm))               //done
	mux.HandleFunc("POST "+prefix+"transactions/scan-hybrid2", authMW(tH.ScanReceiptHybrid))      //done
	mux.HandleFunc("POST "+prefix+"transactions/scan-alt", authMW(tH.ScanAlternative))            //done
	mux.HandleFunc("POST "+prefix+"transactions/scan-alt/confirm", authMW(tH.ConfirmScan))        //done

	// --- PENDING & EMAIL ROUTES ---
	mux.HandleFunc("GET "+prefix+"emails/pending", authMW(tH.GetPendingEmails))   //done
	mux.HandleFunc("POST "+prefix+"emails/{id}/approve", authMW(tH.ApproveEmail)) //done
	mux.HandleFunc("POST "+prefix+"emails/{id}/reject", authMW(tH.RejectEmail))   //done

	// --- DEBT & SPLIT BILL ROUTES ---
	mux.HandleFunc("GET "+prefix+"workspaces/{id}/debts", authMW(dH.GetWorkspaceDebts)) //done
	mux.HandleFunc("POST "+prefix+"transactions/split", authMW(dH.AssignSplitBill))     //done   // <-- Pindah ke dH (DebtHandler)
	mux.HandleFunc("PATCH "+prefix+"debts/{id}/pay", authMW(dH.PayDebt))
	// --- CATEGORY ROUTES ---
	mux.HandleFunc("POST "+prefix+"workspaces/{id}/categories", authMW(ownerMW(cH.Create))) //done
	mux.HandleFunc("GET "+prefix+"workspaces/{id}/categories", authMW(cH.GetByWorkspace))   //done
}

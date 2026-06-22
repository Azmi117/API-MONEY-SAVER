package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/testutils"
	"github.com/stretchr/testify/assert"
)

func TestIntegration_WorkspaceAPI(t *testing.T) {
	// 1. SETUP TEST DATABASE
	db := testutils.SetupTestDB()
	defer testutils.CleanTestDB(db)

	// 2. SEED DUMMY USER
	user := models.User{
		Name:  "Azmi Integration",
		Email: "azmi@integration.com",
	}
	db.Create(&user)

	user2 := models.User{
		Name:  "Invitee User",
		Email: "invitee@integration.com",
	}
	db.Create(&user2)

	// 3. SETUP DEPENDENCY INJECTION
	wsRepo := repository.NewWorkspaceRepository(db)
	authRepo := repository.NewAuthRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	targetRepo := repository.NewTargetRepository(db)
	txRepo := repository.NewTransactionRepository(db)

	wsUsecase := usecase.NewWorkspaceUsecase(wsRepo, authRepo, categoryRepo, targetRepo)
	targetUsecase := usecase.NewTargetUsecase(targetRepo, txRepo)

	wsHandler := NewWorkspaceHandler(wsUsecase, targetUsecase)

	// HELPER: Reset Database State before each test
	resetDB := func() {
		testutils.CleanTestDB(db)
		// Re-create the user since CleanTestDB truncates all tables
		db.Create(&user)
	}

	// ==========================================
	// SCENARIO 1: CREATE WORKSPACE
	// ==========================================
	t.Run("Should successfully create a new workspace", func(t *testing.T) {
		resetDB()

		payload := `{"name": "Family Budget", "type": "budgeting"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", strings.NewReader(payload))

		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		wsHandler.Create(w, req)

		assert.Equal(t, http.StatusCreated, w.Code, "Expected HTTP status to be 201 Created")

		var count int64
		db.Model(&models.Workspace{}).Where("name = ?", "Family Budget").Count(&count)
		assert.Equal(t, int64(1), count, "Expected exactly 1 workspace record in the database")
	})

	// ==========================================
	// SCENARIO 2: GET WORKSPACES
	// ==========================================
	t.Run("Should successfully retrieve user workspaces", func(t *testing.T) {
		resetDB()

		// 1. Seed a workspace into the database
		ws := models.Workspace{Name: "My Personal Wallet", Type: "budgeting", OwnerID: user.ID}
		db.Create(&ws)

		// 2. IMPORTANT FIX: Seed the workspace_members relation!
		// The repository uses a JOIN on this table, so it must exist.
		member := models.WorkspaceMember{UserID: user.ID, WorkspaceID: ws.ID}
		db.Create(&member)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		wsHandler.GetMyWorkspaces(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected HTTP status to be 200 OK")

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		// Assert that the data array contains at least 1 item
		data := response["data"].([]interface{})
		assert.GreaterOrEqual(t, len(data), 1, "Expected data array to contain the seeded workspace")
	})

	// ==========================================
	// SCENARIO 3: UPDATE WORKSPACE
	// ==========================================
	t.Run("Should successfully update workspace name", func(t *testing.T) {
		resetDB()

		// Seed the old workspace
		ws := models.Workspace{Name: "Old Name", Type: "budgeting", OwnerID: user.ID}
		db.Create(&ws)

		payload := `{"name": "Updated Name"}`
		targetURL := fmt.Sprintf("/api/v1/workspaces/%d", ws.ID)
		req := httptest.NewRequest(http.MethodPut, targetURL, strings.NewReader(payload))

		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)

		// ✨ MAGIC GO 1.22 ✨: Inject the path variable "id" manually for testing
		req.SetPathValue("id", fmt.Sprintf("%d", ws.ID))

		w := httptest.NewRecorder()
		wsHandler.UpdateWorkspace(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected HTTP status to be 200 OK")

		// Verify database state
		var updatedWs models.Workspace
		db.First(&updatedWs, ws.ID)
		assert.Equal(t, "Updated Name", updatedWs.Name, "Expected workspace name to be updated in the database")
	})

	// ==========================================
	// SCENARIO 4: DELETE WORKSPACE
	// ==========================================
	t.Run("Should successfully delete workspace", func(t *testing.T) {
		resetDB()

		ws := models.Workspace{Name: "To Be Deleted", Type: "split", OwnerID: user.ID}
		db.Create(&ws)

		targetURL := fmt.Sprintf("/api/v1/workspaces/%d", ws.ID)
		req := httptest.NewRequest(http.MethodDelete, targetURL, nil)

		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)

		// Set Path variable
		req.SetPathValue("id", fmt.Sprintf("%d", ws.ID))

		w := httptest.NewRecorder()
		wsHandler.DeleteWorkspace(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected HTTP status to be 200 OK")

		// Verify database state (Ensure the record is deleted)
		var count int64
		db.Model(&models.Workspace{}).Where("id = ?", ws.ID).Count(&count)
		assert.Equal(t, int64(0), count, "Expected the workspace record to be removed from the database")
	})
	// ==========================================
	// SCENARIO 5: INVITE MEMBER
	// ==========================================
	t.Run("Should successfully invite another user to the workspace", func(t *testing.T) {
		resetDB()

		// Bikin user baru yang bener-bener fresh biar gak bentrok ID-nya
		freshInvitee := models.User{
			Name:  "Fresh Invitee",
			Email: "fresh.invitee@integration.com",
		}
		db.Create(&freshInvitee)

		ws := models.Workspace{Name: "Shared Wallet", Type: "split", OwnerID: user.ID}
		db.Create(&ws)
		db.Create(&models.WorkspaceMember{UserID: user.ID, WorkspaceID: ws.ID})

		payload := fmt.Sprintf(`{"email": "%s"}`, freshInvitee.Email)
		targetURL := fmt.Sprintf("/api/v1/workspaces/%d/invite", ws.ID)
		req := httptest.NewRequest(http.MethodPost, targetURL, strings.NewReader(payload))

		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)
		req.SetPathValue("id", fmt.Sprintf("%d", ws.ID))

		w := httptest.NewRecorder()
		wsHandler.Invite(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected HTTP status to be 200 OK")

		// Verify database state: Kita tembak langsung ke nama tabelnya biar GORM gak ngaco
		var count int64
		db.Table("workspace_invitations").Where("workspace_id = ?", ws.ID).Count(&count)
		assert.Equal(t, int64(1), count, "Expected exactly 1 invitation record in the database")
	})

	// ==========================================
	// SCENARIO 6: ACCEPT INVITATION
	// ==========================================
	t.Run("Should successfully accept a workspace invitation", func(t *testing.T) {
		resetDB()
		db.Create(&user2)

		ws := models.Workspace{Name: "Shared Wallet", Type: "split", OwnerID: user.ID}
		db.Create(&ws)

		// Seed the invitation with the correct field names
		invitation := models.WorkspaceInvitation{
			WorkspaceID: ws.ID,
			InvitedID:   user2.ID,
			InviterID:   user.ID,
			Status:      "pending",
		}
		db.Create(&invitation)

		// Adjust URL based on your routes
		targetURL := fmt.Sprintf("/api/v1/workspaces/invitations/%d/accept", invitation.ID)
		req := httptest.NewRequest(http.MethodPost, targetURL, nil)

		// IMPORTANT: The request context uses user2's ID, because user2 is accepting it!
		ctx := context.WithValue(req.Context(), "user_id", user2.ID)
		req = req.WithContext(ctx)
		req.SetPathValue("id", fmt.Sprintf("%d", invitation.ID))

		w := httptest.NewRecorder()
		wsHandler.AcceptInvitation(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected HTTP status to be 200 OK")

		// Verify database state: User2 should now be a member
		var count int64
		db.Model(&models.WorkspaceMember{}).Where("workspace_id = ? AND user_id = ?", ws.ID, user2.ID).Count(&count)
		assert.Equal(t, int64(1), count, "Expected user2 to be added as a workspace member")
	})

	// ==========================================
	// SCENARIO 7: REJECT INVITATION
	// ==========================================
	t.Run("Should successfully reject a workspace invitation", func(t *testing.T) {
		resetDB()
		db.Create(&user2)

		ws := models.Workspace{Name: "Shared Wallet", Type: "split", OwnerID: user.ID}
		db.Create(&ws)

		// Seed the invitation with the correct field names
		invitation := models.WorkspaceInvitation{
			WorkspaceID: ws.ID,
			InvitedID:   user2.ID,
			InviterID:   user.ID,
			Status:      "pending",
		}
		db.Create(&invitation)

		targetURL := fmt.Sprintf("/api/v1/workspaces/invitations/%d/reject", invitation.ID)
		req := httptest.NewRequest(http.MethodPost, targetURL, nil)

		ctx := context.WithValue(req.Context(), "user_id", user2.ID)
		req = req.WithContext(ctx)
		req.SetPathValue("id", fmt.Sprintf("%d", invitation.ID))

		w := httptest.NewRecorder()
		wsHandler.RejectInvitation(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected HTTP status to be 200 OK")

		// Verify database state: Invitation status should be rejected
		var updatedInv models.WorkspaceInvitation
		db.First(&updatedInv, invitation.ID)
		assert.Equal(t, "rejected", updatedInv.Status, "Expected invitation status to be 'rejected'")
	})

	// ==========================================
	// SCENARIO 8: GET PENDING INVITATIONS
	// ==========================================
	t.Run("Should successfully retrieve pending invitations for user", func(t *testing.T) {
		resetDB()
		db.Create(&user2)

		ws := models.Workspace{Name: "Shared Wallet", Type: "split", OwnerID: user.ID}
		db.Create(&ws)
		// Seed the pending invitation with the correct field names
		db.Create(&models.WorkspaceInvitation{
			WorkspaceID: ws.ID,
			InvitedID:   user2.ID,
			InviterID:   user.ID,
			Status:      "pending",
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/invitations/pending", nil)

		// user2 is checking their pending invitations
		ctx := context.WithValue(req.Context(), "user_id", user2.ID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		wsHandler.GetPendingInvitations(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected HTTP status to be 200 OK")

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].([]interface{})
		assert.GreaterOrEqual(t, len(data), 1, "Expected to retrieve at least 1 pending invitation")
	})

	// ==========================================
	// SCENARIO 9: GET MEMBERS
	// ==========================================
	t.Run("Should successfully retrieve workspace members", func(t *testing.T) {
		resetDB()
		db.Create(&user2)

		ws := models.Workspace{Name: "Shared Wallet", Type: "split", OwnerID: user.ID}
		db.Create(&ws)
		// Add both users as members
		db.Create(&models.WorkspaceMember{UserID: user.ID, WorkspaceID: ws.ID})
		db.Create(&models.WorkspaceMember{UserID: user2.ID, WorkspaceID: ws.ID})

		targetURL := fmt.Sprintf("/api/v1/workspaces/%d/members", ws.ID)
		req := httptest.NewRequest(http.MethodGet, targetURL, nil)

		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)
		req.SetPathValue("id", fmt.Sprintf("%d", ws.ID))

		w := httptest.NewRecorder()
		wsHandler.GetMembers(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected HTTP status to be 200 OK")

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].([]interface{})
		assert.Equal(t, 2, len(data), "Expected exactly 2 members in the workspace")
	})

	// ==========================================
	// SCENARIO 10: SET TARGET
	// ==========================================
	t.Run("Should successfully set a target for the workspace", func(t *testing.T) {
		resetDB()

		ws := models.Workspace{Name: "My Wallet", Type: "budgeting", OwnerID: user.ID}
		db.Create(&ws)
		db.Create(&models.WorkspaceMember{UserID: user.ID, WorkspaceID: ws.ID})

		// FIX: Adjusted the JSON payload to match SetTargetRequest DTO structure
		payload := fmt.Sprintf(`{
			"workspace_id": %d, 
			"period": "2026-06", 
			"amount_limit": 5000000, 
			"savings_target": 1000000
		}`, ws.ID)

		targetURL := fmt.Sprintf("/api/v1/workspaces/%d/target", ws.ID)
		req := httptest.NewRequest(http.MethodPost, targetURL, strings.NewReader(payload))

		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)
		req.SetPathValue("id", fmt.Sprintf("%d", ws.ID))

		w := httptest.NewRecorder()
		wsHandler.SetTarget(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected HTTP status to be 200 OK")

		var count int64
		db.Model(&models.Target{}).Where("workspace_id = ?", ws.ID).Count(&count)
		assert.Equal(t, int64(1), count, "Expected a target record to be created in the database")
	})

	// ==========================================
	// SCENARIO 11: GET SUMMARY
	// ==========================================
	t.Run("Should successfully retrieve workspace summary", func(t *testing.T) {
		resetDB()

		ws := models.Workspace{Name: "My Wallet", Type: "budgeting", OwnerID: user.ID}
		db.Create(&ws)
		db.Create(&models.WorkspaceMember{UserID: user.ID, WorkspaceID: ws.ID})

		targetURL := fmt.Sprintf("/api/v1/workspaces/%d/summary", ws.ID)
		req := httptest.NewRequest(http.MethodGet, targetURL, nil)

		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)
		req.SetPathValue("id", fmt.Sprintf("%d", ws.ID))

		w := httptest.NewRecorder()
		wsHandler.GetSummary(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Expected HTTP status to be 200 OK")
	})
	// ==========================================
	// SCENARIO 12: NEGATIVE TESTS (SAD PATHS TO INCREASE COVERAGE)
	// ==========================================
	t.Run("Should return 405 when using incorrect HTTP method", func(t *testing.T) {
		// Intentionally using GET on a POST endpoint
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
		w := httptest.NewRecorder()
		wsHandler.Create(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code, "Expected HTTP status to be 405 Method Not Allowed")
	})

	t.Run("Should return 400 when payload is invalid JSON", func(t *testing.T) {
		// Intentionally broken JSON (missing closing quote and bracket)
		payload := `{"name": "Broken JSON`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", strings.NewReader(payload))

		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		wsHandler.Create(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, "Expected HTTP status to be 400 Bad Request")
	})

	t.Run("Should return error when updating a non-existent workspace", func(t *testing.T) {
		payload := `{"name": "Ghost Workspace"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/9999", strings.NewReader(payload))

		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)
		req.SetPathValue("id", "9999") // Intentionally querying a workspace ID that does not exist

		w := httptest.NewRecorder()
		wsHandler.UpdateWorkspace(w, req)

		assert.NotEqual(t, http.StatusOK, w.Code, "Expected an error HTTP status, not 200 OK")
	})

	t.Run("Should return 400 when workspace ID format is completely invalid", func(t *testing.T) {
		// Intentionally sending a string ("invalid-id") instead of a numeric ID
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/invalid-id/members", nil)
		req.SetPathValue("id", "invalid-id")

		w := httptest.NewRecorder()
		wsHandler.GetMembers(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, "Expected HTTP status to be 400 Bad Request due to invalid ID format")
	})

	t.Run("Should return error when accepting a non-existent invitation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/invitations/9999/accept", nil)

		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)
		req.SetPathValue("id", "9999") // Intentionally querying an invitation ID that does not exist

		w := httptest.NewRecorder()
		wsHandler.AcceptInvitation(w, req)

		assert.NotEqual(t, http.StatusOK, w.Code, "Expected an error HTTP status, not 200 OK")
	})

	t.Run("Should return 200 with zero values when getting summary for non-existent workspace", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/9999/summary", nil)

		ctx := context.WithValue(req.Context(), "user_id", user.ID)
		req = req.WithContext(ctx)

		// The handler parses the URL path instead of PathValue, so we construct it exactly
		req.URL.Path = "/api/v1/workspaces/9999/summary"

		w := httptest.NewRecorder()
		wsHandler.GetSummary(w, req)

		// FIX: SQL aggregation on empty records doesn't error, it returns 0. So 200 OK is expected!
		assert.Equal(t, http.StatusOK, w.Code, "Expected HTTP status to be 200 OK with zeroed data")
	})
	// ==========================================
	// SCENARIO 13: EXTREME NEGATIVE TESTS (COVERAGE BOOSTERS)
	// ==========================================
	t.Run("Coverage Boost: Unauthorized and Method Not Allowed across all endpoints", func(t *testing.T) {
		// 1. Create: Unauthorized (No User in Context)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", strings.NewReader(`{"name":"test"}`))
		w := httptest.NewRecorder()
		wsHandler.Create(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "Expected 401 Unauthorized")

		// 2. GetMyWorkspaces: Method Not Allowed
		req = httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", nil) // Should be GET
		w = httptest.NewRecorder()
		wsHandler.GetMyWorkspaces(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

		// 2. GetMyWorkspaces: Unauthorized
		req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
		w = httptest.NewRecorder()
		wsHandler.GetMyWorkspaces(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// 3. UpdateWorkspace: Method Not Allowed
		req = httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/1", nil) // Should be PUT
		w = httptest.NewRecorder()
		wsHandler.UpdateWorkspace(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

		// 4. DeleteWorkspace: Method Not Allowed
		req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/1", nil) // Should be DELETE
		w = httptest.NewRecorder()
		wsHandler.DeleteWorkspace(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

		// 5. Invite: Method Not Allowed
		req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/1/invite", nil) // Should be POST
		w = httptest.NewRecorder()
		wsHandler.Invite(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

		// 7. SetTarget: Unauthorized
		req = httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/target", strings.NewReader(`{}`))
		w = httptest.NewRecorder()
		wsHandler.SetTarget(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// 10. GetPendingInvitations: Method Not Allowed
		req = httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/invitations/pending", nil) // Should be GET
		w = httptest.NewRecorder()
		wsHandler.GetPendingInvitations(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

		// 10. GetPendingInvitations: Unauthorized
		req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/invitations/pending", nil)
		w = httptest.NewRecorder()
		wsHandler.GetPendingInvitations(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Coverage Boost: Broken JSON Payloads and Missing Entities", func(t *testing.T) {
		resetDB()
		ctx := context.WithValue(context.Background(), "user_id", user.ID)

		// 3. UpdateWorkspace: Bad JSON
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/1", strings.NewReader("{ broken_json"))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		wsHandler.UpdateWorkspace(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "Expected 400 for broken JSON")

		// 4. DeleteWorkspace: Usecase Error (Forbidden / Not Found - Deleting ID 9999)
		req = httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/9999", nil)
		req.SetPathValue("id", "9999")
		req = req.WithContext(ctx)
		w = httptest.NewRecorder()
		wsHandler.DeleteWorkspace(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code, "Expected error when deleting non-existent workspace")

		// 5. Invite: Bad JSON
		req = httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/1/invite", strings.NewReader("{ broken"))
		req = req.WithContext(ctx)
		w = httptest.NewRecorder()
		wsHandler.Invite(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 6b. RejectInvitation: Usecase Error (Rejecting ID 9999)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/invitations/9999/reject", nil)
		req.SetPathValue("id", "9999")
		req = req.WithContext(ctx)
		w = httptest.NewRecorder()
		wsHandler.RejectInvitation(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)

		// 7. SetTarget: Bad JSON
		req = httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/target", strings.NewReader("{ broken"))
		req = req.WithContext(ctx)
		w = httptest.NewRecorder()
		wsHandler.SetTarget(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 10. GetPendingInvitations: Empty Response Check
		// User has no invitations, should hit the "response = []map[string]interface{}{}" block
		req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/invitations/pending", nil)
		req = req.WithContext(ctx)
		w = httptest.NewRecorder()
		wsHandler.GetPendingInvitations(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "[]", "Expected empty JSON array in response")
	})
	// ==========================================
	// SCENARIO 14: THE FINAL BOSS (FORCING DB FAILURES)
	// ==========================================
	t.Run("Coverage Boost: Database Connection Crash Simulation", func(t *testing.T) {
		// ULTIMATE HACK: Forcefully close the database connection specifically for this final test.
		// This will force all Repositories to throw errors to the Usecase,
		// and the Usecase to cascade those errors to the Handler (Closing the remaining coverage gap).
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close() // Simulate database connection failure
		}

		ctx := context.WithValue(context.Background(), "user_id", user.ID)

		// 1. Create -> Force SendError(w, err) execution
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", strings.NewReader(`{"name": "test", "type": "budgeting"}`))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		wsHandler.Create(w, req)
		assert.NotEqual(t, http.StatusCreated, w.Code)

		// 2. GetMyWorkspaces -> Force SendError(w, err) execution
		req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
		req = req.WithContext(ctx)
		w = httptest.NewRecorder()
		wsHandler.GetMyWorkspaces(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)

		// 3. Invite -> Force SendError(w, err) execution
		req = httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/1/invite", strings.NewReader(`{"email": "test@test.com"}`))
		req = req.WithContext(ctx)
		req.SetPathValue("id", "1")
		w = httptest.NewRecorder()
		wsHandler.Invite(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)

		// 4. SetTarget -> Force SendError(w, err) execution
		req = httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/target", strings.NewReader(`{"workspace_id": 1, "period": "2026-06", "amount_limit": 100}`))
		req = req.WithContext(ctx)
		w = httptest.NewRecorder()
		wsHandler.SetTarget(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)

		// 5. GetMembers -> Force SendError(w, err) execution
		req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/1/members", nil)
		req = req.WithContext(ctx)
		req.SetPathValue("id", "1")
		w = httptest.NewRecorder()
		wsHandler.GetMembers(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)

		// 6. GetSummary -> Force custom 500 error block execution
		req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/1/summary", nil)
		req = req.WithContext(ctx)
		req.URL.Path = "/api/v1/workspaces/1/summary"
		w = httptest.NewRecorder()
		wsHandler.GetSummary(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		// 7. GetPendingInvitations -> Force SendError(w, err) execution
		req = httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/invitations/pending", nil)
		req = req.WithContext(ctx)
		w = httptest.NewRecorder()
		wsHandler.GetPendingInvitations(w, req)
		assert.NotEqual(t, http.StatusOK, w.Code)
	})
}

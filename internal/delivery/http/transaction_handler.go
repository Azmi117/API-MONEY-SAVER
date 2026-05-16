package http

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/utils"
	"github.com/jaytaylor/html2text"
)

type TransactionHandler struct {
	usecase        usecase.TransactionUsecase
	iUsecase       usecase.IntegrationUsecase
	pendingUsecase usecase.PendingUsecase
	debtUsecase    usecase.DebtUsecase
}

func NewTransactionHandler(
	u usecase.TransactionUsecase,
	iu usecase.IntegrationUsecase,
	pu usecase.PendingUsecase,
	du usecase.DebtUsecase,
) *TransactionHandler {
	return &TransactionHandler{
		usecase:        u,
		iUsecase:       iu,
		pendingUsecase: pu,
		debtUsecase:    du,
	}
}

// 1. POST /transactions/manual
func (h *TransactionHandler) CreateManual(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, please use POST"))
		return
	}

	var req dto.CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, apperror.BadRequest("Invalid request payload"))
		return
	}

	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	tx, _, err := h.usecase.CreateManual(r.Context(), userID, req)
	if err != nil {
		SendError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	// FIX: Pake anonymous struct biar urutan JSON ke-kunci dari atas ke bawah
	response := struct {
		StatusCode int         `json:"status_code"`
		Message    string      `json:"message"`
		Data       interface{} `json:"data"`
	}{
		StatusCode: http.StatusCreated,
		Message:    "Manual transaction recorded successfully",
		Data:       tx,
	}

	json.NewEncoder(w).Encode(response)
}

// 2. GET /transactions/history
func (h *TransactionHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, please use GET"))
		return
	}

	// FIX: Ambil dari Path Param URL, bukan Query
	idStr := r.PathValue("id")
	workspaceID, _ := strconv.Atoi(idStr)

	if workspaceID == 0 {
		SendError(w, apperror.BadRequest("Invalid workspace ID"))
		return
	}

	history, err := h.usecase.GetHistory(uint(workspaceID))
	if err != nil {
		SendError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := struct {
		StatusCode int         `json:"status_code"`
		Message    string      `json:"message"`
		Data       interface{} `json:"data"`
	}{
		StatusCode: http.StatusOK,
		Message:    "Transaction history retrieved successfully",
		Data:       history,
	}

	json.NewEncoder(w).Encode(response)
}

// 3. DELETE /transactions
// 3. DELETE /transactions/{id}
// 3. DELETE /transactions/{id}
func (h *TransactionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, please use DELETE"))
		return
	}

	// Ambil ID Transaksi dari Path Parameter
	idStr := r.PathValue("id")
	id, _ := strconv.Atoi(idStr)

	if id == 0 {
		SendError(w, apperror.BadRequest("Invalid transaction ID"))
		return
	}

	// FIX: Ambil userID dari context (hasil dari middleware authMW)
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	// FIX: Pass context, transactionID, dan userID ke Usecase
	err := h.usecase.DeleteTransaction(r.Context(), uint(id), userID)
	if err != nil {
		SendError(w, err)
		return
	}

	// Standarisasi Response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := struct {
		StatusCode int         `json:"status_code"`
		Message    string      `json:"message"`
		Data       interface{} `json:"data"`
	}{
		StatusCode: http.StatusOK,
		Message:    "Transaction deleted successfully",
		Data:       nil,
	}

	json.NewEncoder(w).Encode(response)
}

// 4. POST /webhooks/email-mandiri
func (h *TransactionHandler) EmailMandiriWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, please use POST"))
		return
	}

	if r.Header.Get("X-Webhook-Secret") != os.Getenv("WEBHOOK_SECRET") {
		SendError(w, apperror.Unauthorized("Invalid webhook secret key"))
		return
	}

	var payload struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		SendError(w, apperror.BadRequest("Failed to decode email payload"))
		return
	}

	msg, _ := mail.ReadMessage(strings.NewReader(payload.Body))
	var finalBody string

	// Simple check instead of complex multipart for brevity, adjust if needed
	bodyBytes, _ := io.ReadAll(msg.Body)
	finalBody = string(bodyBytes)

	plainBody, _ := html2text.FromString(finalBody)
	plainBody = strings.TrimSpace(plainBody)

	tx, err := h.iUsecase.ProcessEmailMandiri(r.Context(), 1, 1, payload.Subject, plainBody)
	if err != nil {
		log.Printf("[Webhook Error] Parsing failed: %v", err)
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Email processed successfully", tx)
}

// 5. PATCH /transactions/{id}/confirm
func (h *TransactionHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, please use PATCH"))
		return
	}

	idStr := r.PathValue("id")
	id, _ := strconv.Atoi(idStr)

	if id == 0 {
		SendError(w, apperror.BadRequest("Invalid transaction ID"))
		return
	}

	// FIX: Tangkep variabel tx (data transaksi asli), dan buang budgetData pake "_"
	tx, _, err := h.usecase.ConfirmTransaction(r.Context(), uint(id))
	if err != nil {
		SendError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// FIX: Response jadi standar, Data isinya detail transaksi yang barusan di-ACC
	response := struct {
		StatusCode int         `json:"status_code"`
		Message    string      `json:"message"`
		Data       interface{} `json:"data"`
	}{
		StatusCode: http.StatusOK,
		Message:    "Transaction confirmed successfully",
		Data:       tx,
	}

	json.NewEncoder(w).Encode(response)
}

// 6. POST /transactions/scan-hybrid2
func (h *TransactionHandler) ScanReceiptHybrid(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		SendError(w, apperror.BadRequest("File size exceeds 5MB limit"))
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		SendError(w, apperror.BadRequest("Receipt image is required"))
		return
	}
	defer file.Close()

	imgData, _ := io.ReadAll(file)
	wsIDStr := r.FormValue("workspace_id")
	wsID, _ := strconv.Atoi(wsIDStr)

	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	result, pendingID, err := h.usecase.ProcessScanHybrid2(r.Context(), userID, uint(wsID), imgData, header.Header.Get("Content-Type"))
	if err != nil {
		SendError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := struct {
		StatusCode int         `json:"status_code"`
		Message    string      `json:"message"`
		Data       interface{} `json:"data"`
		PendingID  uint        `json:"pending_id"`
	}{
		StatusCode: http.StatusOK,
		Message:    "Receipt scanned successfully (Pending)",
		Data:       result,
		PendingID:  pendingID,
	}

	json.NewEncoder(w).Encode(response)
}

// 7. GET /emails/pending
func (h *TransactionHandler) GetPendingEmails(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	logs, err := h.pendingUsecase.GetPendingEmailLogs(userID)
	if err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Pending email logs retrieved successfully", logs)
}

// 8. POST /emails/{id}/approve
func (h *TransactionHandler) ApproveEmail(w http.ResponseWriter, r *http.Request) {
	logIDStr := r.PathValue("id")
	logID, _ := strconv.ParseUint(logIDStr, 10, 32)
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	var body struct {
		WorkspaceID uint `json:"workspace_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		SendError(w, apperror.BadRequest("Invalid request format"))
		return
	}

	confirmReq := dto.ConfirmEmailRequest{
		EmailParsedID: uint(logID),
		WorkspaceID:   body.WorkspaceID,
	}

	// FIX: ConfirmEmailTransaction returns (*dto.BudgetStatusResponse, error)
	notification, err := h.pendingUsecase.ConfirmEmailTransaction(r.Context(), userID, confirmReq)
	if err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Email approved and transaction created", map[string]interface{}{
		"budget_status": notification,
	})
}

// 9. POST /emails/{id}/reject
func (h *TransactionHandler) RejectEmail(w http.ResponseWriter, r *http.Request) {
	logIDStr := r.PathValue("id")
	logID, _ := strconv.ParseUint(logIDStr, 10, 32)
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	err := h.pendingUsecase.RejectEmailLog(r.Context(), uint(logID), userID)
	if err != nil {
		SendError(w, err)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, "success", "Email log rejected successfully", nil)
}

// 10. POST /transactions/scan-alt
func (h *TransactionHandler) ScanAlternative(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		SendError(w, apperror.BadRequest("File size exceeds 10MB limit"))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		SendError(w, apperror.BadRequest("Receipt file is required"))
		return
	}
	defer file.Close()

	workspaceIDStr := r.FormValue("workspace_id")
	wID, _ := strconv.ParseUint(workspaceIDStr, 10, 32)

	filePath := fmt.Sprintf("uploads/%d_%s", userID, header.Filename)
	dst, _ := os.Create(filePath)
	io.Copy(dst, file)
	dst.Close()
	defer os.Remove(filePath)

	result, pendingID, err := h.usecase.ProcessScanAlternative(r.Context(), filePath, userID, uint(wID))
	if err != nil {
		SendError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// FIX: Pake anonymous struct biar gak dobel data dan pake status_code
	response := struct {
		StatusCode int         `json:"status_code"`
		Message    string      `json:"message"`
		Data       interface{} `json:"data"`
		PendingID  uint        `json:"pending_id"`
	}{
		StatusCode: http.StatusOK,
		Message:    "Scan successful, pending confirmation",
		Data:       result,
		PendingID:  pendingID,
	}

	json.NewEncoder(w).Encode(response)
}

// 11. POST /transactions/scan-alt/confirm
func (h *TransactionHandler) ConfirmScan(w http.ResponseWriter, r *http.Request) {
	var req dto.ConfirmTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, apperror.BadRequest("Invalid confirmation payload"))
		return
	}

	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Invalid user session"))
		return
	}

	// FIX 1: Parse date pake RFC3339 karena JSON dari ocr_space formatnya ada "T" dan "Z"
	parsedDate, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		// Fallback kalau ternyata formatnya pendek
		parsedDate, _ = time.Parse("2006-01-02", req.Date)
	}

	var modelItems []models.TransactionItem
	for _, item := range req.Items {
		modelItems = append(modelItems, models.TransactionItem{
			Description: item.Description,
			Price:       item.Price,
			Quantity:    item.Quantity,
			Total:       item.Price * float64(item.Quantity),
		})
	}

	transaction := models.Transaction{
		UserID:           userID,
		WorkspaceID:      req.WorkspaceID,
		Merchant:         req.Merchant,
		Amount:           req.Amount,
		Date:             parsedDate,
		Type:             req.Type,
		CategoryID:       req.CategoryID,
		Note:             req.Note,
		Status:           "approved",
		Source:           "ocr_space",
		TransactionItems: modelItems,
		// FIX 2: Ini yang tadi kelupaan makanya meledak!
		Method:  req.Method,
		GmailID: req.GmailID,
	}

	// FIX 3: Tangkep err doang, budgetStatus buang aja karena gak dipake di response Web/API
	_, err = h.usecase.ConfirmScanTransaction(r.Context(), &transaction, modelItems)
	if err != nil {
		SendError(w, err)
		return
	}

	// FIX 4: Response dibikin elegan dan konsisten
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	response := struct {
		StatusCode int         `json:"status_code"`
		Message    string      `json:"message"`
		Data       interface{} `json:"data"`
	}{
		StatusCode: http.StatusCreated,
		Message:    "Transaction confirmed and saved",
		Data:       transaction,
	}

	json.NewEncoder(w).Encode(response)
}

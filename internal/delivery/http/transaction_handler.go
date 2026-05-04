package http

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
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
	"github.com/jaytaylor/html2text"
)

type TransactionHandler struct {
	usecase usecase.TransactionUsecase
}

func NewTransactionHandler(u usecase.TransactionUsecase) *TransactionHandler {
	return &TransactionHandler{u}
}

// 1. POST /transactions/manual
func (h *TransactionHandler) CreateManual(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use POST!"))
		return
	}

	var req dto.CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, apperror.BadRequest("Invalid payload!"))
		return
	}

	// Sesuai diskusi, userID nanti diambil dari middleware JWT
	userID := uint(1)

	err := h.usecase.CreateManual(r.Context(), userID, req)
	if err != nil {
		SendError(w, apperror.Internal(err.Error()))
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Success record manual transaction"})
}

// 2. GET /transactions/history?workspace_id=1
func (h *TransactionHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use GET!"))
		return
	}

	workspaceIDStr := r.URL.Query().Get("workspace_id")
	workspaceID, _ := strconv.Atoi(workspaceIDStr)

	history, err := h.usecase.GetHistory(uint(workspaceID))
	if err != nil {
		SendError(w, apperror.Internal(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// 3. DELETE /transactions/delete?id=1
func (h *TransactionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use DELETE!"))
		return
	}

	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)

	err := h.usecase.DeleteTransaction(uint(id))
	if err != nil {
		SendError(w, apperror.Internal(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Transaksi berhasil dihapus"})
}

// 4. POST /webhooks/email-mandiri (Untuk Cloudflare)
func (h *TransactionHandler) EmailMandiriWebhook(w http.ResponseWriter, r *http.Request) {
	// 1. Validasi Method
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use POST!"))
		return
	}

	// 2. Validasi Secret (Samain sama di Cloudflare & .env)
	if r.Header.Get("X-Webhook-Secret") != os.Getenv("WEBHOOK_SECRET") {
		SendError(w, apperror.Unauthorized("Invalid Secret Key"))
		return
	}

	// 3. Decode Payload dari Cloudflare Worker
	var payload struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("[Webhook Error] Gagal decode JSON: %v", err)
		SendError(w, apperror.BadRequest("Failed decode Email!"))
		return
	}

	// 4. PARSING RAW EMAIL (MIME Handling)
	// Cloudflare ngirim format RFC822, kita harus buang header metadata-nya
	msg, err := mail.ReadMessage(strings.NewReader(payload.Body))
	var finalBody string

	if err != nil {
		log.Printf("[Webhook Warning] Not RFC822 format, use raw body: %v", err)
		finalBody = payload.Body
	} else {
		// Cek apakah emailnya multipart (ada HTML + Plain Text)
		mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
		if err == nil && strings.HasPrefix(mediaType, "multipart/") {
			mr := multipart.NewReader(msg.Body, params["boundary"])
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					break
				}
				// Prioritaskan ambil yang text/plain atau text/html
				slurp, _ := io.ReadAll(p)
				finalBody += string(slurp)
			}
		} else {
			// Kalau email simple, langsung baca body-nya
			bodyBytes, _ := io.ReadAll(msg.Body)
			finalBody = string(bodyBytes)
		}
	}

	// 5. Ubah HTML ke Text & Bersihin Spasi
	plainBody, _ := html2text.FromString(finalBody)
	plainBody = strings.TrimSpace(plainBody)

	// 6. Jalankan Usecase
	// Logika parsing Mandiri lu ada di dalem sini
	tx, err := h.usecase.ProcessEmailMandiri(r.Context(), 1, 1, payload.Subject, plainBody)
	if err != nil {
		// Log biar ketauan di terminal Go lu kalau parsing gagal
		log.Printf("[Webhook Error] Usecase Gagal: %v", err)
		log.Printf("[Webhook Debug] Plain Body yg bikin gagal: %s", plainBody)

		SendError(w, apperror.Internal(err.Error()))
		return
	}

	// 7. Response Sukses
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tx)
}

// 5. POST /transactions/scan
func (h *TransactionHandler) ScanReceipt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use POST!"))
		return
	}

	// Limit upload 5MB
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		SendError(w, apperror.BadRequest("File size is bigger than 5MB, reduce file size!"))
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		SendError(w, apperror.BadRequest("Image not found!"))
		return
	}
	defer file.Close()

	// Cek MIME Type
	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		SendError(w, apperror.BadRequest("File must be an image!"))
		return
	}

	imgData, err := io.ReadAll(file)
	if err != nil {
		SendError(w, apperror.Internal("Failed load image!"))
		return
	}

	wsID, _ := strconv.Atoi(r.FormValue("workspace_id"))
	userID := uint(1)

	tx, err := h.usecase.ProcessScan(r.Context(), userID, uint(wsID), imgData, contentType)
	if err != nil {
		// Balikin JSON error manual biar frontend gampang baca
		SendError(w, apperror.Internal(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Success scan receipt!",
		"data":    tx,
	})
}

// 6. PATCH /transactions/confirm?id=1
func (h *TransactionHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use PATCH!"))
		return
	}

	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)

	err := h.usecase.ConfirmTransaction(r.Context(), uint(id))
	if err != nil {
		SendError(w, apperror.Internal(err.Error()))
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Success confirm transaction"})
}

func (h *TransactionHandler) ScanReceiptHybrid(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, apperror.MethodNotAllowed("Method not allowed, use POST!"))
		return
	}

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		SendError(w, apperror.BadRequest("File size is bigger than 5MB, reduce file size!"))
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		SendError(w, apperror.BadRequest("Image not found!"))
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		SendError(w, apperror.BadRequest("File must be an image!"))
		return
	}

	imgData, err := io.ReadAll(file)
	if err != nil {
		SendError(w, apperror.Internal("Failed load image!"))
		return
	}

	wsID, _ := strconv.Atoi(r.FormValue("workspace_id"))

	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		SendError(w, apperror.Unauthorized("Unauthorized"))
		return
	}

	result, err := h.usecase.ProcessScanHybrid2(r.Context(), userID, uint(wsID), imgData, contentType)
	if err != nil {
		SendError(w, apperror.Internal(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Success scan receipt (hybrid)",
		"data":    result.Transaction,
		"meta": map[string]interface{}{
			"engine":        result.Engine,
			"confidence":    result.Confidence,
			"fallback_used": result.FallbackUsed,
		},
	})
}

// 1. GET PENDING EMAILS
func (h *TransactionHandler) GetPendingEmails(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(uint)

	// Ambil list dari usecase (nanti kita tambahin methodnya di usecase)
	logs, err := h.usecase.GetPendingEmailLogs(userID)
	if err != nil {
		SendError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// 2. APPROVE EMAIL LOG
func (h *TransactionHandler) ApproveEmail(w http.ResponseWriter, r *http.Request) {
	logIDStr := r.PathValue("id")
	logID, _ := strconv.ParseUint(logIDStr, 10, 32)
	userID := r.Context().Value("user_id").(uint)

	err := h.usecase.ApproveEmailLog(r.Context(), uint(logID), userID)
	if err != nil {
		SendError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Email approved and transaction created!"})
}

// 3. REJECT EMAIL LOG
func (h *TransactionHandler) RejectEmail(w http.ResponseWriter, r *http.Request) {
	logIDStr := r.PathValue("id")
	logID, _ := strconv.ParseUint(logIDStr, 10, 32)
	userID := r.Context().Value("user_id").(uint)

	err := h.usecase.RejectEmailLog(r.Context(), uint(logID), userID)
	if err != nil {
		SendError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Email log rejected successfully"})
}

func (h *TransactionHandler) ScanAlternative(w http.ResponseWriter, r *http.Request) {
	fmt.Println("📩 [Handler] Masuk ke ScanAlternative")

	// 1. Ambil user_id dari context (Middleware Auth)
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		fmt.Println("❌ [Handler] User ID not found in context or wrong type")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Parse Multipart Form (Max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		fmt.Println("❌ [Handler] Parse Form Error:", err)
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	// 3. Ambil file struk
	file, header, err := r.FormFile("file")
	if err != nil {
		fmt.Println("❌ [Handler] FormFile Error:", err)
		http.Error(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 4. Ambil workspace_id dari form
	workspaceIDStr := r.FormValue("workspace_id")
	wID, err := strconv.ParseUint(workspaceIDStr, 10, 32)
	if err != nil {
		fmt.Println("❌ [Handler] Invalid Workspace ID:", workspaceIDStr)
		http.Error(w, "Invalid workspace_id", http.StatusBadRequest)
		return
	}
	workspaceID := uint(wID)

	// 5. Simpan file sementara secara lokal
	filePath := fmt.Sprintf("uploads/%d_%s", userID, header.Filename)
	dst, err := os.Create(filePath)
	if err != nil {
		fmt.Println("❌ [Handler] Create File Error:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	io.Copy(dst, file)
	defer os.Remove(filePath) // Bersihkan file setelah proses selesai

	fmt.Println("📡 [Handler] Calling Usecase Alternative...")

	// 6. Eksekusi Usecase (Menampung 3 return value: result, pendingID, err)
	// Gunakan r.Context() untuk ctx, dan filePath untuk imagePath
	result, pendingID, err := h.usecase.ProcessScanAlternative(r.Context(), filePath, userID, workspaceID)
	if err != nil {
		fmt.Println("❌ [Handler] Usecase Error:", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 7. Kirim Response sukses
	// Kita bisa tambahkan PendingID ke dalam response kalau front-end butuh
	fmt.Printf("✅ [Handler] Scan Success! Pending ID: %d\n", pendingID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":       result,
		"pending_id": pendingID,
		"message":    "Scan successful, please confirm to save.",
	})
}

func (h *TransactionHandler) ConfirmScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req dto.ConfirmTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Ambil userID dari context (asumsi lu set di middleware auth lu)
	userID, ok := r.Context().Value("user_id").(uint)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse tanggal dari string YYYY-MM-DD
	parsedDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		http.Error(w, "Invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	// Mapping ke model Transaction
	transaction := models.Transaction{
		UserID:           userID,
		WorkspaceID:      req.WorkspaceID,
		Merchant:         req.Merchant,
		Amount:           req.Amount,
		Date:             parsedDate,
		Type:             req.Type,
		CategoryID:       req.CategoryID,
		Note:             req.Note,
		Status:           "approved", // Review selesai
		Source:           "ocr_space_pure",
		TransactionItems: req.Items, // Pasangkan list items
	}

	// Panggil usecase ConfirmScanTransaction yang udah lu buat tadi
	err = h.usecase.ConfirmScanTransaction(r.Context(), transaction, req.Items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Transaction confirmed and saved successfully",
	})
}

func (h *TransactionHandler) ConfirmScanAlternative(w http.ResponseWriter, r *http.Request) {
	var request struct {
		PendingID uint `json:"pending_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Panggil usecase confirm yang tadi kita buat
	err := h.usecase.ConfirmPendingTransaction(r.Context(), request.PendingID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Transaction confirmed and saved successfully"})
}

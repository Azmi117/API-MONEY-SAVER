package http

import (
	"encoding/json"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"strings"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req dto.CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload tidak valid", http.StatusBadRequest)
		return
	}

	// Sesuai diskusi, userID nanti diambil dari middleware JWT
	userID := uint(1)

	err := h.usecase.CreateManual(r.Context(), userID, req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Transaksi manual berhasil dicatat"})
}

// 2. GET /transactions/history?workspace_id=1
func (h *TransactionHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workspaceIDStr := r.URL.Query().Get("workspace_id")
	workspaceID, _ := strconv.Atoi(workspaceIDStr)

	history, err := h.usecase.GetHistory(uint(workspaceID))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// 3. DELETE /transactions/delete?id=1
func (h *TransactionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)

	err := h.usecase.DeleteTransaction(uint(id))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Transaksi berhasil dihapus"})
}

// 4. POST /webhooks/email-mandiri (Untuk Cloudflare)
func (h *TransactionHandler) EmailMandiriWebhook(w http.ResponseWriter, r *http.Request) {
	// 1. Validasi Method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2. Validasi Secret (Samain sama di Cloudflare & .env)
	if r.Header.Get("X-Webhook-Secret") != os.Getenv("WEBHOOK_SECRET") {
		http.Error(w, "Unauthorized: Secret key salah", http.StatusUnauthorized)
		return
	}

	// 3. Decode Payload dari Cloudflare Worker
	var payload struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("[Webhook Error] Gagal decode JSON: %v", err)
		http.Error(w, "Gagal decode email", http.StatusBadRequest)
		return
	}

	// 4. PARSING RAW EMAIL (MIME Handling)
	// Cloudflare ngirim format RFC822, kita harus buang header metadata-nya
	msg, err := mail.ReadMessage(strings.NewReader(payload.Body))
	var finalBody string

	if err != nil {
		log.Printf("[Webhook Warning] Bukan format RFC822, pake body mentah: %v", err)
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

		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit upload size (misal 5MB)
	r.ParseMultipartForm(5 << 20)

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Gambar tidak ditemukan", http.StatusBadRequest)
		return
	}
	defer file.Close()

	imgData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Gagal membaca gambar", http.StatusInternalServerError)
		return
	}

	// Ambil Workspace ID dari form field
	wsID, _ := strconv.Atoi(r.FormValue("workspace_id"))
	userID := uint(1) // Placeholder

	tx, err := h.usecase.ProcessScan(r.Context(), userID, uint(wsID), imgData, header.Header.Get("Content-Type"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Struk berhasil di-scan, silakan konfirmasi",
		"data":    tx,
	})
}

// 6. PATCH /transactions/confirm?id=1
func (h *TransactionHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)

	err := h.usecase.ConfirmTransaction(r.Context(), uint(id))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Transaksi berhasil dikonfirmasi"})
}

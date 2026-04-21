package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GeminiClient struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

type TransactionItemAI struct {
	Description string  `json:"description"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Total       float64 `json:"total"`
}

// ResultScan ini buat nampung balikan dari AI biar rapi
type ResultScan struct {
	Amount   float64             `json:"amount"`
	Merchant string              `json:"merchant"`
	Date     string              `json:"date"` // Format: YYYY-MM-DD HH:mm:ss
	Type     string              `json:"type"`
	Method   string              `json:"method"` // NEW: Cash, Debit, QRIS, dll
	Note     string              `json:"note"`
	Items    []TransactionItemAI `json:"items"`
	Total    float64             `json:"total"`
}

func NewGeminiClient(ctx context.Context, apiKey string) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}

	// Sesuai hasil list models tadi, kita pakai versi 2.5 Flash yang paling gacor
	model := client.GenerativeModel("models/gemini-2.5-flash")

	return &GeminiClient{client: client, model: model}, nil
}

func (g *GeminiClient) ScanReceipt(ctx context.Context, imgData []byte, mimeType string) (*ResultScan, error) {
	// 1. DEBUG: Liat aslinya apa sebelum dibersihin
	cleanType := "jpeg" // default
	if strings.Contains(mimeType, "png") {
		cleanType = "png"
	} else if strings.Contains(mimeType, "pdf") {
		cleanType = "pdf"
	}

	// Kita pake variabel cleanType ini buat dikirim ke Google
	fmt.Printf("📂 [Gemini] Hack MIME Type: image/%s (sent as %s)\n", cleanType, cleanType)

	fmt.Println("📸 [Gemini] Mulai proses ScanReceipt...")

	prompt := genai.Text("Tolong baca struk ini. Berikan jawaban HANYA dalam format JSON: " +
		"{\"amount\": float, \"merchant\": string, \"date\": \"YYYY-MM-DD HH:mm:ss\", \"type\": \"expense\", \"method\": \"string\", \"note\": \"string\", " +
		"\"items\": [{\"description\": string, \"quantity\": int, \"unit_price\": float, \"total\": float}]}. " +
		"Field 'method' diisi cara bayarnya (Cash/Debit/QRIS). Field 'note' isi ringkasan singkat.")

	// DISINI KUNCINYA: Jangan kirim "image/jpeg", kirim "jpeg" aja!
	data := genai.ImageData(cleanType, imgData)

	fmt.Println("📡 [Gemini] Mengirim gambar ke Google AI Studio...")
	resp, err := g.model.GenerateContent(ctx, prompt, data)
	if err != nil {
		fmt.Printf("❌ [Gemini] API Error: %v\n", err)
		return nil, err
	}

	fmt.Println("✅ [Gemini] Respon berhasil diterima!")

	// Validasi dasar
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		fmt.Println("⚠️ [Gemini] Respon kosong (no candidates)")
		return nil, fmt.Errorf("AI tidak memberikan jawaban")
	}

	var result ResultScan
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			rawText := string(txt)
			// fmt.Printf("📝 [Gemini] Raw AI Response:\n%s\n", rawText)

			cleanJSON := formatJSON(rawText)
			// fmt.Printf("🧹 [Gemini] Cleaned JSON: %s\n", cleanJSON)

			err := json.Unmarshal([]byte(cleanJSON), &result)
			if err != nil {
				fmt.Printf("❌ [Gemini] Gagal Unmarshal JSON: %v\n", err)
				return nil, fmt.Errorf("gagal parsing JSON AI: %v", err)
			}

			// --- LOGIC TAMBAHAN DISINI ---
			// Kalau amount kosong tapi ada field total, kita ambil totalnya
			if result.Amount == 0 && result.Total != 0 {
				result.Amount = result.Total
			}
			// -----------------------------

			fmt.Printf("✨ [Gemini] Berhasil Ekstraksi: Merchant=%s, Amount=%.2f\n", result.Merchant, result.Amount)
			return &result, nil
		}
	}

	fmt.Println("⚠️ [Gemini] Tidak ditemukan teks dalam respon")
	return nil, fmt.Errorf("AI tidak memberikan jawaban dalam format teks")
}

// Struct khusus untuk respon Hybrid
type OCRFixResponse struct {
	Merchant    string  `json:"merchant"`
	TotalAmount float64 `json:"total_amount"`
}

func (g *GeminiClient) FixAndParseOCR(ctx context.Context, rawText string) (*OCRFixResponse, error) {
	// Pake 2.0 Flash biar ngebut
	model := g.client.GenerativeModel("models/gemini-2.5-flash")

	prompt := fmt.Sprintf(`
		Analyze this messy OCR text from an Indonesian receipt.
		Fix typos (e.g. '0' to 'O', 'I' to '1') and extract:
		1. Merchant Name
		2. Total Amount (the final price paid)

		OCR TEXT:
		"""%s"""

		Return ONLY JSON:
		{"merchant": "string", "total_amount": number}
	`, rawText)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, err
	}

	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		for _, part := range resp.Candidates[0].Content.Parts {
			if txt, ok := part.(genai.Text); ok {
				// Gunakan fungsi formatJSON yang udah lo punya buat bersihin backticks
				cleanJSON := formatJSON(string(txt))
				var result OCRFixResponse
				if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
					return nil, err
				}
				return &result, nil
			}
		}
	}
	return nil, fmt.Errorf("AI empty response")
}

// Helper buat bersihin backticks ```json dari AI
func formatJSON(raw string) string {
	res := strings.ReplaceAll(raw, "```json", "")
	res = strings.ReplaceAll(res, "```", "")
	return strings.TrimSpace(res)
}

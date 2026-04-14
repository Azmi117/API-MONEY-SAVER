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

// ResultScan ini buat nampung balikan dari AI biar rapi
type ResultScan struct {
	Amount   float64 `json:"amount"`
	Merchant string  `json:"merchant"`
	Date     string  `json:"date"` // Format: YYYY-MM-DD HH:mm:ss
	Type     string  `json:"type"`
}

func NewGeminiClient(ctx context.Context, apiKey string) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}

	// Kita pake 1.5 Flash karena murah dan kenceng buat struk
	model := client.GenerativeModel("gemini-1.5-flash")
	return &GeminiClient{client: client, model: model}, nil
}

func (g *GeminiClient) ScanReceipt(ctx context.Context, imgData []byte, mimeType string) (*ResultScan, error) {
	prompt := genai.Text("Tolong baca struk ini. Berikan jawaban HANYA dalam format JSON: " +
		"{\"amount\": float, \"merchant\": string, \"date\": \"YYYY-MM-DD HH:mm:ss\", \"type\": \"expense\"}. " +
		"Jika tanggal tidak ada jamnya, set jam ke 12:00:00. Jika tidak yakin, berikan tebakan terbaik.")

	data := genai.ImageData(mimeType, imgData)

	resp, err := g.model.GenerateContent(ctx, prompt, data)
	if err != nil {
		return nil, err
	}

	// Ambil teks dari respon AI
	var result ResultScan
	for _, cand := range resp.Candidates {
		for _, part := range cand.Content.Parts {
			if txt, ok := part.(genai.Text); ok {
				// Bersihin markdown ```json ... ``` kalau ada
				cleanJSON := formatJSON(string(txt))
				err := json.Unmarshal([]byte(cleanJSON), &result)
				if err != nil {
					return nil, fmt.Errorf("gagal parsing JSON AI: %v", err)
				}
				return &result, nil
			}
		}
	}

	return nil, fmt.Errorf("AI tidak memberikan jawaban")
}

// Helper buat bersihin backticks ```json dari AI
func formatJSON(raw string) string {
	res := strings.ReplaceAll(raw, "```json", "")
	res = strings.ReplaceAll(res, "```", "")
	return strings.TrimSpace(res)
}

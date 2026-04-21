package dto

import "github.com/Azmi117/API-MONEY-SAVER.git/internal/models"

type ProcessScanHybridResult struct {
	Transaction  *models.Transaction `json:"transaction"`
	Engine       string              `json:"engine"`
	Confidence   int                 `json:"confidence"`
	FallbackUsed bool                `json:"fallback_used"`
}

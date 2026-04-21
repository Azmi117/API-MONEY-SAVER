package ocr

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/gemini"
)

var (
	moneyRegex         = regexp.MustCompile(`(\d{1,3}(?:[.,]\d{3})+|\d{4,})`)
	itemLineRegex      = regexp.MustCompile(`^\s*(\d+)\s+(.+?)\s+(\d{1,3}(?:[.,]\d{3})+|\d{4,})\s*$`)
	merchantCleanRegex = regexp.MustCompile(`[^a-zA-Z0-9&\-. ]+`)
)

type RawTextExtractor interface {
	ExtractRawText(ctx context.Context, imgData []byte, mimeType string) (string, error)
}

type HybridScanResult struct {
	Amount       float64                    `json:"amount"`
	Merchant     string                     `json:"merchant"`
	Date         string                     `json:"date"`
	Type         string                     `json:"type"`
	Method       string                     `json:"method"`
	Note         string                     `json:"note"`
	Items        []gemini.TransactionItemAI `json:"items"`
	Confidence   int                        `json:"confidence"`
	Engine       string                     `json:"engine"`
	Source       string                     `json:"source"`
	FallbackUsed bool                       `json:"fallback_used"`
	RawText      string                     `json:"raw_text,omitempty"`
}

type HybridScanner struct {
	extractor     RawTextExtractor
	geminiClient  *gemini.GeminiClient
	minConfidence int
}

func NewHybridScanner(extractor RawTextExtractor, geminiClient *gemini.GeminiClient) *HybridScanner {
	return &HybridScanner{
		extractor:     extractor,
		geminiClient:  geminiClient,
		minConfidence: 65,
	}
}

func (h *HybridScanner) ScanReceiptHybrid(ctx context.Context, imgData []byte, mimeType string) (*HybridScanResult, error) {
	rawText, ocrErr := h.extractor.ExtractRawText(ctx, imgData, mimeType)

	var parsedOCR *HybridScanResult
	var shouldFallback bool

	if ocrErr == nil && strings.TrimSpace(rawText) != "" {
		parsedOCR, shouldFallback = parseReceiptRawText(rawText)

		if parsedOCR != nil && !shouldFallback && parsedOCR.isReliable(h.minConfidence) {
			parsedOCR.Engine = "tesseract_rules"
			parsedOCR.Source = "scan_hybrid_ocr"
			parsedOCR.FallbackUsed = false
			return parsedOCR, nil
		}
	}

	aiResult, aiErr := h.geminiClient.ScanReceipt(ctx, imgData, mimeType)
	if aiErr != nil {
		if parsedOCR != nil && parsedOCR.Amount > 0 {
			parsedOCR.Engine = "tesseract_low_confidence"
			parsedOCR.Source = "scan_hybrid_ocr"
			parsedOCR.FallbackUsed = false
			return parsedOCR, nil
		}

		return nil, fmt.Errorf("hybrid scan gagal, ocr error: %v, gemini error: %v", ocrErr, aiErr)
	}

	amount := aiResult.Amount
	if amount == 0 && aiResult.Total != 0 {
		amount = aiResult.Total
	}

	return &HybridScanResult{
		Amount:       amount,
		Merchant:     strings.TrimSpace(aiResult.Merchant),
		Date:         strings.TrimSpace(aiResult.Date),
		Type:         defaultString(aiResult.Type, "expense"),
		Method:       strings.TrimSpace(aiResult.Method),
		Note:         strings.TrimSpace(aiResult.Note),
		Items:        aiResult.Items,
		Confidence:   95,
		Engine:       "gemini_fallback",
		Source:       "scan_hybrid_ai",
		FallbackUsed: true,
		RawText:      rawText,
	}, nil
}

func (r *HybridScanResult) isReliable(minConfidence int) bool {
	if r == nil {
		return false
	}
	if r.Amount <= 0 {
		return false
	}
	if strings.TrimSpace(r.Merchant) == "" || strings.EqualFold(strings.TrimSpace(r.Merchant), "unknown merchant") {
		return false
	}
	return r.Confidence >= minConfidence
}

func parseReceiptRawText(raw string) (*HybridScanResult, bool) {
	lines := normalizeLines(raw)

	merchant := extractMerchant(lines)
	subtotal, service, tax, totalBill, grandTotal := extractReceiptTotals(lines)
	amount, totalReliable := chooseBestTotal(subtotal, service, tax, totalBill, grandTotal)

	date := extractDate(lines, raw)
	method := detectMethod(raw)

	items, invalidItemCount := extractSimpleItems(lines)
	itemSum := sumItems(items)

	suspiciousMerchant := merchantLooksSuspicious(merchant)
	itemSubtotalMatch := true
	if subtotal > 0 {
		itemSubtotalMatch = almostEqualMoney(itemSum, subtotal, 5000)
	}

	confidence := scoreParsedResult(
		merchant,
		amount,
		date,
		method,
		len(items),
		len(raw),
		suspiciousMerchant,
		totalReliable,
		itemSubtotalMatch,
		invalidItemCount,
	)

	result := &HybridScanResult{
		Amount:     amount,
		Merchant:   merchant,
		Date:       date,
		Type:       "expense",
		Method:     method,
		Note:       "parsed by local OCR hybrid",
		Items:      items,
		Confidence: confidence,
		RawText:    raw,
	}

	shouldFallback := shouldFallbackToGemini(result, subtotal, itemSum, invalidItemCount, totalReliable)
	return result, shouldFallback
}

func normalizeLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")

	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		clean := strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if clean != "" {
			lines = append(lines, clean)
		}
	}
	return lines
}

func extractMerchant(lines []string) string {
	maxCheck := 6
	if len(lines) < maxCheck {
		maxCheck = len(lines)
	}

	for i := 0; i < maxCheck; i++ {
		line := strings.TrimSpace(lines[i])
		upper := strings.ToUpper(line)

		if len(line) < 3 {
			continue
		}

		if isSeparatorLine(line) || isMostlyNumeric(line) || isHeaderLikeLine(upper) {
			continue
		}

		digitCount := 0
		for _, r := range line {
			if r >= '0' && r <= '9' {
				digitCount++
			}
		}
		if digitCount > 3 {
			continue
		}

		merchant := normalizeMerchant(line)
		if merchant == "" {
			continue
		}

		return merchant
	}

	return "Unknown Merchant"
}

func normalizeMerchant(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = merchantCleanRegex.ReplaceAllString(raw, "")
	raw = strings.Join(strings.Fields(raw), " ")
	raw = strings.Trim(raw, "-. ")

	if raw == "" {
		return ""
	}

	return titleWords(raw)
}

func merchantLooksSuspicious(s string) bool {
	if strings.TrimSpace(s) == "" || strings.EqualFold(strings.TrimSpace(s), "unknown merchant") {
		return true
	}

	if strings.ContainsAny(s, "?@#$%^*[]{}|\\") {
		return true
	}

	if len(strings.TrimSpace(s)) < 3 {
		return true
	}

	return false
}

func extractReceiptTotals(lines []string) (subtotal, service, tax, totalBill, grandTotal float64) {
	for _, line := range lines {
		upper := strings.ToUpper(line)

		switch {
		case strings.Contains(upper, "SUB TOTAL") || strings.Contains(upper, "SUBTOTAL"):
			subtotal = extractLastMoney(line)

		case strings.Contains(upper, "SERV"):
			service = extractLastMoney(line)

		case strings.Contains(upper, "PAJAK") || strings.Contains(upper, "TAX"):
			tax = extractLastMoney(line)

		case strings.Contains(upper, "TOTAL BILL"):
			totalBill = extractLastMoney(line)

		case strings.Contains(upper, "GRAND TOTAL"):
			grandTotal = extractLastMoney(line)
		}
	}

	return
}

func chooseBestTotal(subtotal, service, tax, totalBill, grandTotal float64) (float64, bool) {
	componentTotal := subtotal + service + tax

	if subtotal > 0 && componentTotal > 0 {
		if grandTotal > 0 && almostEqualMoney(componentTotal, grandTotal, 2000) {
			return componentTotal, true
		}
		if totalBill > 0 && almostEqualMoney(componentTotal, totalBill, 2000) {
			return componentTotal, true
		}

		// kalau grand total / total bill kebaca salah, tetap prioritaskan hasil komponen
		if grandTotal > 0 || totalBill > 0 {
			return componentTotal, true
		}

		return componentTotal, true
	}

	if grandTotal > 0 {
		return grandTotal, false
	}

	if totalBill > 0 {
		return totalBill, false
	}

	return 0, false
}

func extractLastMoney(line string) float64 {
	matches := moneyRegex.FindAllString(line, -1)
	if len(matches) == 0 {
		return 0
	}
	return parseMoney(matches[len(matches)-1])
}

func extractDate(lines []string, raw string) string {
	var datePart, timePart string

	dateRegex := regexp.MustCompile(`\b\d{2}[-/]\d{2}[-/]\d{2,4}\b`)
	timeRegex := regexp.MustCompile(`\b\d{2}:\d{2}(?::\d{2})?\b`)

	for _, line := range lines {
		upper := strings.ToUpper(line)

		if datePart == "" && (strings.Contains(upper, "TANGGAL") || strings.Contains(upper, "DATE")) {
			datePart = dateRegex.FindString(line)
		}

		if timePart == "" && (strings.Contains(upper, "JAM") || strings.Contains(upper, "TIME") || strings.Contains(upper, "WAKTU")) {
			timePart = timeRegex.FindString(line)
		}
	}

	if datePart != "" && timePart != "" {
		if parsed := parseDateTimeCandidate(datePart + " " + timePart); parsed != "" {
			return parsed
		}
	}

	if datePart != "" {
		if parsed := parseDateTimeCandidate(datePart); parsed != "" {
			return parsed
		}
	}

	// fallback ke raw text kalau line-based gagal
	patterns := []string{
		`\d{4}[-/]\d{2}[-/]\d{2}[ T]\d{2}:\d{2}(?::\d{2})?`,
		`\d{2}[-/]\d{2}[-/]\d{4}[ T]\d{2}:\d{2}(?::\d{2})?`,
		`\d{2}[-/]\d{2}[-/]\d{2}[ T]\d{2}:\d{2}(?::\d{2})?`,
		`\d{2}[-/]\d{2}[-/]\d{4}`,
		`\d{2}[-/]\d{2}[-/]\d{2}`,
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p)
		found := re.FindString(raw)
		if found == "" {
			continue
		}

		if parsed := parseDateTimeCandidate(found); parsed != "" {
			return parsed
		}
	}

	return ""
}

func parseDateTimeCandidate(value string) string {
	value = strings.TrimSpace(value)

	layouts := []string{
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"2006-01-02 15:04",
		"2006/01/02 15:04",

		"02-01-2006 15:04:05",
		"02/01/2006 15:04:05",
		"02-01-2006 15:04",
		"02/01/2006 15:04",

		"02-01-06 15:04:05",
		"02/01/06 15:04:05",
		"02-01-06 15:04",
		"02/01/06 15:04",

		"2006-01-02",
		"2006/01/02",
		"02-01-2006",
		"02/01/2006",
		"02-01-06",
		"02/01/06",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, value)
		if err == nil {
			if strings.Contains(layout, "15:04") {
				return t.Format("2006-01-02 15:04:05")
			}
			return t.Format("2006-01-02 00:00:00")
		}
	}

	return ""
}

func detectMethod(raw string) string {
	upper := strings.ToUpper(raw)

	switch {
	case strings.Contains(upper, "QRIS") || strings.Contains(upper, "E-WALLET"):
		return "QRIS"
	case strings.Contains(upper, "DEBIT"):
		return "Debit"
	case strings.Contains(upper, "KREDIT") || strings.Contains(upper, "CREDIT"):
		return "Credit Card"
	case strings.Contains(upper, "TUNAI") || strings.Contains(upper, "CASH"):
		return "Cash"
	default:
		return ""
	}
}

func getItemSection(lines []string) []string {
	start := -1
	end := len(lines)
	seenHeaderInfo := false

	for i, line := range lines {
		upper := strings.ToUpper(strings.TrimSpace(line))

		if strings.Contains(upper, "KASIR") ||
			strings.Contains(upper, "JUMLAH TAMU") ||
			strings.Contains(upper, "PELAYAN") ||
			strings.Contains(upper, "NO.MEJA") ||
			strings.Contains(upper, "NO. MEJA") {
			seenHeaderInfo = true
		}

		if start == -1 && seenHeaderInfo && isSeparatorLine(upper) {
			start = i + 1
			continue
		}

		if start != -1 {
			if strings.Contains(upper, "SUB TOTAL") ||
				strings.Contains(upper, "SUBTOTAL") ||
				strings.Contains(upper, "SERV") ||
				strings.Contains(upper, "PAJAK") ||
				strings.Contains(upper, "TAX") ||
				strings.Contains(upper, "TOTAL BILL") ||
				strings.Contains(upper, "GRAND TOTAL") ||
				strings.Contains(upper, "THANK YOU") {
				end = i
				break
			}
		}
	}

	if start == -1 || start >= end {
		return nil
	}

	var section []string
	for _, line := range lines[start:end] {
		clean := strings.TrimSpace(line)
		if clean == "" || isSeparatorLine(clean) {
			continue
		}
		section = append(section, clean)
	}

	return section
}

func extractSimpleItems(lines []string) ([]gemini.TransactionItemAI, int) {
	itemLines := getItemSection(lines)
	if len(itemLines) == 0 {
		return nil, 0
	}

	var items []gemini.TransactionItemAI
	invalidItemCount := 0

	for _, line := range itemLines {
		upper := strings.ToUpper(line)

		if isHeaderLikeLine(upper) {
			continue
		}

		// kalau line punya angka uang tapi format item gagal, tandai invalid
		hasMoney := moneyRegex.MatchString(line)

		m := itemLineRegex.FindStringSubmatch(line)
		if len(m) != 4 {
			if hasMoney && containsLetter(line) {
				invalidItemCount++
			}
			continue
		}

		qty, err := strconv.Atoi(strings.TrimSpace(m[1]))
		if err != nil || qty <= 0 {
			invalidItemCount++
			continue
		}

		desc := sanitizeItemDescription(m[2])
		total := parseMoney(m[3])

		if desc == "" || total < 1000 {
			invalidItemCount++
			continue
		}

		if !containsLetter(desc) || looksLikePhoneLine(desc) {
			invalidItemCount++
			continue
		}

		unitPrice := total
		if qty > 1 {
			unitPrice = total / float64(qty)
		}

		items = append(items, gemini.TransactionItemAI{
			Description: desc,
			Quantity:    qty,
			UnitPrice:   unitPrice,
			Total:       total,
		})

		if len(items) >= 30 {
			break
		}
	}

	return items, invalidItemCount
}

func sanitizeItemDescription(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")
	s = strings.Trim(s, `"'`+"`"+`-_:;,. `)
	return s
}

func scoreParsedResult(
	merchant string,
	amount float64,
	date string,
	method string,
	itemCount int,
	rawLen int,
	suspiciousMerchant bool,
	totalReliable bool,
	itemSubtotalMatch bool,
	invalidItemCount int,
) int {
	score := 0

	if merchant != "" && !strings.EqualFold(merchant, "Unknown Merchant") {
		score += 25
	}
	if amount > 0 {
		score += 20
	}
	if date != "" {
		score += 10
	}
	if method != "" {
		score += 5
	}
	if itemCount > 0 {
		score += 20
	}
	if totalReliable {
		score += 10
	}
	if itemSubtotalMatch {
		score += 10
	}
	if suspiciousMerchant {
		score -= 15
	}
	if invalidItemCount > 0 {
		score -= 15
	}
	if rawLen < 30 {
		score -= 10
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

func shouldFallbackToGemini(result *HybridScanResult, subtotal float64, itemSum float64, invalidItemCount int, totalReliable bool) bool {
	if result == nil {
		return true
	}

	if result.Amount <= 0 {
		return true
	}

	if strings.TrimSpace(result.Merchant) == "" || strings.EqualFold(strings.TrimSpace(result.Merchant), "Unknown Merchant") {
		return true
	}

	if merchantLooksSuspicious(result.Merchant) {
		return true
	}

	if len(result.Items) == 0 {
		return true
	}

	if invalidItemCount > 0 {
		return true
	}

	if subtotal > 0 && !almostEqualMoney(itemSum, subtotal, 5000) {
		return true
	}

	if !totalReliable && result.Confidence < 80 {
		return true
	}

	return result.Confidence < 70
}

func sumItems(items []gemini.TransactionItemAI) float64 {
	total := 0.0
	for _, item := range items {
		total += item.Total
	}
	return total
}

func isHeaderLikeLine(upper string) bool {
	skipKeywords := []string{
		"TOTAL", "SUBTOTAL", "SUB TOTAL", "TAX", "PPN", "QTY", "JUMLAH", "TUNAI",
		"KEMBALI", "CHANGE", "DATE", "WAKTU", "STRUK", "RECEIPT", "NO.", "TELP",
		"TLP", "CASHIER", "KASIR", "PELAYAN", "JUMLAH TAMU", "NO.MEJA", "NO. MEJA",
		"TANGGAL", "JAM", "IG", "INSTAGRAM", "THANK YOU", "GRAND TOTAL", "TOTAL BILL",
		"HANYA UNTUK PENGAJIHAN", "BUKAN BUKTI BAYAR",
	}

	for _, kw := range skipKeywords {
		if strings.Contains(upper, kw) {
			return true
		}
	}

	return false
}

func isSeparatorLine(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 5 {
		return false
	}
	for _, r := range s {
		if r != '=' && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

func looksLikePhoneLine(s string) bool {
	digitCount := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digitCount++
		}
	}
	return digitCount >= 8
}

func containsLetter(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

func isMostlyNumeric(s string) bool {
	only := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), " ", ""), ".", ""), ",", "")
	if only == "" {
		return false
	}
	for _, r := range only {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseMoney(raw string) float64 {
	clean := strings.ToUpper(strings.TrimSpace(raw))
	clean = strings.ReplaceAll(clean, "RP", "")
	clean = strings.ReplaceAll(clean, " ", "")
	clean = strings.ReplaceAll(clean, ".", "")
	clean = strings.ReplaceAll(clean, ",", "")

	val, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0
	}
	return val
}

func almostEqualMoney(a, b, tolerance float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= tolerance
}

func titleWords(s string) string {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(s)))
	for i, p := range parts {
		if p == "" {
			continue
		}
		if len(p) == 1 {
			parts[i] = strings.ToUpper(p)
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

func defaultString(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}

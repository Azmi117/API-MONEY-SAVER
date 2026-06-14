package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ParsedTransaction struct {
	Amount   float64
	Merchant string
	Method   string
	Note     string
	Date     time.Time
}

func ParseMandiriEmail(subject string, body string) *ParsedTransaction {
	var merchant string
	var method string
	var note string
	var amount float64
	bodyLower := strings.ToLower(body)
	subjectLower := strings.ToLower(subject)

	// 1. Deteksi Metode & Ekstrak Amount (Pake Sistem Prioritas)
	if strings.Contains(bodyLower, "dengan qr") {
		method = "QRIS"
		amount = getFirstValidAmount(body, []string{"Total", "Nominal"})
	} else if strings.Contains(bodyLower, "biaya transfer") {
		method = "Transfer Bank Lain"
		amount = getFirstValidAmount(body, []string{"Total Transaksi", "Total", "Jumlah Transfer"})
	} else if strings.Contains(bodyLower, "jumlah transfer") || strings.Contains(subjectLower, "transfer") {
		method = "Transfer"
		amount = getFirstValidAmount(body, []string{"Total", "Jumlah Transfer", "Nominal"})
	} else if strings.Contains(subjectLower, "top-up") {
		method = "Top-up"
		amount = getFirstValidAmount(body, []string{"Total", "Nominal Top-up", "Nominal"})
	} else if strings.Contains(bodyLower, "nomor va") || strings.Contains(bodyLower, "virtual account") || strings.Contains(subjectLower, "pembayaran") {
		method = "Virtual Account"
		// Khusus VA: WAJIB cari "Total" dulu biar biaya admin kehitung
		amount = getFirstValidAmount(body, []string{"Total", "Nominal Transaksi", "Nominal"})
	} else {
		method = "Transfer"
		amount = getFirstValidAmount(body, []string{"Total", "Jumlah", "Nominal"})
	}

	// 2. Jurus Ultimate: Kalau semua keyword gagal, cari angka paling gede di email
	if amount == 0 {
		amount = extractMaxAmount(body)
	}

	// 3. Deteksi Merchant & Note
	if method == "Top-up" {
		merchant = "Mandiri E-money"
		note = "Top-up via NFC/Livin"
	} else {
		merchant = extractMerchant(body)
		note = extractNote(body)
	}

	// 4. Extract Date
	reTime := regexp.MustCompile(`(\d{2}:\d{2}:\d{2})`)
	reDate := regexp.MustCompile(`(\d{1,2}\s[A-Za-z]{3}\s\d{4})`)
	dateStr := reDate.FindString(body)
	timeStr := reTime.FindString(body)
	parsedDate := time.Now()

	if dateStr != "" && timeStr != "" {
		fullDateStr := translateIndoMonth(strings.TrimSpace(dateStr) + " " + strings.TrimSpace(timeStr))
		if t, err := time.Parse("2 Jan 2006 15:04:05", fullDateStr); err == nil {
			parsedDate = t
		}
	}

	return &ParsedTransaction{
		Amount:   amount,
		Merchant: strings.Title(strings.ToLower(strings.TrimSpace(merchant))),
		Method:   method,
		Note:     note,
		Date:     parsedDate,
	}
}

func translateIndoMonth(dateStr string) string {
	r := strings.NewReplacer(
		"Jan", "Jan", "Feb", "Feb", "Mar", "Mar",
		"Apr", "Apr", "Mei", "May", "Jun", "Jun",
		"Jul", "Jul", "Agu", "Aug", "Sep", "Sep",
		"Okt", "Oct", "Nov", "Nov", "Des", "Dec",
	)
	return r.Replace(dateStr)
}

// Helper: Bersihin string jadi angka
func parseMoney(str string) float64 {
	clean := strings.ReplaceAll(str, ".", "")
	clean = strings.ReplaceAll(clean, ",", ".")
	val, _ := strconv.ParseFloat(clean, 64)
	return val
}

// Helper: Cari angka berdasarkan prioritas array keyword
func getFirstValidAmount(body string, keywords []string) float64 {
	for _, kw := range keywords {
		val := extractAmount(body, kw)
		if val > 0 {
			return val // Langsung return kalau dapet > 0
		}
	}
	return 0
}

func extractAmount(body string, keyword string) float64 {
	cleanBody := stripHTML(body)
	re := regexp.MustCompile(fmt.Sprintf(`(?is)%s.*?(?:Rp|IDR)[\.\s]*([\d\.,]+)`, keyword))
	match := re.FindStringSubmatch(cleanBody)

	if len(match) > 1 {
		return parseMoney(match[1])
	}
	return 0
}

// Fallback nangkep angka terbesar
func extractMaxAmount(body string) float64 {
	cleanBody := stripHTML(body)
	reFallback := regexp.MustCompile(`(?i)(?:Rp|IDR)[\.\s]*([\d\.,]+)`)
	matches := reFallback.FindAllStringSubmatch(cleanBody, -1)

	var maxAmount float64
	for _, m := range matches {
		if len(m) > 1 {
			val := parseMoney(m[1])
			if val > maxAmount {
				maxAmount = val
			}
		}
	}
	return maxAmount
}

func extractMerchant(body string) string {
	cleanBody := stripHTML(body)
	re := regexp.MustCompile(`(?i)(?:Penerima|Penyedia Jasa|Institusi|Merchant)\s+(.*?)(?:\s\s|$)`)
	match := re.FindStringSubmatch(cleanBody)

	if len(match) > 1 {
		res := strings.TrimSpace(match[1])
		if idx := strings.Index(res, "  "); idx != -1 {
			res = res[:idx]
		}
		if res != "" {
			return res
		}
	}
	return "Merchant Tidak Terdeteksi"
}

func extractNote(body string) string {
	cleanBody := stripHTML(body)
	re := regexp.MustCompile(`(?i)Keterangan\s+(.*?)(?:\s\s|$)`)
	match := re.FindStringSubmatch(cleanBody)

	if len(match) > 1 {
		res := strings.TrimSpace(match[1])
		if idx := strings.Index(res, "  "); idx != -1 {
			res = res[:idx]
		}
		if res != "" && res != "-" {
			return res
		}
	}
	return "-"
}

func stripHTML(input string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	clean := re.ReplaceAllString(input, "  ") // Pakai 2 spasi buat bates kolom
	clean = strings.ReplaceAll(clean, "&nbsp;", " ")
	clean = strings.ReplaceAll(clean, "\t", "  ")
	clean = strings.ReplaceAll(clean, "\n", "  ")
	clean = strings.ReplaceAll(clean, "\r", "  ")

	// Bersihin spasi berlebih
	for strings.Contains(clean, "   ") {
		clean = strings.ReplaceAll(clean, "   ", "  ")
	}
	return clean
}

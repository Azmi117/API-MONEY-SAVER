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

	// 1. Deteksi Metode & Amount (Pake Parameter Sakti Lo)
	if strings.Contains(bodyLower, "dengan qr") {
		method = "QRIS"
		amount = extractAmount(body, "Nominal")
	} else if strings.Contains(bodyLower, "biaya transfer") {
		method = "Transfer Bank Lain"
		amount = extractAmount(body, "Total Transaksi") // Sesuai saran lo, ambil TOTAL (biar saldo sinkron)
	} else if strings.Contains(bodyLower, "jumlah transfer") {
		method = "Transfer"
		amount = extractAmount(body, "Jumlah Transfer")
	} else if strings.Contains(subjectLower, "top-up") {
		method = "Top-up"
		amount = extractAmount(body, "Nominal Top-up")
	} else {
		// Fallback jika tidak ada yang cocok
		method = "Transfer"
		amount = extractAmount(body, "(?:Nominal|Total|Jumlah)")
	}

	// 2. Deteksi Merchant (Penerima/Penyedia Jasa)
	if method == "Top-up" {
		merchant = "Mandiri E-money"
	} else {
		merchant = extractMerchant(body)
	}

	// 3. Deteksi Note (Pesan/Keterangan)
	if method == "Top-up" {
		note = "Top-up via NFC/Livin"
	} else {
		note = extractNote(body)
	}

	// 4. Extract Date (Pake logic lama lo yang udah oke)
	reTime := regexp.MustCompile(`(\d{2}:\d{2}:\d{2})`)
	reDate := regexp.MustCompile(`(\d{1,2}\s[A-Za-z]{3}\s\d{4})`)
	dateStr := reDate.FindString(body)
	timeStr := reTime.FindString(body)
	parsedDate := time.Now() // Default

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

func extractAmount(body string, keyword string) float64 {
	// (?s) biar tembus newline, [^R]* biar cari angka setelah Rp
	re := regexp.MustCompile(fmt.Sprintf(`(?s)%s[^R]*Rp\s?([\d\.,]+)`, keyword))
	match := re.FindStringSubmatch(body)
	if len(match) > 1 {
		clean := strings.ReplaceAll(match[1], ".", "")
		clean = strings.ReplaceAll(clean, ",", ".")
		val, _ := strconv.ParseFloat(clean, 64)
		return val
	}
	return 0
}

func extractMerchant(body string) string {
	cleanBody := stripHTML(body)

	// Regex ini ambil teks SETELAH 'Penerima', tapi BERHENTI kalau ketemu 2 spasi atau lebih
	// karena di email Mandiri antar label biasanya dipisah spasi banyak
	re := regexp.MustCompile(`(?i)(?:Penerima|Penyedia Jasa)\s*[:\s]+(.*?)(?:\s{2,}|$)`)
	match := re.FindStringSubmatch(cleanBody)

	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return "Merchant Tidak Terdeteksi"
}

func extractNote(body string) string {
	cleanBody := stripHTML(body)

	// Sama kayak merchant, ambil teks SETELAH 'Keterangan', berhenti kalau ketemu spasi double
	re := regexp.MustCompile(`(?i)Keterangan\s*[:\s]+(.*?)(?:\s{2,}|$)`)
	match := re.FindStringSubmatch(cleanBody)

	if len(match) > 1 {
		note := strings.TrimSpace(match[1])
		if note != "" && note != "-" {
			return note
		}
	}
	return "-"
}

func stripHTML(input string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	// Ganti tag HTML dengan "  " (dua spasi) biar ada pemisah antar kata yang tadinya beda kolom
	clean := re.ReplaceAllString(input, "  ")

	clean = strings.ReplaceAll(clean, "&nbsp;", " ")
	// Hapus tab dan ganti ke spasi
	clean = strings.ReplaceAll(clean, "\t", "  ")

	return clean
}

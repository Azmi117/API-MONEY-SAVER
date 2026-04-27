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

	// 1. Deteksi Metode (QRIS harus paling atas karena subject-nya sering cuma "Transfer")
	if strings.Contains(bodyLower, "dengan qr") {
		method = "QRIS"
		amount = extractAmount(body, "Nominal")
	} else if strings.Contains(bodyLower, "biaya transfer") {
		method = "Transfer Bank Lain"
		amount = extractAmount(body, "Total Transaksi")
	} else if strings.Contains(bodyLower, "jumlah transfer") || strings.Contains(subjectLower, "transfer") {
		method = "Transfer"
		amount = extractAmount(body, "Jumlah Transfer")
	} else if strings.Contains(subjectLower, "top-up") {
		method = "Top-up"
		amount = extractAmount(body, "Nominal Top-up")
	} else {
		method = "Transfer"
		amount = extractAmount(body, "(?:Nominal|Total|Jumlah)")
	}

	// 2. Deteksi Merchant & Note (Pake Body Mentah biar Regex-nya akurat lewat Tag HTML)
	if method == "Top-up" {
		merchant = "Mandiri E-money"
		note = "Top-up via NFC/Livin"
	} else {
		merchant = extractMerchant(body)
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
	// Regex ini nyari teks di dalam tag <h4> atau <td> yang ada setelah kata Penerima/Penyedia Jasa
	// Lebih aman daripada stripHTML dulu
	re := regexp.MustCompile(`(?i)(?:Penerima|Penyedia Jasa).*?<(?:h4|td)[^>]*>\s*(.*?)\s*</(?:h4|td)>`)
	match := re.FindStringSubmatch(body)

	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return "Merchant Tidak Terdeteksi"
}

func extractNote(body string) string {
	// Cari label "Keterangan", lalu ambil isi <td> di sampingnya
	re := regexp.MustCompile(`(?i)Keterangan.*?<td[^>]*>\s*(.*?)\s*</td>`)
	match := re.FindStringSubmatch(body)

	if len(match) > 1 {
		res := strings.TrimSpace(match[1])
		if res != "" && res != "-" {
			return res
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

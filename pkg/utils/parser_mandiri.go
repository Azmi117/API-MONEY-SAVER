package utils

import (
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ParsedTransaction struct {
	Amount   float64
	Merchant string
	Date     time.Time
}

func ParseMandiriEmail(subject string, body string) *ParsedTransaction {
	// 1. Regex Patterns
	// Disederhanakan: Cari kata Nominal/Total, lalu ambil angka setelah Rp
	reAmount := regexp.MustCompile(`(?:Nominal|Total).*?Rp\s?([0-9.]+)`)
	reTime := regexp.MustCompile(`(\d{2}:\d{2}:\d{2})`)
	reDate := regexp.MustCompile(`(\d{1,2}\s[A-Za-z]{3}\s\d{4})`)

	// 2. Tentukan Merchant
	var merchant string
	subjectLower := strings.ToLower(subject)

	if strings.Contains(subjectLower, "top-up") {
		merchant = "top-up e-money"
	} else if strings.Contains(subjectLower, "berhasil") {
		merchant = extractMerchant(body, "Penerima")
	} else {
		return nil
	}

	// 3. Extract Amount
	amountMatch := reAmount.FindAllStringSubmatch(body, -1)
	var amount float64

	if len(amountMatch) > 0 {
		// Ambil match terakhir (biar BI Fast dapet yang 'Total', e-money dapet yang 'Nominal')
		lastMatch := amountMatch[len(amountMatch)-1]
		cleanAmount := strings.ReplaceAll(lastMatch[1], ".", "")
		amount, _ = strconv.ParseFloat(cleanAmount, 64)
	}

	// 4. Extract & Parse Date
	dateMatch := reDate.FindString(body)
	timeMatch := reTime.FindString(body)

	var parsedDate time.Time
	if dateMatch != "" && timeMatch != "" {
		// Gabungkan dan bersihkan spasi double jika ada
		fullDateStr := strings.TrimSpace(dateMatch) + " " + strings.TrimSpace(timeMatch)

		// Samain format bulan (Jan, Feb, dsb)
		fullDateStr = translateIndoMonth(fullDateStr)

		// Layout Go: 2 Jan 2006 15:04:05
		layout := "2 Jan 2006 15:04:05"
		var err error
		parsedDate, err = time.Parse(layout, fullDateStr)

		if err != nil {
			log.Printf("[Parser Error] Gagal parse tanggal '%s': %v", fullDateStr, err)
			parsedDate = time.Now()
		}
	} else {
		parsedDate = time.Now()
	}

	return &ParsedTransaction{
		Amount:   amount,
		Merchant: strings.ToLower(strings.TrimSpace(merchant)),
		Date:     parsedDate,
	}
}

// Helper buat ganti 'Apr' (Indo) ke 'Apr' (Eng) - kebetulan sama,
// tapi kalau 'Mei' jadi 'May', 'Agu' jadi 'Aug'
func translateIndoMonth(dateStr string) string {
	r := strings.NewReplacer(
		"Jan", "Jan", "Feb", "Feb", "Mar", "Mar",
		"Apr", "Apr", "Mei", "May", "Jun", "Jun",
		"Jul", "Jul", "Agu", "Aug", "Sep", "Sep",
		"Okt", "Oct", "Nov", "Nov", "Des", "Dec",
	)
	return r.Replace(dateStr)
}

// Helper buat ambil nama merchant setelah label tertentu
func extractMerchant(body string, label string) string {
	// Logic sederhana: cari kata setelah "Penerima"
	// Di aplikasi nyata, lu mungkin perlu library HTML parser kalau emailnya format HTML
	return "Mandiri Transfer/QRIS" // Placeholder, bisa lu improve sesuai body emailnya
}

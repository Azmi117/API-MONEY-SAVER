package utils

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
)

func ManualParserV3(raw string) (string, float64, time.Time, []models.TransactionItem) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r", ""), "\n")

	var merchant string
	var amount float64
	var items []models.TransactionItem
	transactionDate := time.Now()

	reMoney := regexp.MustCompile(`\d{1,3}([.,]\d{3})+`)

	for _, line := range lines {
		clean := strings.TrimSpace(line)
		upper := strings.ToUpper(clean)
		noiseRegex := regexp.MustCompile(`(?i)(CASHIER|RECEIPT|DATE|TELP|HP|JL|ALAMAT|NPWP|===|ITEM|QTY|NO\.|CUSTOMER|WELCOME|PRINT|COMPUTER)`)

		if len(clean) > 3 && !noiseRegex.MatchString(upper) {
			merchant = strings.Split(upper, "/")[0]
			merchant = strings.Split(merchant, "\t")[0]
			merchant = strings.Split(merchant, " - ")[0]
			merchant = strings.TrimSpace(merchant)
			break
		}
	}

	reDateStandard := regexp.MustCompile(`\d{2}[./-]\d{2}[./-]\d{2,4}`)
	reDateText := regexp.MustCompile(`(?i)\d{1,2}\s+(January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{4}`)

	if match := reDateText.FindString(raw); match != "" {
		t, _ := time.Parse("02 January 2006", match)
		if !t.IsZero() {
			transactionDate = t
		}
	} else if match := reDateStandard.FindString(raw); match != "" {
		normalized := strings.NewReplacer("/", ".", "-", ".").Replace(match)
		layouts := []string{"02.01.06", "02.01.2006"}
		for _, l := range layouts {
			if t, err := time.Parse(l, normalized); err == nil {
				transactionDate = t
				break
			}
		}
	}

	keywordFound := false
	priorityKeys := []string{"TOTAL BELANJA", "GRAND TOTAL", "TOTAL TAGIHAN", "TOTAL BAYAR", "NET TOTAL", "TOTAL RP"}

	for _, key := range priorityKeys {
		for i, line := range lines {
			if strings.Contains(strings.ToUpper(line), key) {
				match := reMoney.FindString(line)
				if match == "" && i+1 < len(lines) {
					match = reMoney.FindString(lines[i+1])
				}
				if match != "" {
					cleanNum := strings.NewReplacer(".", "", ",", "").Replace(match)
					val, _ := strconv.ParseFloat(cleanNum, 64)
					if val > 0 {
						amount = val
						keywordFound = true
						break
					}
				}
			}
		}
		if keywordFound {
			break
		}
	}

	if !keywordFound {
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.Contains(strings.ToUpper(lines[i]), "TOTAL") {
				match := reMoney.FindString(lines[i])
				if match == "" && i+1 < len(lines) {
					match = reMoney.FindString(lines[i+1])
				}
				if match != "" {
					cleanNum := strings.NewReplacer(".", "", ",", "").Replace(match)
					val, _ := strconv.ParseFloat(cleanNum, 64)
					if val > 0 {
						amount = val
						keywordFound = true
						break
					}
				}
			}
		}
	}

	if !keywordFound {
		allMatches := reMoney.FindAllString(raw, -1)
		for _, m := range allMatches {
			cleanNum := strings.NewReplacer(".", "", ",", "").Replace(m)
			num, _ := strconv.ParseFloat(cleanNum, 64)
			if num > amount && num < 2000000 {
				amount = num
			}
		}
	}

	for i, line := range lines {
		upperLine := strings.ToUpper(line)

		skipRegex := regexp.MustCompile(`(?i)(TOTAL|BELANJA|NPWP|TELP|CASH|TUNAI|KEMBALI|===|---|SUBTOTAL|PB1|TAX|PAJAK|DISC|ITEM|QTY|POWERED|BY)`)
		if skipRegex.MatchString(upperLine) || len(strings.TrimSpace(line)) < 3 {
			continue
		}

		matches := reMoney.FindAllString(line, -1)

		if len(matches) > 0 {
			priceMatch := matches[len(matches)-1]
			cleanNum := strings.NewReplacer(".", "", ",", "", "Rp", "").Replace(priceMatch)
			price, _ := strconv.ParseFloat(cleanNum, 64)

			if price > 0 && price < amount {
				itemName := strings.TrimSpace(strings.Replace(line, priceMatch, "", 1))
				itemName = strings.ReplaceAll(itemName, "Rp", "")

				isJustMath := regexp.MustCompile(`^[\d\sX@x,.*:-]*$`).MatchString(itemName)
				if (len(itemName) < 2 || isJustMath) && i > 0 {
					potentialName := strings.TrimSpace(lines[i-1])
					if len(potentialName) > 2 && !skipRegex.MatchString(potentialName) {
						itemName = potentialName
					}
				}

				qty := 1.0
				reQty := regexp.MustCompile(`(\d+)\s*[xX]`)
				qtyMatch := reQty.FindStringSubmatch(itemName)
				if len(qtyMatch) > 1 {
					qty, _ = strconv.ParseFloat(qtyMatch[1], 64)
					itemName = strings.TrimSpace(reQty.ReplaceAllString(itemName, ""))
				}

				itemName = regexp.MustCompile(`[:@\t]`).ReplaceAllString(itemName, " ")
				itemName = regexp.MustCompile(`\s+`).ReplaceAllString(itemName, " ")
				itemName = strings.TrimSpace(itemName)

				if len(itemName) > 2 {
					items = append(items, models.TransactionItem{
						Description: itemName,
						Quantity:    int(qty),
						Price:       price,
						Total:       price * qty,
					})
				}
			}
		}
	}

	return merchant, amount, transactionDate, items
}

package telegram

import (
	"regexp"
	"strconv"
	"strings"
)

func ParseChatToTransaction(text string) (string, float64, bool) {
	// Contoh: "Nasi Padang 25000"
	re := regexp.MustCompile(`(?i)(.+?)\s+(\d+)`)
	match := re.FindStringSubmatch(text)
	if len(match) < 3 {
		return "", 0, false
	}
	amount, _ := strconv.ParseFloat(match[2], 64)
	return strings.TrimSpace(match[1]), amount, true
}

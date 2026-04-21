package ocr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func (t *TesseractClient) ExtractRawText(ctx context.Context, imgData []byte, mimeType string) (string, error) {
	ext := ".jpg"
	if strings.Contains(strings.ToLower(mimeType), "png") {
		ext = ".png"
	}

	tmpFile, err := os.CreateTemp("", "receipt-*"+ext)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(imgData); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	tesseractPath := os.Getenv("TESSERACT_PATH")
	if tesseractPath == "" {
		tesseractPath = "tesseract"
	}

	cmd := exec.CommandContext(
		ctx,
		tesseractPath,
		tmpFile.Name(),
		"stdout",
		"-l", "ind+eng",
		"--oem", "1",
		"--psm", "6",
		"-c", "preserve_interword_spaces=1",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tesseract extract raw text failed: %v | output: %s", err, string(output))
	}

	rawText := strings.TrimSpace(string(output))
	if rawText == "" {
		return "", fmt.Errorf("tesseract return empty text")
	}

	return rawText, nil
}

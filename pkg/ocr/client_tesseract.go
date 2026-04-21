package ocr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type TesseractResult struct {
	RawText string
}

type TesseractClient struct{}

func NewTesseractClient() *TesseractClient {
	return &TesseractClient{}
}

func (t *TesseractClient) ScanReceiptRaw(ctx context.Context, imgData []byte) (string, error) {
	imgPath := filepath.Join(".", "temp_hybrid.png")
	err := os.WriteFile(imgPath, imgData, 0644)
	if err != nil {
		return "", err
	}
	defer os.Remove(imgPath)

	// Pastikan path ini sesuai di Windows lo
	tesseractPath := `C:\Program Files\Tesseract-OCR\tesseract.exe`

	// Kita pake whitelist biar gak terlalu banyak noise karakter aneh
	cmd := exec.Command(tesseractPath, imgPath, "stdout", "-l", "ind+eng", "--psm", "6")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tesseract failed: %v", err)
	}

	return string(output), nil
}

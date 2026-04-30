package ocr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

type OCRSpaceClient struct {
	APIKey string
}

func NewOCRSpaceClient(apiKey string) *OCRSpaceClient {
	return &OCRSpaceClient{APIKey: apiKey}
}

type OCRSpaceResponse struct {
	ParsedResults []struct {
		ParsedText string `json:"ParsedText"`
	} `json:"ParsedResults"`
	OCRExitCode           int      `json:"OCRExitCode"`
	IsErroredOnProcessing bool     `json:"IsErroredOnProcessing"`
	ErrorMessage          []string `json:"ErrorMessage"`
}

func (c *OCRSpaceClient) ExtractRawText(imagePath string) (string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("apikey", c.APIKey)
	_ = writer.WriteField("language", "eng")
	_ = writer.WriteField("isTable", "true")
	_ = writer.WriteField("OCREngine", "3")

	part, err := writer.CreateFormFile("file", imagePath)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return "", err
	}
	writer.Close()

	req, err := http.NewRequest("POST", "https://api.ocr.space/parse/image", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res OCRSpaceResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	if res.IsErroredOnProcessing {
		return "", fmt.Errorf("OCR.space error: %v", res.ErrorMessage)
	}

	if len(res.ParsedResults) > 0 {
		return res.ParsedResults[0].ParsedText, nil
	}

	return "", fmt.Errorf("no text detected by OCR.space")
}

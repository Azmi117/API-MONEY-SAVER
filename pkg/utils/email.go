package utils

import (
	"crypto/tls"
	"fmt"
	"os"
	"strconv"

	"gopkg.in/gomail.v2"
)

// SendEmail adalah fungsi serbaguna buat ngirim email (termasuk OTP)
func SendEmail(to string, subject string, body string) error {
	mailer := gomail.NewMessage()

	// Set sender, penerima, judul, dan isi email (bisa HTML)
	mailer.SetHeader("From", os.Getenv("SMTP_USER"))
	mailer.SetHeader("To", to)
	mailer.SetHeader("Subject", subject)
	mailer.SetBody("text/html", body)

	// Tarik config dari .env
	port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	dialer := gomail.NewDialer(
		os.Getenv("SMTP_HOST"),
		port,
		os.Getenv("SMTP_USER"),
		os.Getenv("SMTP_PASS"),
	)

	// Bypass SSL certificate check (opsional, tapi biasanya dibutuhin buat SMTP Google/Mailtrap)
	dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	if err := dialer.DialAndSend(mailer); err != nil {
		return fmt.Errorf("gagal ngirim email: %v", err)
	}

	return nil
}

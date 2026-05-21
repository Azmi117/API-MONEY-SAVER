package utils

import (
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/apperror" // <-- IMPORT PACKAGE ERROR LU DI SINI
	"github.com/matthewhartstonge/argon2"
)

func HashPassword(password string) (string, error) {
	argon := argon2.DefaultConfig()
	encoded, err := argon.HashEncoded([]byte(password))
	if err != nil {
		return "", apperror.Internal("Failed to hash password") // <-- Pake apperror sekalian
	}
	return string(encoded), nil
}

func VerifyPassword(password string, encodedHash string) error {
	ok, err := argon2.VerifyEncoded([]byte(password), []byte(encodedHash))

	// 1. Kalo format hash di DB rusak / corrupt, lempar sebagai Internal Error
	if err != nil {
		return apperror.Internal("Failed to verify password structure")
	}

	// 2. KUNCI REFACTOR: Kalo password salah, bungkus pake apperror.Unauthorized!
	if !ok {
		return apperror.Unauthorized("Invalid email or password")
	}

	return nil
}

package utils

import "github.com/matthewhartstonge/argon2"

func HashPassword(password string) (string, error) {
	argon := argon2.DefaultConfig()

	encoded, err := argon.HashEncoded([]byte(password))

	if err != nil {
		return "", err
	}

	return string(encoded), nil
}

func VerifyPassword(pasword string, encodedHash string) error {
	_, err := argon2.VerifyEncoded([]byte(pasword), []byte(encodedHash))

	if err != nil {
		return err
	}

	return nil
}

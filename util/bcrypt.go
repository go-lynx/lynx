package util

import (
	"golang.org/x/crypto/bcrypt"
)

// HashEncryption Plaintext Encryption
func HashEncryption(plaintext string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plaintext), 10)
	return string(bytes), err
}

// CheckCiphertext Ciphertext Verification
func CheckCiphertext(plaintext, ciphertext string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(ciphertext), []byte(plaintext))
	return err == nil
}

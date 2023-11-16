package util

import (
	"golang.org/x/crypto/bcrypt"
)

// HashEncryption 进行 bcrypt 加密
func HashEncryption(plaintext string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plaintext), 10)
	return string(bytes), err
}

// CheckCiphertext bcrypt 密文校验
func CheckCiphertext(plaintext, ciphertext string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(ciphertext), []byte(plaintext))
	return err == nil
}

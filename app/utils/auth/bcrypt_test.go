package auth

import (
	"fmt"
	"testing"
)

// TestBcrypt unit test for bcrypt encryption and decryption verification
func TestBcrypt(t *testing.T) {
	// Define plaintext password
	plaintext := "123"
	// Call HashPassword to generate hash
	encryption, err := HashPassword(plaintext, 10)
	// Check if there is an error during encryption
	if err != nil {
		// If there is an error, use panic to terminate the program and output error message
		panic(err)
	}
	// Print the encrypted ciphertext
	fmt.Printf("Ciphertext: %s\n", encryption)
	// Call CheckPassword to verify plaintext and ciphertext
	check := CheckPassword(encryption, plaintext)
	// Check if the verification result is false
	if !check {
		// If verification fails, use panic to terminate the program and output error message
		panic("check error")
	}
}

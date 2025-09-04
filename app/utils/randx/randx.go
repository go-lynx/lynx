package randx

import (
	"crypto/rand"
	"errors"
	"io"
	"math/big"
)

// Default URL-safe alphabet (excluding easily confused characters).
const defaultAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_"

// CryptoBytes returns a cryptographically strong random byte slice of length n.
func CryptoBytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, errors.New("CryptoBytes: negative n")
	}
	b := make([]byte, n)
	if n == 0 {
		return b, nil
	}
	_, err := io.ReadFull(rand.Reader, b)
	return b, err
}

// randInt generates a uniform random integer in [0, max) using crypto/rand.
func randInt(max int64) (int64, error) {
	if max <= 0 {
		return 0, errors.New("randInt: non-positive max")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}

// RandString generates a random string of length n; uses the default alphabet when empty.
func RandString(n int, alphabet string) (string, error) {
	if n < 0 {
		return "", errors.New("RandString: negative n")
	}
	if n == 0 {
		return "", nil
	}
	if alphabet == "" {
		alphabet = defaultAlphabet
	}
	runes := []rune(alphabet)
	if len(runes) == 0 {
		return "", errors.New("RandString: empty alphabet")
	}
	out := make([]rune, n)
	for i := 0; i < n; i++ {
		idx, err := randInt(int64(len(runes)))
		if err != nil {
			return "", err
		}
		out[i] = runes[idx]
	}
	return string(out), nil
}

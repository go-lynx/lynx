package randx

import (
	"crypto/rand"
	"errors"
	"io"
	"math/big"
)

// 默认 URL 安全字符集（不含容易混淆的字符）。
const defaultAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_"

// CryptoBytes 返回长度为 n 的强随机字节切片。
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

// randInt 生成 [0, max) 的均匀分布随机整数，基于 crypto/rand。
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

// RandString 生成长度为 n 的随机字符串，alphabet 为空则使用默认字符集。
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

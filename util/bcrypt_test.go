package util

import (
	"fmt"
	"testing"
)

// TestBcrypt 进行 bcrypt 加密
func TestBcrypt(t *testing.T) {
	// 加密
	encryption, err := HashEncryption("123")
	if err != nil {
		panic(err)
	}
	fmt.Printf("密文：" + encryption + "\n")
	// 解密并校验
	check := CheckCiphertext("123", encryption)
	if !check {
		panic("check error")
	}
}

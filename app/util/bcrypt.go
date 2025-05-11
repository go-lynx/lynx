package util

import (
	"golang.org/x/crypto/bcrypt"
)

// HashEncryption 对明文进行加密操作，使用 bcrypt 算法。
// 参数 plaintext 是需要加密的明文。
// 返回值为加密后的字符串和可能出现的错误。
func HashEncryption(plaintext string, cost int) (string, error) {
	// 检查 cost 值是否在 bcrypt 的有效范围内
	if cost < 4 || cost > 31 {
		cost = 10
	}
	// 使用 bcrypt.GenerateFromPassword 函数对明文进行加密，第二个参数 10 是 cost 值，控制加密的计算复杂度。
	bytes, err := bcrypt.GenerateFromPassword([]byte(plaintext), cost)
	// 将加密后的字节切片转换为字符串返回
	return string(bytes), err
}

// CheckCiphertext 对明文和密文进行校验，验证明文是否与密文匹配。
// 参数 plaintext 是需要验证的明文，ciphertext 是用于比对的密文。
// 返回值为布尔类型，表示校验是否通过。
func CheckCiphertext(plaintext, ciphertext string) bool {
	// 使用 bcrypt.CompareHashAndPassword 函数比较明文和密文是否匹配
	err := bcrypt.CompareHashAndPassword([]byte(ciphertext), []byte(plaintext))
	// 若 err 为 nil 则表示匹配成功，返回 true；否则返回 false
	return err == nil
}

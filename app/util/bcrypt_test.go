package util

import (
	"fmt"
	"testing"
)

// TestBcrypt 进行 bcrypt 加密和解密校验的单元测试
func TestBcrypt(t *testing.T) {
	// 定义明文密码
	plaintext := "123"
	// 调用 HashPassword 生成哈希
	encryption, err := HashPassword(plaintext, 10)
	// 检查加密过程中是否出现错误
	if err != nil {
		// 若出现错误，使用 panic 终止程序并输出错误信息
		panic(err)
	}
	// 打印加密后的密文
	fmt.Printf("密文：%s\n", encryption)
	// 调用 CheckPassword 对明文和密文进行校验
	check := CheckPassword(encryption, plaintext)
	// 检查校验结果是否为 false
	if !check {
		// 若校验失败，使用 panic 终止程序并输出错误信息
		panic("check error")
	}
}

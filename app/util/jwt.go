package util

import (
	"crypto/ecdsa"
	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims interface {
	Init() error
	Valid() error
	Decoration() error
	jwt.Claims
}

// Sign 方法用于生成一个 JWT 令牌
func Sign(c CustomClaims, alg string, key *ecdsa.PrivateKey) (string, error) {
	// 初始化自定义声明
	err := c.Init()
	// 如果初始化失败，返回空字符串和错误信息
	if err != nil {
		return "", err
	}

	// 验证自定义声明
	err = c.Valid()
	// 如果验证失败，返回空字符串和错误信息
	if err != nil {
		return "", err
	}

	// 创建一个新的 JWT 对象，使用指定的签名算法和自定义声明
	t := jwt.NewWithClaims(jwt.GetSigningMethod(alg), c)
	// 使用指定的私钥对 JWT 进行签名，并返回签名后的字符串
	return t.SignedString(key)
}

// Check 方法用于验证一个 JWT 令牌的有效性
func Check(token string, c CustomClaims, key ecdsa.PublicKey) (bool, error) {
	// 解析 JWT 令牌，并将自定义声明绑定到解析结果上
	parse, err := jwt.ParseWithClaims(token, c, func(token *jwt.Token) (interface{}, error) {
		// 返回用于验证签名的公钥
		return &key, nil
	})
	// 如果发生错误，返回 false 和错误信息
	if err != nil {
		return false, err
	}
	// 对自定义声明进行装饰
	err = c.Decoration()
	// 如果发生错误，返回 false 和错误信息
	if err != nil {
		return false, err
	}
	// 返回解析结果是否有效
	return parse.Valid, nil
}

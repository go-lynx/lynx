package jwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"

	jwt "github.com/golang-jwt/jwt/v5"
)

// baseClaims 为测试提供最小实现，满足 CustomClaims 接口。
type baseClaims struct{ jwt.RegisteredClaims }

func (b *baseClaims) Init() error       { return nil }
func (b *baseClaims) Valid() error      { return nil }
func (b *baseClaims) Decoration() error { return nil }

// TestClaims: 测试使用的 claims，避免在结构中出现接口字段导致的 JSON 反序列化问题。
type TestClaims struct {
	jwt.RegisteredClaims
	ID       int64  `json:"id"`
	Nickname string `json:"nickname"`
}

func (t *TestClaims) Init() error       { return nil }
func (t *TestClaims) Valid() error      { return nil }
func (t *TestClaims) Decoration() error { return nil }

func TestJwtTokenSigning(t *testing.T) {
	// Generate a key
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	// Convert the key to a PEM
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		panic(err)
	}
	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	fmt.Println("Private key:")
	fmt.Println(string(keyPem))

	// Reverse parse the private key
	privateBlock, _ := pem.Decode(keyPem)
	if privateBlock == nil {
		panic("failed to parse PEM block containing the public key")
	}
	privateKey, err := x509.ParseECPrivateKey(privateBlock.Bytes)
	if err != nil {
		panic(err)
	}

	// Convert the public key to a PEM
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		panic(err)
	}
	pubKeyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	fmt.Println("Public key:")
	fmt.Println(string(pubKeyPem))

	// Sign a JWT token（使用不含接口字段的 TestClaims）
	signing, err := Sign(&TestClaims{
		ID:       123,
		Nickname: "老王",
	}, "ES256", privateKey)
	if err != nil {
		panic(err)
	}
	fmt.Printf("JWT private key signing: %s\n", signing)

	// Reverse parse the public key
	block, _ := pem.Decode(pubKeyPem)
	if block == nil {
		panic("failed to parse PEM block containing the public key")
	}
	pubKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic(err)
	}

	pubKey, ok := pubKeyInterface.(*ecdsa.PublicKey)
	if !ok {
		panic("cannot parse public key")
	}

	// Verify the JWT token
	parsed := &TestClaims{}
	check, err := Verify(signing, parsed, *pubKey)
	if check {
		fmt.Printf("JWT public key verification: %d\n", parsed.ID)
	}
	if err != nil {
		panic(err)
	}
}

// LoginClaims represents the claims in a JWT token for user login
// LoginClaims 表示用户登录的 JWT 令牌中的声明信息
type LoginClaims struct {
	CustomClaims
	Id       int64  `json:"id"`
	Nickname string `json:"nickname"`
}

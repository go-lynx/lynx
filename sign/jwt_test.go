package sign

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
)

func TestJwtTokenSigning(t *testing.T) {
	// 加密
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	// 将密钥转换为 PEM
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		panic(err)
	}
	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	fmt.Println("私钥\n" + string(keyPem))

	// 从字符串反转取私钥
	privateBlock, _ := pem.Decode(keyPem)
	if privateBlock == nil {
		panic("failed to parse PEM block containing the public key")
	}

	privateKey, err := x509.ParseECPrivateKey(privateBlock.Bytes)

	// 将公钥转换为 PEM
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		panic(err)
	}
	pubKeyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	fmt.Println("公钥\n" + string(pubKeyPem))

	signing, err := Sign(&LoginClaims{
		Id:       123,
		Nickname: "老王",
	}, "ES256", privateKey)
	if err != nil {
		panic(err)
	}
	fmt.Printf("\njwt私钥签名：" + signing + "\n")

	// 从字符串反转取公钥
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
		panic("未能成功解析出公钥")
	}

	// 解密
	l := &LoginClaims{}
	check, err := Check(signing, l, *pubKey)
	if check {
		fmt.Printf("jwt公钥解密：%d", l.Id)
	}
	if err != nil {
		panic(err)
	}
}

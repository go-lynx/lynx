package login

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/go-lynx/lynx/conf"
)

var (
	method     string
	privateKey *ecdsa.PrivateKey
	publicKey  *ecdsa.PublicKey
)

type Login struct {
}

func (l *Login) Init(Token *conf.Token) error {
	// 加密方式
	method = Token.Jwt.LoginMethod

	//  私钥
	privateBlock, _ := pem.Decode([]byte(Token.Jwt.LoginPrivateKey))
	if privateBlock == nil {
		panic("failed to parse PEM block containing the private key")
	}

	prk, err := x509.ParseECPrivateKey(privateBlock.Bytes)
	if err != nil {
		return err
	}
	privateKey = prk

	// 公钥
	publicBlock, _ := pem.Decode([]byte(Token.Jwt.LoginPublicKey))
	if err != nil {
		return err
	}
	if publicBlock == nil {
		panic("failed to parse PEM block containing the public key")
	}

	pubKeyInterface, err := x509.ParsePKIXPublicKey(publicBlock.Bytes)
	if err != nil {
		return err
	}
	puk := pubKeyInterface.(*ecdsa.PublicKey)
	publicKey = puk
	return nil
}

func NewLogin() *Login {
	return &Login{}
}

func GetMethod() string {
	return method
}

func GetPrivateKey() *ecdsa.PrivateKey {
	return privateKey
}

func GetPublicKey() *ecdsa.PublicKey {
	return publicKey
}

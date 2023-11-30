package login

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/go-lynx/lynx/plugin/token/conf"
)

var (
	method     string
	privateKey *ecdsa.PrivateKey
	publicKey  *ecdsa.PublicKey
)

type Login struct {
}

func (l *Login) Init(base interface{}) error {
	c, ok := base.(*conf.Jtw)
	if !ok {
		return fmt.Errorf("invalid c type, expected *conf.Grpc")
	}

	method = c.LoginMethod

	privateBlock, _ := pem.Decode([]byte(c.LoginPrivateKey))
	if privateBlock == nil {
		panic("failed to parse PEM block containing the private key")
	}

	prk, err := x509.ParseECPrivateKey(privateBlock.Bytes)
	if err != nil {
		return err
	}
	privateKey = prk

	publicBlock, _ := pem.Decode([]byte(c.LoginPublicKey))
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

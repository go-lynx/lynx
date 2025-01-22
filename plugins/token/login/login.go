package login

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
)

var (
	login *Login
)

type Login struct {
	conf       *Jwt
	privateKey *ecdsa.PrivateKey
	publicKey  *ecdsa.PublicKey
}

func (l *Login) Init(b config.Config) error {
	err := b.Scan(l.conf)
	if err != nil {
		return err
	}

	privateBlock, _ := pem.Decode([]byte(l.conf.GetLoginPrivateKey()))
	if privateBlock == nil {
		app.Lynx().GetLogHelper().Error("failed to parse PEM block containing the private key")
		panic("failed to parse PEM block containing the private key")
	}

	prk, err := x509.ParseECPrivateKey(privateBlock.Bytes)
	if err != nil {
		return err
	}
	l.privateKey = prk

	publicBlock, _ := pem.Decode([]byte(l.conf.GetLoginPublicKey()))
	if err != nil {
		return err
	}
	if publicBlock == nil {
		app.Lynx().GetLogHelper().Error("failed to parse PEM block containing the public key")
		panic("failed to parse PEM block containing the public key")
	}

	pubKeyInterface, err := x509.ParsePKIXPublicKey(publicBlock.Bytes)
	if err != nil {
		return err
	}
	puk := pubKeyInterface.(*ecdsa.PublicKey)
	l.publicKey = puk
	return nil
}

func NewLogin() *Login {
	login = &Login{
		conf: &Jwt{},
	}
	return login
}

func GetMethod() string {
	return login.conf.GetLoginMethod()
}

func GetPrivateKey() *ecdsa.PrivateKey {
	return login.privateKey
}

func GetPublicKey() *ecdsa.PublicKey {
	return login.publicKey
}

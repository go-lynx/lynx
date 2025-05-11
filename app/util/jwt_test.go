package util

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

	// Sign a JWT token
	signing, err := Sign(&LoginClaims{
		Id:       123,
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
	l := &LoginClaims{}
	check, err := Check(signing, l, *pubKey)
	if check {
		fmt.Printf("JWT public key verification: %d\n", l.Id)
	}
	if err != nil {
		panic(err)
	}
}

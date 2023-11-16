package sign

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

// Sign JWT token sing
func Sign(c CustomClaims, alg string, key *ecdsa.PrivateKey) (string, error) {
	err := c.Init()
	if err != nil {
		return "", err
	}

	err = c.Valid()
	if err != nil {
		return "", err
	}

	t := jwt.NewWithClaims(jwt.GetSigningMethod(alg), c)
	return t.SignedString(key)
}

// Check JWT token check
func Check(token string, c CustomClaims, key ecdsa.PublicKey) (bool, error) {
	parse, err := jwt.ParseWithClaims(token, c, func(token *jwt.Token) (interface{}, error) {
		return &key, nil
	})
	if err != nil {
		return false, err
	}
	err = c.Decoration()
	if err != nil {
		return false, err
	}
	return parse.Valid, nil
}

package jwt

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

type LoginClaims struct {
	Id       int64
	Nickname string
	Avatar   string
	jwt.RegisteredClaims
}

func (c *LoginClaims) Valid() error {
	if c.Id == 0 {
		return fmt.Errorf("LoginClaims id is empty")
	}
	if c.Nickname == "" {
		return fmt.Errorf("LoginClaims name is empty")
	}

	return nil
}

func (c *LoginClaims) Init() error {
	// set current sign time
	now := time.Now()
	c.IssuedAt = jwt.NewNumericDate(now)
	c.Issuer = "lynx"

	// exp expired
	if c.ExpiresAt != nil {
		if now.Unix() > c.ExpiresAt.Unix() {
			c.ExpiresAt = nil
		}
	}

	// exp not set, give a default value
	if c.ExpiresAt == nil {
		c.ExpiresAt = jwt.NewNumericDate(now.Add(time.Hour * 1))
	}
	return nil
}

func (c *LoginClaims) Decoration() error {
	return nil
}

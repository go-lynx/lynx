package sign

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

type LoginClaims struct {
	Id       int64
	Nickname string
	Avatar   string
	Num      string
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
	// 设置当前签名时间
	now := time.Now()
	c.IssuedAt = jwt.NewNumericDate(now)
	c.Issuer = "rc"

	// 如果载体中的 exp 存在并且已经过期 则直接给他去掉
	if c.ExpiresAt != nil {
		if now.Unix() > c.ExpiresAt.Unix() {
			c.ExpiresAt = nil
		}
	}

	// exp 如果没设置，默认设置 JWT 1小时过期时间
	if c.ExpiresAt == nil {
		c.ExpiresAt = jwt.NewNumericDate(now.Add(time.Hour * 1))
	}
	return nil
}

func (c *LoginClaims) Decoration() error {
	return nil
}

package util

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ============================================================
// Claims Interfaces
// ============================================================
type CustomClaims interface {
	Init() error
	Valid() error
	Decoration() error
	jwt.Claims
}

// ============================================================
// Signing Options
// ============================================================
// 签名相关选项
type SignOptions struct {
	// 可选：设置 JWT Header 的 kid，便于密钥轮换
	Kid string
}

// ============================================================
// Signing
// ============================================================
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
	m := jwt.GetSigningMethod(alg)
	if m == nil {
		return "", fmt.Errorf("unsupported signing method: %s", alg)
	}
	// key 为 ECDSA 时，强约束算法前缀为 ES*
	if !strings.HasPrefix(strings.ToUpper(alg), "ES") {
		return "", fmt.Errorf("signing method %s not compatible with ECDSA key", alg)
	}
	t := jwt.NewWithClaims(m, c)
	// 使用指定的私钥对 JWT 进行签名，并返回签名后的字符串
	return t.SignedString(key)
}

// SignJWT 是 Sign 的语义化别名，建议使用更清晰的命名。
func SignJWT(c CustomClaims, alg string, key *ecdsa.PrivateKey) (string, error) {
	return Sign(c, alg, key)
}

// SignWithOptions 支持设置 kid 等 Header 信息。
func SignWithOptions(c CustomClaims, alg string, key *ecdsa.PrivateKey, opts *SignOptions) (string, error) {
	// 基于 Sign 的安全校验逻辑
	if err := c.Init(); err != nil {
		return "", err
	}
	if err := c.Valid(); err != nil {
		return "", err
	}
	m := jwt.GetSigningMethod(alg)
	if m == nil {
		return "", fmt.Errorf("unsupported signing method: %s", alg)
	}
	if !strings.HasPrefix(strings.ToUpper(alg), "ES") {
		return "", fmt.Errorf("signing method %s not compatible with ECDSA key", alg)
	}
	t := jwt.NewWithClaims(m, c)
	if opts != nil && opts.Kid != "" {
		t.Header["kid"] = opts.Kid
	}
	return t.SignedString(key)
}

// ============================================================
// Errors and Decorators
// ============================================================
// 预定义错误
var (
	// ErrUnexpectedSigningMethod 当令牌签名算法与预期不一致时返回
	ErrUnexpectedSigningMethod = errors.New("unexpected signing method")
)

// Decorator 是可选接口：如实现则优先调用 Decorate()，否则回退调用 CustomClaims.Decoration()
type Decorator interface {
	Decorate() error
}

// decorate 优先调用 Decorate()，否则调用 c.Decoration()
func decorate(c CustomClaims) error {
	if d, ok := any(c).(Decorator); ok && d != nil {
		return d.Decorate()
	}
	return c.Decoration()
}

// ============================================================
// Verification (Preferred APIs)
// ============================================================
// Verify 验证 token（不强制算法一致性），与 Check 行为一致，便于规范命名迁移。
func Verify(token string, c CustomClaims, key ecdsa.PublicKey) (bool, error) {
	return Check(token, c, key)
}

// VerifyWithAlg 验证 token 并强制签名算法与 expectedAlg 一致（推荐在生产环境使用）。
func VerifyWithAlg(token string, c CustomClaims, key ecdsa.PublicKey, expectedAlg string) (bool, error) {
	return CheckSecure(token, c, key, expectedAlg)
}

// VerifyWithKeyFunc 验证 token 并使用自定义 keyfunc。
func VerifyWithKeyFunc(token string, c CustomClaims, expectedAlg string, keyFunc jwt.Keyfunc) (bool, error) {
    if keyFunc == nil {
        return false, errors.New("keyFunc cannot be nil")
    }

    wrapped := func(tok *jwt.Token) (interface{}, error) {
        if expectedAlg != "" {
            if tok.Method == nil || tok.Method.Alg() != expectedAlg {
                return nil, ErrUnexpectedSigningMethod
            }
        }
        return keyFunc(tok)
    }

    parsed, err := jwt.ParseWithClaims(token, c, wrapped)
    if err != nil {
        return false, err
    }
    if err := decorate(c); err != nil {
        return false, err
    }
    return parsed.Valid, nil
}

// ============================================================
// Verification (Legacy APIs)
// ============================================================
// Check 方法用于验证一个 JWT 令牌的有效性
// Deprecated: 请使用 Verify 或 VerifyWithAlg 替代，以获得更清晰的命名与更安全的默认行为。
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
    if err := decorate(c); err != nil {
        return false, err
    }
    // 返回解析结果是否有效
    return parse.Valid, nil
}

// CheckSecure 提供更严格的 JWT 校验：
// 1) 强制校验签名算法与 expectedAlg 一致（防止算法降级/误配）。
// 2) 完成 claims.Decoration() 的增强处理。
// 注意：expectedAlg 例如 "ES256"。当 expectedAlg 为空时，不做算法强校验。
// Deprecated: 请使用 VerifyWithAlg 替代。
func CheckSecure(token string, c CustomClaims, key ecdsa.PublicKey, expectedAlg string) (bool, error) {
	// 使用自定义 keyfunc，在其中做算法一致性检查。
	keyFunc := func(tok *jwt.Token) (interface{}, error) {
		if expectedAlg != "" {
			if tok.Method == nil || tok.Method.Alg() != expectedAlg {
				return nil, ErrUnexpectedSigningMethod
			}
		}
		return &key, nil
	}

	parsed, err := jwt.ParseWithClaims(token, c, keyFunc)
	if err != nil {
		return false, err
	}

	if err := decorate(c); err != nil {
		return false, err
	}

	return parsed.Valid, nil
}

// CheckWithKeyFunc 提供基于自定义 KeyFunc 的严格校验入口：
// 允许调用方通过 token header(kid) 等动态选择公钥，并可同时指定 expectedAlg 做算法校验。
// 示例：
//  keyFunc := func(t *jwt.Token) (interface{}, error) {
//      // 根据 t.Header["kid"] 动态查找并返回相应公钥
//      return &pubKey, nil
//  }
// Deprecated: 请使用 VerifyWithKeyFunc 替代。
func CheckWithKeyFunc(token string, c CustomClaims, expectedAlg string, keyFunc jwt.Keyfunc) (bool, error) {
    if keyFunc == nil {
        return false, errors.New("keyFunc cannot be nil")
    }

    wrapped := func(tok *jwt.Token) (interface{}, error) {
        if expectedAlg != "" {
            if tok.Method == nil || tok.Method.Alg() != expectedAlg {
                return nil, ErrUnexpectedSigningMethod
            }
        }
        return keyFunc(tok)
    }

    parsed, err := jwt.ParseWithClaims(token, c, wrapped)
    if err != nil {
        return false, err
    }
    if err := decorate(c); err != nil {
        return false, err
    }
    return parsed.Valid, nil
}

 

// VerifyOptions 提供更强的可配置验证能力
type VerifyOptions struct {
	// 期望算法（如 ES256），为空则不强制
	ExpectedAlg string
	// 期望发行者与受众，可选
	ExpectedIss string
	ExpectedAud string
	// 时钟偏移，默认 0
	Leeway time.Duration
	// 二选一：KeyFunc 或 PublicKey 必须至少提供一个
	KeyFunc   jwt.Keyfunc
	PublicKey *ecdsa.PublicKey
}

// VerifyWithOptions 使用 jwt/v5 Parser 选项做统一校验
func VerifyWithOptions(token string, c CustomClaims, opts VerifyOptions) (bool, error) {
	// 构造 keyfunc
	var kf jwt.Keyfunc
	if opts.KeyFunc != nil {
		kf = func(t *jwt.Token) (interface{}, error) {
			if opts.ExpectedAlg != "" {
				if t.Method == nil || t.Method.Alg() != opts.ExpectedAlg {
					return nil, ErrUnexpectedSigningMethod
				}
			}
			return opts.KeyFunc(t)
		}
	} else if opts.PublicKey != nil {
		expectedAlg := opts.ExpectedAlg
		kf = func(t *jwt.Token) (interface{}, error) {
			if expectedAlg != "" {
				if t.Method == nil || t.Method.Alg() != expectedAlg {
					return nil, ErrUnexpectedSigningMethod
				}
			}
			return opts.PublicKey, nil
		}
	} else {
		return false, errors.New("either KeyFunc or PublicKey must be provided")
	}

	// 组装 parser 选项
	var parseOpts []jwt.ParserOption
	if opts.ExpectedAlg != "" {
		parseOpts = append(parseOpts, jwt.WithValidMethods([]string{opts.ExpectedAlg}))
	}
	if opts.ExpectedIss != "" {
		parseOpts = append(parseOpts, jwt.WithIssuer(opts.ExpectedIss))
	}
	if opts.ExpectedAud != "" {
		parseOpts = append(parseOpts, jwt.WithAudience(opts.ExpectedAud))
	}
	if opts.Leeway > 0 {
		parseOpts = append(parseOpts, jwt.WithLeeway(opts.Leeway))
	}

	parsed, err := jwt.ParseWithClaims(token, c, kf, parseOpts...)
	if err != nil {
		return false, err
	}
	if err := decorate(c); err != nil {
		return false, err
	}
	return parsed.Valid, nil
}

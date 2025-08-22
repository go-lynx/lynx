package jwt

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
// CustomClaims defines a custom claims interface.
// Constraints: caller must implement Init, Valid, and Decoration, and also implement jwt.Claims.
//
// Example usage:
//  type MyClaims struct { jwt.RegisteredClaims; ... }
//  func (m *MyClaims) Init() error { ... }
//  func (m *MyClaims) Valid() error { ... }
//  func (m *MyClaims) Decoration() error { ... }
//  func (m *MyClaims) GetExpirationTime() (*jwt.NumericDate, error) { return m.ExpiresAt, nil }
//  ...
//
//  token, _ := jwt.Sign(&myClaims, "ES256", privateKey)
//
//  ok, _ := jwt.Verify(token, &myClaims, *publicKey)
//  ok, _ := jwt.VerifyWithAlg(token, &myClaims, *publicKey, "ES256")
//  ok, _ := jwt.VerifyWithOptions(token, &myClaims, jwt.VerifyOptions{ExpectedAlg: "ES256", PublicKey: publicKey})
//
// Note: jwt refers to github.com/golang-jwt/jwt/v5.
//
//go:generate echo "This is a placeholder for possible future codegen"

type CustomClaims interface {
	Init() error
	Valid() error
	Decoration() error
	jwt.Claims
}

// ============================================================
// Signing Options
// ============================================================
// Options for signing
type SignOptions struct {
	// Optional: set JWT header "kid" for key rotation
	Kid string
}

// ============================================================
// Signing
// ============================================================
// Sign generates a JWT token
func Sign(c CustomClaims, alg string, key *ecdsa.PrivateKey) (string, error) {
	// Initialize custom claims
	err := c.Init()
	if err != nil {
		return "", err
	}
	// Validate custom claims
	if err = c.Valid(); err != nil {
		return "", err
	}
	m := jwt.GetSigningMethod(alg)
	if m == nil {
		return "", fmt.Errorf("unsupported signing method: %s", alg)
	}
	// With ECDSA keys, enforce algorithms with ES* prefix
	if !strings.HasPrefix(strings.ToUpper(alg), "ES") {
		return "", fmt.Errorf("signing method %s not compatible with ECDSA key", alg)
	}
	t := jwt.NewWithClaims(m, c)
	return t.SignedString(key)
}

// SignJWT is a semantic alias for Sign
func SignJWT(c CustomClaims, alg string, key *ecdsa.PrivateKey) (string, error) {
	return Sign(c, alg, key)
}

// SignWithOptions supports setting header fields such as kid
func SignWithOptions(c CustomClaims, alg string, key *ecdsa.PrivateKey, opts *SignOptions) (string, error) {
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
var (
	// ErrUnexpectedSigningMethod is returned when the token signing method doesn't match expectation
	ErrUnexpectedSigningMethod = errors.New("unexpected signing method")
)

// Decorator is optional: if implemented, Decorate() is preferred; otherwise fall back to CustomClaims.Decoration()
type Decorator interface {
	Decorate() error
}

// decorate prefers Decorate() and otherwise calls c.Decoration()
func decorate(c CustomClaims) error {
	if d, ok := any(c).(Decorator); ok && d != nil {
		return d.Decorate()
	}
	return c.Decoration()
}

// ============================================================
// Verification (Preferred APIs)
// ============================================================
// Verify validates a token without enforcing algorithm consistency (same behavior as Check)
func Verify(token string, c CustomClaims, key ecdsa.PublicKey) (bool, error) {
	return Check(token, c, key)
}

// VerifyWithAlg validates a token and enforces the signing algorithm to match expectedAlg (recommended in production)
func VerifyWithAlg(token string, c CustomClaims, key ecdsa.PublicKey, expectedAlg string) (bool, error) {
	return CheckSecure(token, c, key, expectedAlg)
}

// VerifyWithKeyFunc validates a token using a custom keyfunc
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
// Check validates a JWT token.
// Deprecated: use Verify or VerifyWithAlg for clearer naming and safer defaults.
func Check(token string, c CustomClaims, key ecdsa.PublicKey) (bool, error) {
	parse, err := jwt.ParseWithClaims(token, c, func(token *jwt.Token) (interface{}, error) {
		return &key, nil
	})
	if err != nil {
		return false, err
	}
	if err := decorate(c); err != nil {
		return false, err
	}
	return parse.Valid, nil
}

// CheckSecure provides stricter JWT validation:
// 1) Enforces signing algorithm to match expectedAlg (prevents downgrade/misconfiguration).
// 2) Applies claims.Decoration() for post-processing.
// Note: expectedAlg like "ES256". If empty, algorithm enforcement is skipped.
// Deprecated: use VerifyWithAlg instead.
func CheckSecure(token string, c CustomClaims, key ecdsa.PublicKey, expectedAlg string) (bool, error) {
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

// CheckWithKeyFunc provides strict validation with a custom KeyFunc:
// allows selecting public keys dynamically (e.g., via header kid) and enforcing expectedAlg.
// Deprecated: use VerifyWithKeyFunc instead.
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

// VerifyOptions provides configurable verification options
type VerifyOptions struct {
	ExpectedAlg string
	ExpectedIss string
	ExpectedAud string
	Leeway      time.Duration
	KeyFunc     jwt.Keyfunc
	PublicKey   *ecdsa.PublicKey
}

// VerifyWithOptions validates using jwt/v5 parser options
func VerifyWithOptions(token string, c CustomClaims, opts VerifyOptions) (bool, error) {
	// Build keyfunc
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

	// Assemble parser options
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

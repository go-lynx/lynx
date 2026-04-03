package jwt

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CustomClaims ============================================================
// Claims Interfaces
// ============================================================
// CustomClaims defines a custom claims interface.
// Constraints: caller must implement Init, Valid, and Decoration, and also implement jwt.Claims.
//
// Example usage:
//
//	type MyClaims struct { jwt.RegisteredClaims; ... }
//	func (m *MyClaims) Init() error { ... }
//	func (m *MyClaims) Valid() error { ... }
//	func (m *MyClaims) Decoration() error { ... }
//	func (m *MyClaims) GetExpirationTime() (*jwt.NumericDate, error) { return m.ExpiresAt, nil }
//	...
//
//	// ECDSA
//	token, _ := jwt.Sign(&myClaims, "ES256", privateKey)
//	ok, _    := jwt.Verify(token, &myClaims, *publicKey)
//
//	// RSA
//	token, _ := jwt.SignWithKey(&myClaims, jwt.RSAKey(rsaPrivateKey))
//	ok, _    := jwt.VerifyWithKey(token, &myClaims, jwt.RSAPublicKey(rsaPublicKey))
//
//	// HMAC
//	token, _ := jwt.SignWithKey(&myClaims, jwt.HMACKey([]byte("secret")))
//	ok, _    := jwt.VerifyWithKey(token, &myClaims, jwt.HMACKey([]byte("secret")))
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
// SigningKey – algorithm-agnostic key abstraction
// ============================================================

// SigningKey wraps an algorithm name together with its signing and verification
// keys so that callers do not have to repeat the algorithm string everywhere.
type SigningKey interface {
	// Algorithm returns the JWT signing algorithm name (e.g. "ES256", "RS256", "HS256").
	Algorithm() string
	// PrivateKey returns the key used for signing (may equal the public key for symmetric algorithms).
	PrivateKey() any
	// PublicKey returns the key used for verification.
	PublicKey() any
}

// ECDSAKey builds a SigningKey from an ECDSA private key.
// The public key is derived automatically; the algorithm is chosen based on
// the curve size (P-256 → ES256, P-384 → ES384, P-521 → ES512).
func ECDSAKey(priv *ecdsa.PrivateKey) SigningKey {
	alg := ecdsaAlgorithm(&priv.PublicKey)
	return &fixedSigningKey{alg: alg, priv: priv, pub: &priv.PublicKey}
}

// ECDSAPublicKey builds a verification-only SigningKey from an ECDSA public key.
func ECDSAPublicKey(pub *ecdsa.PublicKey) SigningKey {
	return &fixedSigningKey{alg: ecdsaAlgorithm(pub), pub: pub}
}

// RSAKey builds a SigningKey from an RSA private key.
// The algorithm defaults to RS256; pass rsaKeyWithAlg to use RS384 or RS512.
func RSAKey(priv *rsa.PrivateKey) SigningKey {
	return &fixedSigningKey{alg: "RS256", priv: priv, pub: &priv.PublicKey}
}

// RSAKeyWithAlg builds a SigningKey from an RSA private key using the specified algorithm
// ("RS256", "RS384", "RS512", "PS256", "PS384", "PS512").
func RSAKeyWithAlg(priv *rsa.PrivateKey, alg string) SigningKey {
	return &fixedSigningKey{alg: alg, priv: priv, pub: &priv.PublicKey}
}

// RSAPublicKey builds a verification-only SigningKey from an RSA public key (RS256).
func RSAPublicKey(pub *rsa.PublicKey) SigningKey {
	return &fixedSigningKey{alg: "RS256", pub: pub}
}

// RSAPublicKeyWithAlg builds a verification-only RSA SigningKey with the specified algorithm.
func RSAPublicKeyWithAlg(pub *rsa.PublicKey, alg string) SigningKey {
	return &fixedSigningKey{alg: alg, pub: pub}
}

// HMACKey builds a symmetric SigningKey from a raw secret (HS256).
// For HS384 or HS512 use HMACKeyWithAlg.
func HMACKey(secret []byte) SigningKey {
	return &fixedSigningKey{alg: "HS256", priv: secret, pub: secret}
}

// HMACKeyWithAlg builds a symmetric SigningKey with the specified algorithm
// ("HS256", "HS384", "HS512").
func HMACKeyWithAlg(secret []byte, alg string) SigningKey {
	return &fixedSigningKey{alg: alg, priv: secret, pub: secret}
}

// fixedSigningKey is the internal implementation of SigningKey.
type fixedSigningKey struct {
	alg  string
	priv any
	pub  any
}

func (k *fixedSigningKey) Algorithm() string { return k.alg }
func (k *fixedSigningKey) PrivateKey() any   { return k.priv }
func (k *fixedSigningKey) PublicKey() any    { return k.pub }

// ecdsaAlgorithm selects the ECDSA algorithm name from the curve bit size.
func ecdsaAlgorithm(pub *ecdsa.PublicKey) string {
	switch pub.Curve.Params().BitSize {
	case 384:
		return "ES384"
	case 521:
		return "ES512"
	default: // 256
		return "ES256"
	}
}

// ============================================================
// SignOptions
// ============================================================

// SignOptions holds optional parameters for JWT signing.
type SignOptions struct {
	// Kid sets the JWT header "kid" field for key rotation.
	Kid string
}

// ============================================================
// Signing – algorithm-agnostic API (preferred)
// ============================================================

// SignWithKey generates a JWT token using the provided SigningKey.
// This is the preferred signing API as it supports ECDSA, RSA, and HMAC.
func SignWithKey(c CustomClaims, key SigningKey) (string, error) {
	return SignWithKeyAndOptions(c, key, nil)
}

// SignWithKeyAndOptions generates a JWT token with additional header options.
func SignWithKeyAndOptions(c CustomClaims, key SigningKey, opts *SignOptions) (string, error) {
	if err := c.Init(); err != nil {
		return "", err
	}
	if err := c.Valid(); err != nil {
		return "", err
	}
	m := jwt.GetSigningMethod(key.Algorithm())
	if m == nil {
		return "", fmt.Errorf("unsupported signing method: %s", key.Algorithm())
	}
	t := jwt.NewWithClaims(m, c)
	if opts != nil && opts.Kid != "" {
		t.Header["kid"] = opts.Kid
	}
	return t.SignedString(key.PrivateKey())
}

// VerifyWithKey validates a JWT token using the provided SigningKey.
// This is the preferred verification API as it supports ECDSA, RSA, and HMAC.
func VerifyWithKey(token string, c CustomClaims, key SigningKey) (bool, error) {
	return VerifyWithOptions(token, c, VerifyOptions{
		ExpectedAlg: key.Algorithm(),
		verifyKey:   key.PublicKey(),
	})
}

// ============================================================
// Signing – ECDSA-only API (preserved for backward compatibility)
// ============================================================

// Sign generates a JWT token using ECDSA.
//
// Deprecated: use SignWithKey(c, ECDSAKey(privateKey)) instead, which
// selects the algorithm automatically from the curve.
func Sign(c CustomClaims, alg string, key *ecdsa.PrivateKey) (string, error) {
	// Initialize custom claims
	if err := c.Init(); err != nil {
		return "", err
	}
	// Validate custom claims
	if err := c.Valid(); err != nil {
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

// SignJWT is a semantic alias for Sign.
//
// Deprecated: use SignWithKey instead.
func SignJWT(c CustomClaims, alg string, key *ecdsa.PrivateKey) (string, error) {
	return Sign(c, alg, key)
}

// SignWithOptions supports setting header fields such as kid.
//
// Deprecated: use SignWithKeyAndOptions(c, ECDSAKey(key), opts) instead.
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
	// ErrUnexpectedSigningMethod is returned when the token signing method doesn't match expectation.
	ErrUnexpectedSigningMethod = errors.New("unexpected signing method")
)

// Decorator is optional: if implemented, Decorate() is preferred; otherwise fall back to CustomClaims.Decoration().
type Decorator interface {
	Decorate() error
}

// decorate prefers Decorate() and otherwise calls c.Decoration().
func decorate(c CustomClaims) error {
	if d, ok := any(c).(Decorator); ok && d != nil {
		return d.Decorate()
	}
	return c.Decoration()
}

// ============================================================
// Verification – algorithm-agnostic API (preferred)
// ============================================================

// VerifyOptions provides configurable verification options.
type VerifyOptions struct {
	// ExpectedAlg enforces the signing algorithm (e.g. "ES256", "RS256", "HS256").
	ExpectedAlg string
	// ExpectedIss enforces the token issuer.
	ExpectedIss string
	// ExpectedAud enforces the token audience.
	ExpectedAud string
	// Leeway allows a small clock skew when validating time-based claims.
	Leeway time.Duration
	// KeyFunc provides dynamic key selection (takes priority over PublicKey / verifyKey).
	KeyFunc jwt.Keyfunc
	// PublicKey is used for ECDSA verification (kept for backward compatibility).
	// Prefer verifyKey for multi-algorithm support.
	PublicKey *ecdsa.PublicKey

	// verifyKey is the internal field used by VerifyWithKey; it accepts any key type.
	verifyKey any
}

// VerifyWithOptions validates a JWT token using the supplied options.
// Supports ECDSA, RSA, and HMAC keys via KeyFunc or the PublicKey / verifyKey fields.
func VerifyWithOptions(token string, c CustomClaims, opts VerifyOptions) (bool, error) {
	var kf jwt.Keyfunc
	switch {
	case opts.KeyFunc != nil:
		kf = func(t *jwt.Token) (any, error) {
			if opts.ExpectedAlg != "" {
				if t.Method == nil || t.Method.Alg() != opts.ExpectedAlg {
					return nil, ErrUnexpectedSigningMethod
				}
			}
			return opts.KeyFunc(t)
		}
	case opts.verifyKey != nil:
		expectedAlg := opts.ExpectedAlg
		vk := opts.verifyKey
		kf = func(t *jwt.Token) (any, error) {
			if expectedAlg != "" {
				if t.Method == nil || t.Method.Alg() != expectedAlg {
					return nil, ErrUnexpectedSigningMethod
				}
			}
			return vk, nil
		}
	case opts.PublicKey != nil:
		expectedAlg := opts.ExpectedAlg
		kf = func(t *jwt.Token) (any, error) {
			if expectedAlg != "" {
				if t.Method == nil || t.Method.Alg() != expectedAlg {
					return nil, ErrUnexpectedSigningMethod
				}
			}
			return opts.PublicKey, nil
		}
	default:
		return false, errors.New("one of KeyFunc, PublicKey, or a SigningKey must be provided")
	}

	// Assemble parser options.
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

// ============================================================
// Verification – ECDSA-only API (preserved for backward compatibility)
// ============================================================

// Verify validates a token without enforcing algorithm consistency (same behavior as Check).
//
// Deprecated: use VerifyWithKey or VerifyWithOptions for clearer naming and multi-algorithm support.
func Verify(token string, c CustomClaims, key ecdsa.PublicKey) (bool, error) {
	return Check(token, c, key)
}

// VerifyWithAlg validates a token and enforces the signing algorithm to match expectedAlg (recommended in production).
//
// Deprecated: use VerifyWithKey or VerifyWithOptions.
func VerifyWithAlg(token string, c CustomClaims, key ecdsa.PublicKey, expectedAlg string) (bool, error) {
	return CheckSecure(token, c, key, expectedAlg)
}

// VerifyWithKeyFunc validates a token using a custom keyfunc.
func VerifyWithKeyFunc(token string, c CustomClaims, expectedAlg string, keyFunc jwt.Keyfunc) (bool, error) {
	if keyFunc == nil {
		return false, errors.New("keyFunc cannot be nil")
	}

	wrapped := func(tok *jwt.Token) (any, error) {
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
// Verification – Legacy APIs
// ============================================================

// Check validates a JWT token.
//
// Deprecated: use VerifyWithKey or VerifyWithAlg for clearer naming and safer defaults.
func Check(token string, c CustomClaims, key ecdsa.PublicKey) (bool, error) {
	parse, err := jwt.ParseWithClaims(token, c, func(token *jwt.Token) (any, error) {
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
//
// Deprecated: use VerifyWithAlg instead.
func CheckSecure(token string, c CustomClaims, key ecdsa.PublicKey, expectedAlg string) (bool, error) {
	keyFunc := func(tok *jwt.Token) (any, error) {
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
//
// Deprecated: use VerifyWithKeyFunc instead.
func CheckWithKeyFunc(token string, c CustomClaims, expectedAlg string, keyFunc jwt.Keyfunc) (bool, error) {
	if keyFunc == nil {
		return false, errors.New("keyFunc cannot be nil")
	}

	wrapped := func(tok *jwt.Token) (any, error) {
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

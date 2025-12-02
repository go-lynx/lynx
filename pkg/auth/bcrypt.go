package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword generates a bcrypt hash for the given password.
// - Uses bcrypt.MinCost / DefaultCost / MaxCost to bound the cost and avoid magic numbers.
// - When cost < MinCost, falls back to DefaultCost; when cost > MaxCost, caps at MaxCost.
func HashPassword(password string, cost int) (string, error) {
	if cost < bcrypt.MinCost {
		cost = bcrypt.DefaultCost
	} else if cost > bcrypt.MaxCost {
		cost = bcrypt.MaxCost
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(b), err
}

// VerifyPassword compares a bcrypt hash with a plaintext password.
// - Returns nil if they match; returns an error if they do not match or hash is invalid.
func VerifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// CheckPassword is a boolean wrapper for VerifyPassword.
func CheckPassword(hash, password string) bool {
	return VerifyPassword(hash, password) == nil
}

package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword 使用 bcrypt 生成密码哈希。
// - 使用 bcrypt.MinCost / DefaultCost / MaxCost 管理 cost 边界，避免魔法数。
// - 当 cost < MinCost 时采用 DefaultCost；当 cost > MaxCost 时采用 MaxCost。
func HashPassword(password string, cost int) (string, error) {
	if cost < bcrypt.MinCost {
		cost = bcrypt.DefaultCost
	} else if cost > bcrypt.MaxCost {
		cost = bcrypt.MaxCost
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(b), err
}

// VerifyPassword 比较哈希与明文是否匹配。
// - 匹配返回 nil；不匹配或哈希损坏返回 error。
func VerifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// CheckPassword 是 VerifyPassword 的布尔包装器。
func CheckPassword(hash, password string) bool {
	return VerifyPassword(hash, password) == nil
}

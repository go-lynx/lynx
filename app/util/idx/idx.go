package idx

import "github.com/go-lynx/lynx/app/util/randx"

// NanoID 生成 URL 安全的短 ID，长度为 n。默认字符集与 RandString 相同。
// 典型长度为 21（~128bit 熵）。
func NanoID(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	return randx.RandString(n, "")
}

// DefaultNanoID 生成长度为 21 的短 ID。
func DefaultNanoID() (string, error) { return NanoID(21) }

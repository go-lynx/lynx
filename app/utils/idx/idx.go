package idx

import "github.com/go-lynx/lynx/app/utils/randx"

// NanoID generates a URL-safe short ID of length n. The default alphabet matches RandString.
// A typical length is 21 (~128-bit entropy).
func NanoID(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	return randx.RandString(n, "")
}

// DefaultNanoID generates a short ID of length 21.
func DefaultNanoID() (string, error) { return NanoID(21) }

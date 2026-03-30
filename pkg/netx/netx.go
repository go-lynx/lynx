package netx

import (
	"errors"
	"net"
	"time"
)

// IsTemporary reports whether the error is a temporary network error.
// Note: net.Error.Temporary() is deprecated as of Go 1.18. This function is
// retained for backward compatibility; callers should prefer checking the specific
// error type or using errors.Is/As with known sentinel errors instead.
//
// Deprecated: net.Error.Temporary() is deprecated in Go 1.18+. Use IsTimeout or
// check specific error types instead.
func IsTemporary(err error) bool { //nolint:staticcheck // intentionally retained for compat
	var ne net.Error
	if errors.As(err, &ne) {
		return ne.Temporary() //nolint:staticcheck // deprecated but retained for compat
	}
	return false
}

// IsTimeout reports whether the error is a timeout.
func IsTimeout(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) {
		return ne.Timeout()
	}
	return false
}

// WaitPort keeps trying to connect to addr (host:port) until success or timeout.
func WaitPort(addr string, timeout time.Duration) error {
	if timeout <= 0 {
		// Try once immediately
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
		}
		return err
	}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	return lastErr
}

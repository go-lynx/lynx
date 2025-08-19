package netx

import (
	"errors"
	"net"
	"time"
)

// IsTemporary 判断是否为临时性网络错误。
func IsTemporary(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) {
		return ne.Temporary()
	}
	return false
}

// IsTimeout 判断是否为超时错误。
func IsTimeout(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) {
		return ne.Timeout()
	}
	return false
}

// WaitPort 在指定超时时间内尝试连接 addr（host:port），直到成功或超时。
func WaitPort(addr string, timeout time.Duration) error {
	if timeout <= 0 {
		// 直接尝试一次
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

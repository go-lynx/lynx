package netx

import (
	"net"
	"testing"
	"time"
)

type tempTimeoutErr struct{}

func (tempTimeoutErr) Error() string   { return "tmp" }
func (tempTimeoutErr) Timeout() bool   { return true }
func (tempTimeoutErr) Temporary() bool { return true }

func TestNetX(t *testing.T) {
	var e net.Error = tempTimeoutErr{}
	if !IsTimeout(e) || !IsTemporary(e) {
		t.Fatalf("timeout/temporary detection failed")
	}

	// success case
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
	}()
	if err := WaitPort(addr, time.Second); err != nil {
		t.Fatalf("WaitPort success: %v", err)
	}

	// timeout case: pick an unlikely port without listener
	bad := "127.0.0.1:1" // privileged port; usually no listener in tests
	if err := WaitPort(bad, 300*time.Millisecond); err == nil {
		t.Fatalf("WaitPort should timeout")
	}
}

package netx

import (
	"errors"
	"net"
	"testing"
	"time"
)

type tempTimeoutErr struct{}

func (tempTimeoutErr) Error() string   { return "tmp" }
func (tempTimeoutErr) Timeout() bool   { return true }
func (tempTimeoutErr) Temporary() bool { return true } //nolint:staticcheck

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

// ---- Additional coverage ----

func TestIsTimeout_NilError(t *testing.T) {
	if IsTimeout(nil) {
		t.Error("IsTimeout(nil) should return false")
	}
}

func TestIsTemporary_NilError(t *testing.T) {
	if IsTemporary(nil) { //nolint:staticcheck
		t.Error("IsTemporary(nil) should return false")
	}
}

func TestIsTimeout_NonNetError(t *testing.T) {
	if IsTimeout(errors.New("plain error")) {
		t.Error("IsTimeout should return false for non-net.Error")
	}
}

func TestIsTemporary_NonNetError(t *testing.T) {
	if IsTemporary(errors.New("plain error")) { //nolint:staticcheck
		t.Error("IsTemporary should return false for non-net.Error")
	}
}

type timeoutOnlyErr struct{}

func (timeoutOnlyErr) Error() string   { return "timeout-only" }
func (timeoutOnlyErr) Timeout() bool   { return true }
func (timeoutOnlyErr) Temporary() bool { return false } //nolint:staticcheck

func TestIsTimeout_TrueIsTemporary_False(t *testing.T) {
	var e net.Error = timeoutOnlyErr{}
	if !IsTimeout(e) {
		t.Error("IsTimeout should be true for timeoutOnlyErr")
	}
	if IsTemporary(e) { //nolint:staticcheck
		t.Error("IsTemporary should be false for timeoutOnlyErr")
	}
}

func TestWaitPort_ZeroTimeoutTriesOnce(t *testing.T) {
	// With a closed port and zero timeout, WaitPort should try once and return an error.
	err := WaitPort("127.0.0.1:1", 0)
	if err == nil {
		t.Error("expected error connecting to unavailable port with zero timeout")
	}
}

func TestWaitPort_ImmediateSuccess(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		c, _ := ln.Accept()
		if c != nil {
			c.Close()
		}
	}()

	if err := WaitPort(ln.Addr().String(), 500*time.Millisecond); err != nil {
		t.Errorf("WaitPort should succeed immediately for open port: %v", err)
	}
}

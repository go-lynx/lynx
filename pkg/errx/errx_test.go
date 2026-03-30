package errx

import (
	"errors"
	"testing"
)

func TestErrX(t *testing.T) {
	if All(nil, nil) != nil {
		t.Fatalf("All nil -> nil")
	}
	e1 := errors.New("e1")
	e2 := errors.New("e2")
	j := All(e1, nil, e2)
	if j == nil {
		t.Fatalf("All should join")
	}
	if First(nil, e2) != e2 {
		t.Fatalf("First pick first non-nil")
	}

	w := Wrap(e1, "ctx")
	if !errors.Is(w, e1) {
		t.Fatalf("Wrap should keep original")
	}

	var caught any
	func() {
		defer DeferRecover(func(e any) { caught = e })
		panic("boom")
	}()
	if caught == nil {
		t.Fatalf("DeferRecover didn't catch")
	}
}

// ---- Additional coverage ----

func TestAll_AllNil(t *testing.T) {
	if All() != nil {
		t.Error("All() with no args should return nil")
	}
	if All(nil, nil, nil) != nil {
		t.Error("All(nil,nil,nil) should return nil")
	}
}

func TestAll_SingleError(t *testing.T) {
	e := errors.New("solo")
	got := All(e)
	if got == nil {
		t.Fatal("All with single error should return non-nil")
	}
	if !errors.Is(got, e) {
		t.Error("All should preserve original error for single-error case")
	}
}

func TestFirst_AllNil(t *testing.T) {
	if First(nil, nil) != nil {
		t.Error("First(nil,nil) should return nil")
	}
	if First() != nil {
		t.Error("First() with no args should return nil")
	}
}

func TestFirst_ReturnsFirstNonNil(t *testing.T) {
	e1 := errors.New("first")
	e2 := errors.New("second")
	if got := First(nil, nil, e1, e2); got != e1 {
		t.Errorf("expected e1, got %v", got)
	}
}

func TestWrap_NilError(t *testing.T) {
	if Wrap(nil, "ctx") != nil {
		t.Error("Wrap(nil, ...) should return nil")
	}
}

func TestWrap_EmptyMsg(t *testing.T) {
	e := errors.New("orig")
	wrapped := Wrap(e, "")
	if wrapped != e {
		t.Error("Wrap with empty msg should return original error unchanged")
	}
}

func TestWrap_MessageIncluded(t *testing.T) {
	e := errors.New("base")
	wrapped := Wrap(e, "prefix")
	if wrapped == nil {
		t.Fatal("Wrap should return non-nil error")
	}
	if !errors.Is(wrapped, e) {
		t.Error("wrapped error should unwrap to original")
	}
}

func TestDeferRecover_NilHandler(t *testing.T) {
	// Should not panic even if handler is nil
	func() {
		defer DeferRecover(nil)
		panic("test panic")
	}()
}

func TestDeferRecover_NoPanic(t *testing.T) {
	called := false
	func() {
		defer DeferRecover(func(any) { called = true })
		// No panic
	}()
	if called {
		t.Error("DeferRecover handler should not be called when there is no panic")
	}
}

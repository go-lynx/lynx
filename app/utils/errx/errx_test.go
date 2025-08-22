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

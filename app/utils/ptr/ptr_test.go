package ptr

import "testing"

func TestPtrDerefOrDefault(t *testing.T) {
	p := Ptr(10)
	if *p != 10 {
		t.Fatalf("Ptr: got %d", *p)
	}

	if got := Deref[int](nil, 7); got != 7 {
		t.Fatalf("Deref nil: got %d", got)
	}
	if got := Deref(p, 0); got != 10 {
		t.Fatalf("Deref: got %d", got)
	}

	if OrDefault(0, 5) != 5 {
		t.Fatalf("OrDefault zero -> def")
	}
	if OrDefault(3, 5) != 3 {
		t.Fatalf("OrDefault non-zero -> self")
	}
}

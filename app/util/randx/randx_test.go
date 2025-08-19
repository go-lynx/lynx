package randx

import "testing"

func TestRandX(t *testing.T) {
	b, err := CryptoBytes(16)
	if err != nil || len(b) != 16 {
		t.Fatalf("CryptoBytes len: %v %d", err, len(b))
	}
	if _, err := CryptoBytes(-1); err == nil {
		t.Fatalf("CryptoBytes negative should error")
	}

	s, err := RandString(8, "")
	if err != nil || len([]rune(s)) != 8 {
		t.Fatalf("RandString len: %v %q", err, s)
	}
	if _, err := RandString(1, ""); err != nil { /* alphabet default, ok */
	}
	if _, err := RandString(1, ""); err != nil {
		t.Fatalf("RandString unexpected error: %v", err)
	}
	if _, err := RandString(-1, ""); err == nil {
		t.Fatalf("RandString negative should error")
	}
}

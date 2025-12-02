package idx

import "testing"

func TestIDX(t *testing.T) {
	id, err := NanoID(10)
	if err != nil || len([]rune(id)) != 10 {
		t.Fatalf("NanoID len")
	}
	id, err = DefaultNanoID()
	if err != nil || len([]rune(id)) != 21 {
		t.Fatalf("DefaultNanoID len")
	}
}

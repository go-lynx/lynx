package collection

import "testing"

func TestMapX(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2}
	ks := Keys(m)
	if len(ks) != 2 {
		t.Fatalf("Keys len")
	}
	vs := Values(m)
	if len(vs) != 2 {
		t.Fatalf("Values len")
	}

	dst := Merge(map[string]int{"a": 1}, map[string]int{"b": 3})
	if len(dst) != 2 || dst["b"] != 3 {
		t.Fatalf("Merge failed: %v", dst)
	}

	inv := Invert(map[string]int{"x": 1, "y": 2})
	if inv[1] != "x" || inv[2] != "y" {
		t.Fatalf("Invert failed: %v", inv)
	}
}

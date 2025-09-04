package collection

import "testing"

func TestSet(t *testing.T) {
	s := NewSet(1, 2)
	s.Add(3)
	if !s.Has(2) || s.Len() != 3 {
		t.Fatalf("Set basic failed")
	}
	s.Del(2)
	if s.Has(2) {
		t.Fatalf("Set Del failed")
	}

	u := s.Union(NewSet(3, 4))
	if !u.Has(4) || !u.Has(1) {
		t.Fatalf("Union failed: %v", u)
	}
	i := s.Intersect(NewSet(3, 5))
	if !i.Has(3) || i.Has(5) {
		t.Fatalf("Intersect failed: %v", i)
	}
	d := s.Diff(NewSet(3))
	if d.Has(3) || !d.Has(1) {
		t.Fatalf("Diff failed: %v", d)
	}
}

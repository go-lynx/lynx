package collection

import "testing"

func TestSliceX(t *testing.T) {
	m := Map([]int{1, 2, 3}, func(i int) int { return i * i })
	if len(m) != 3 || m[2] != 9 {
		t.Fatalf("Map failed: %v", m)
	}

	f := Filter([]int{1, 2, 3, 4}, func(i int) bool { return i%2 == 0 })
	if len(f) != 2 || f[0] != 2 || f[1] != 4 {
		t.Fatalf("Filter failed: %v", f)
	}

	u := Unique([]int{1, 1, 2, 2, 3})
	if len(u) != 3 {
		t.Fatalf("Unique failed: %v", u)
	}

	ch := Chunk([]int{1, 2, 3, 4, 5}, 2)
	if len(ch) != 3 || len(ch[2]) != 1 || ch[2][0] != 5 {
		t.Fatalf("Chunk failed: %v", ch)
	}

	g := GroupBy([]string{"a", "aa", "b"}, func(s string) int { return len(s) })
	if len(g[1]) != 2 || len(g[2]) != 1 {
		t.Fatalf("GroupBy failed: %v", g)
	}
}

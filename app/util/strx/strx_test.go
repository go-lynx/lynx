package strx

import "testing"

func TestStrX(t *testing.T) {
	if !HasPrefixAny("https://a", "http://", "https://") {
		t.Fatalf("HasPrefixAny")
	}
	if !HasSuffixAny("a.json", ".json") {
		t.Fatalf("HasSuffixAny")
	}

	if Truncate("中文ABC", 0, "…") != "" {
		t.Fatalf("Truncate max<=0")
	}
	if Truncate("中文ABC", 2, "") != "中文" {
		t.Fatalf("Truncate runes no ellipsis")
	}
	got := Truncate("中文ABC", 3, "…")
	if got == "中文ABC" || len([]rune(got)) != 3 {
		t.Fatalf("Truncate with ellipsis: %q", got)
	}

	if TrimSpaceAndCompress(" a\t b\n c ") != "a b c" {
		t.Fatalf("TrimSpaceAndCompress")
	}
	parts := SplitAndTrim("a, b, ,c", ",")
	if len(parts) != 3 || parts[2] != "c" {
		t.Fatalf("SplitAndTrim: %v", parts)
	}
}

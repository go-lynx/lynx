package timex

import (
	"testing"
	"time"
)

func TestParseAlignJitterWithin(t *testing.T) {
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05"}
	_, err := ParseAny(layouts, "2025-08-19T12:00:00Z")
	if err != nil {
		t.Fatalf("ParseAny RFC3339: %v", err)
	}
	if _, err := ParseAny(layouts, "bad"); err == nil {
		t.Fatalf("ParseAny should fail")
	}

	base := time.Date(2025, 1, 1, 10, 23, 0, 0, time.UTC)
	al := Align(base, 5*time.Minute)
	if al.Minute() != 20 || al.Second() != 0 {
		t.Fatalf("Align failed: %v", al)
	}

	d := Jitter(2*time.Second, 0.2)
	if d < 2*time.Second || d > time.Duration(float64(2*time.Second)*1.2)+0 {
		t.Fatalf("Jitter out of range: %v", d)
	}

	if !Within(base, base.Add(-time.Second), base.Add(time.Second)) {
		t.Fatalf("Within failed")
	}
}

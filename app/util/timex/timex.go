package timex

import (
	"fmt"
	"math/rand"
	"time"
)

// NowUTC returns the current time in UTC.
func NowUTC() time.Time { return time.Now().UTC() }

// ParseAny tries layouts in order to parse the time string.
func ParseAny(layouts []string, s string) (time.Time, error) {
	var lastErr error
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return time.Time{}, lastErr
	}
	return time.Time{}, fmt.Errorf("ParseAny: no layouts provided for input %q", s)
}

// Align floors time t to the nearest lower multiple of d (e.g., 5m, 1h).
func Align(t time.Time, d time.Duration) time.Time {
	if d <= 0 {
		return t
	}
	// Align using nanoseconds since Unix epoch
	unixNano := t.UnixNano()
	aligned := unixNano - (unixNano % int64(d))
	return time.Unix(0, aligned).In(t.Location())
}

// Jitter multiplies duration by a random factor in [1, 1+ratio].
// ratio<0 is treated as 0; ratio>1 is capped at 1.
func Jitter(d time.Duration, ratio float64) time.Duration {
	if d <= 0 || ratio == 0 {
		return d
	}
	if ratio < 0 {
		ratio = 0
	} else if ratio > 1 {
		ratio = 1
	}
	// Generate a random float in [0, ratio]
	f := rand.Float64() * ratio
	return time.Duration(float64(d) * (1 + f))
}

// Within reports whether t is within the closed interval [start, end].
func Within(t, start, end time.Time) bool {
	if end.Before(start) {
		start, end = end, start
	}
	return (t.Equal(start) || t.After(start)) && (t.Equal(end) || t.Before(end))
}

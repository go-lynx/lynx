package timex

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

var jitterSeq atomic.Uint64

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
	f := randFloat64() * ratio
	return time.Duration(float64(d) * (1 + f))
}

// JitterAround returns d multiplied by a factor in [1-ratio, 1+ratio].
func JitterAround(d time.Duration, ratio float64) time.Duration {
	if d <= 0 || ratio == 0 {
		return d
	}
	if ratio < 0 {
		ratio = 0
	} else if ratio > 1 {
		ratio = 1
	}
	factor := 1 - ratio + randFloat64()*2*ratio
	if factor <= 0 {
		return d
	}
	return time.Duration(float64(d) * factor)
}

// ExponentialBackoff returns base*2^attempt capped at max, then applies +/- jitterRatio.
func ExponentialBackoff(base, max time.Duration, attempt int, jitterRatio float64) time.Duration {
	if base <= 0 {
		return 0
	}
	if attempt < 0 {
		attempt = 0
	}
	delay := base
	for i := 0; i < attempt; i++ {
		if max > 0 && delay >= max/2 {
			delay = max
			break
		}
		delay *= 2
	}
	if max > 0 && delay > max {
		delay = max
	}
	delay = JitterAround(delay, jitterRatio)
	if max > 0 && delay > max {
		return max
	}
	return delay
}

// RandomDuration returns a pseudo-random duration in [min, max].
func RandomDuration(min, max time.Duration) time.Duration {
	if max <= min {
		return min
	}
	span := uint64((max - min).Nanoseconds())
	if span == 0 {
		return min
	}
	return min + time.Duration(randUint64()%(span+1))
}

func randFloat64() float64 {
	return float64(randUint64()>>11) * (1.0 / (1 << 53))
}

func randUint64() uint64 {
	x := jitterSeq.Add(0x9e3779b97f4a7c15)
	x += uint64(time.Now().UnixNano()) ^ uint64(os.Getpid())
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// Within reports whether t is within the closed interval [start, end].
func Within(t, start, end time.Time) bool {
	if end.Before(start) {
		start, end = end, start
	}
	return (t.Equal(start) || t.After(start)) && (t.Equal(end) || t.Before(end))
}

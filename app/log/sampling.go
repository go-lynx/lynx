package log

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// sampling and rate limiting variables
var (
	samplingEnabled = false
	infoRatio       = 1.0
	debugRatio      = 1.0
	maxInfoPerSec   = 0
	maxDebugPerSec  = 0

	// internal counters for rate limiting
	rateMu     sync.Mutex
	secWindow  int64
	infoCount  int
	debugCount int
)

// rng is a package-local RNG for sampling. Methods on *rand.Rand are not
// goroutine-safe, so we rely on rateMu to serialize access around uses.
var rng *rand.Rand

// init initializes rng using a cryptographically secure seed; falls back to
// time-based seed if crypto RNG is unavailable.
func init() {
	var seed int64
	var b [8]byte
	if _, err := crand.Read(b[:]); err == nil {
		seed = int64(binary.LittleEndian.Uint64(b[:]))
	} else {
		seed = time.Now().UnixNano()
	}
	rng = rand.New(rand.NewSource(seed))
}

// allowLog applies ratio sampling and per-second rate limiting for debug/info levels.
// It returns true if the log should be emitted.
func allowLog(level log.Level) bool {
	if !samplingEnabled {
		return true
	}

	nowSec := time.Now().Unix()
	rateMu.Lock()
	if secWindow != nowSec {
		secWindow = nowSec
		infoCount = 0
		debugCount = 0
	}
	defer rateMu.Unlock()

	switch level {
	case log.LevelDebug:
		// ratio sampling
		if debugRatio < 1.0 {
			if rng.Float64() > debugRatio {
				return false
			}
		}
		// rate limit
		if maxDebugPerSec > 0 {
			if debugCount >= maxDebugPerSec {
				return false
			}
			debugCount++
		}
	case log.LevelInfo:
		if infoRatio < 1.0 {
			if rng.Float64() > infoRatio {
				return false
			}
		}
		if maxInfoPerSec > 0 {
			if infoCount >= maxInfoPerSec {
				return false
			}
			infoCount++
		}
	}
	return true
}

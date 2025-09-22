package log

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// samplingConfig holds runtime-configurable sampling and rate limit parameters.
type samplingConfig struct {
	enabled        bool
	infoRatio      float64
	debugRatio     float64
	maxInfoPerSec  int
	maxDebugPerSec int
}

// sconf stores the current sampling configuration atomically.
var sconf atomic.Value // *samplingConfig

// internal counters for rate limiting
var (
	rateMu     sync.Mutex
	secWindow  int64
	infoCount  int
	debugCount int
)

// rng is a package-local RNG for sampling. Methods on *rand.Rand are not
// goroutine-safe, so we rely on rateMu to serialize access around uses.
var rng *rand.Rand

// init initializes rng using a cryptographically secure seed; falls back to
// time-based seed if crypto RNG is unavailable. It also sets default sampling config.
func init() {
	var seed int64
	var b [8]byte
	if _, err := crand.Read(b[:]); err == nil {
		seed = int64(binary.LittleEndian.Uint64(b[:]))
	} else {
		seed = time.Now().UnixNano()
	}
	rng = rand.New(rand.NewSource(seed))

	// default: disabled sampling, keep all info/debug, no rate limit
	sconf.Store(&samplingConfig{
		enabled:        false,
		infoRatio:      1.0,
		debugRatio:     1.0,
		maxInfoPerSec:  0,
		maxDebugPerSec: 0,
	})
}

func getSamplingConfig() *samplingConfig {
	if v := sconf.Load(); v != nil {
		if c, ok := v.(*samplingConfig); ok && c != nil {
			return c
		}
	}
	// fallback default
	return &samplingConfig{enabled: false, infoRatio: 1.0, debugRatio: 1.0}
}

// setSamplingConfig updates the sampling configuration atomically.
func setSamplingConfig(enabled bool, infoRatio, debugRatio float64, maxInfoPerSec, maxDebugPerSec int) {
	// clamp ratios to [0,1]
	if infoRatio < 0 {
		infoRatio = 0
	}
	if infoRatio > 1 {
		infoRatio = 1
	}
	if debugRatio < 0 {
		debugRatio = 0
	}
	if debugRatio > 1 {
		debugRatio = 1
	}
	sconf.Store(&samplingConfig{
		enabled:        enabled,
		infoRatio:      infoRatio,
		debugRatio:     debugRatio,
		maxInfoPerSec:  maxInfoPerSec,
		maxDebugPerSec: maxDebugPerSec,
	})
}

// allowLog applies ratio sampling and per-second rate limiting for debug/info levels.
// It returns true if the log should be emitted.
func allowLog(level log.Level) bool {
	cfg := getSamplingConfig()
	if cfg == nil || !cfg.enabled {
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
		if cfg.debugRatio < 1.0 {
			if rng.Float64() > cfg.debugRatio {
				return false
			}
		}
		// rate limit
		if cfg.maxDebugPerSec > 0 {
			if debugCount >= cfg.maxDebugPerSec {
				return false
			}
			debugCount++
		}
	case log.LevelInfo:
		if cfg.infoRatio < 1.0 {
			if rng.Float64() > cfg.infoRatio {
				return false
			}
		}
		if cfg.maxInfoPerSec > 0 {
			if infoCount >= cfg.maxInfoPerSec {
				return false
			}
			infoCount++
		}
	default:
		return true
	}
	return true
}

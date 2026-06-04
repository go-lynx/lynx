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

// Per-second rate-limit counters. Reads use atomics; the mutex guards only the
// once-per-second window reset, keeping the hot path lock-free.
var (
	rateMu          sync.Mutex
	secWindow       atomic.Int64
	infoCount       atomic.Int64
	debugCount      atomic.Int64
	samplingEnabled atomic.Bool // fast-path flag checked before any sampling work
)

// rngPool gives each goroutine its own RNG, avoiding lock contention on the
// shared math/rand source during sampling.
var rngPool = sync.Pool{
	New: func() any {
		var seed int64
		var b [8]byte
		if _, err := crand.Read(b[:]); err == nil {
			seed = int64(binary.LittleEndian.Uint64(b[:]))
		} else {
			seed = time.Now().UnixNano()
		}
		return rand.New(rand.NewSource(seed))
	},
}

func getRNG() *rand.Rand {
	return rngPool.Get().(*rand.Rand)
}

func putRNG(r *rand.Rand) {
	rngPool.Put(r)
}

func init() {
	// Default: sampling off, all info/debug kept, no rate limit.
	defaultConfig := &samplingConfig{
		enabled:        false,
		infoRatio:      1.0,
		debugRatio:     1.0,
		maxInfoPerSec:  0,
		maxDebugPerSec: 0,
	}
	sconf.Store(defaultConfig)
	samplingEnabled.Store(false)
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
	samplingEnabled.Store(enabled)
}

// allowLog reports whether a log at the given level should be emitted, applying
// ratio sampling and per-second rate limiting to debug/info only. warn/error/fatal
// are always emitted. Returns true immediately when sampling is disabled.
func allowLog(level log.Level) bool {
	if !samplingEnabled.Load() {
		return true
	}

	if level != log.LevelDebug && level != log.LevelInfo {
		return true
	}

	cfg := getSamplingConfig()
	if cfg == nil || !cfg.enabled {
		return true
	}

	nowSec := time.Now().Unix()
	currentWindow := secWindow.Load()

	// Hot path: same window, no lock needed (ratio sampling + CAS rate limit).
	if currentWindow == nowSec {
		rng := getRNG()
		defer putRNG(rng)

		switch level {
		case log.LevelDebug:
			if cfg.debugRatio < 1.0 {
				if rng.Float64() > cfg.debugRatio {
					return false
				}
			}
			// CAS-increment the counter; retry on contention rather than lock.
			if cfg.maxDebugPerSec > 0 {
				for {
					current := debugCount.Load()
					if current >= int64(cfg.maxDebugPerSec) {
						return false
					}
					if debugCount.CompareAndSwap(current, current+1) {
						break
					}
				}
			}
		case log.LevelInfo:
			if cfg.infoRatio < 1.0 {
				if rng.Float64() > cfg.infoRatio {
					return false
				}
			}
			if cfg.maxInfoPerSec > 0 {
				for {
					current := infoCount.Load()
					if current >= int64(cfg.maxInfoPerSec) {
						return false
					}
					if infoCount.CompareAndSwap(current, current+1) {
						break
					}
				}
			}
		default:
			return true
		}
		return true
	}

	// New second: reset counters under lock, double-checking in case a
	// concurrent caller already rotated the window.
	rateMu.Lock()
	if secWindow.Load() != nowSec {
		secWindow.Store(nowSec)
		infoCount.Store(0)
		debugCount.Store(0)
	}
	rateMu.Unlock()

	// Re-run the hot-path logic against the reset window (iterative, not recursive).
	currentWindow = secWindow.Load()
	if currentWindow == nowSec {
		rng := getRNG()
		defer putRNG(rng)

		switch level {
		case log.LevelDebug:
			if cfg.debugRatio < 1.0 {
				if rng.Float64() > cfg.debugRatio {
					return false
				}
			}
			if cfg.maxDebugPerSec > 0 {
				for {
					current := debugCount.Load()
					if current >= int64(cfg.maxDebugPerSec) {
						return false
					}
					if debugCount.CompareAndSwap(current, current+1) {
						break
					}
				}
			}
		case log.LevelInfo:
			if cfg.infoRatio < 1.0 {
				if rng.Float64() > cfg.infoRatio {
					return false
				}
			}
			if cfg.maxInfoPerSec > 0 {
				for {
					current := infoCount.Load()
					if current >= int64(cfg.maxInfoPerSec) {
						return false
					}
					if infoCount.CompareAndSwap(current, current+1) {
						break
					}
				}
			}
		default:
			return true
		}
		return true
	}

	// Should not reach here, but return true as fallback
	return true
}

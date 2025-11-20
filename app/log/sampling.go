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
// Use atomic operations for better performance
var (
	rateMu     sync.Mutex // Only used for window reset, not for every check
	secWindow  atomic.Int64
	infoCount  atomic.Int64
	debugCount atomic.Int64
	// Fast path: atomic flag to check if sampling is enabled
	samplingEnabled atomic.Bool
)

// rngPool is a pool of random number generators for lock-free sampling
// Each goroutine can get its own RNG from the pool, avoiding lock contention
var rngPool = sync.Pool{
	New: func() interface{} {
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

// getRNG gets a random number generator from the pool
// Callers should put it back after use for better performance
func getRNG() *rand.Rand {
	return rngPool.Get().(*rand.Rand)
}

// putRNG returns a random number generator to the pool
func putRNG(r *rand.Rand) {
	rngPool.Put(r)
}

// init initializes default sampling configuration
func init() {
	// default: disabled sampling, keep all info/debug, no rate limit
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
	// Update fast path flag
	samplingEnabled.Store(enabled)
}

// allowLog applies ratio sampling and per-second rate limiting for debug/info levels.
// It returns true if the log should be emitted.
// Optimized with fast path: if sampling is disabled, return immediately.
func allowLog(level log.Level) bool {
	// Fast path: if sampling is disabled, return immediately without any overhead
	if !samplingEnabled.Load() {
		return true
	}

	// Fast path: warn/error/fatal are never sampled
	if level != log.LevelDebug && level != log.LevelInfo {
		return true
	}

	cfg := getSamplingConfig()
	if cfg == nil || !cfg.enabled {
		return true
	}

	nowSec := time.Now().Unix()
	currentWindow := secWindow.Load()
	
	// Fast path: if window hasn't changed, use atomic operations (lock-free)
	if currentWindow == nowSec {
		// Get RNG from pool for lock-free random number generation
		rng := getRNG()
		defer putRNG(rng)
		
		switch level {
		case log.LevelDebug:
			// ratio sampling
			if cfg.debugRatio < 1.0 {
				if rng.Float64() > cfg.debugRatio {
					return false
				}
			}
			// rate limit using CAS to prevent race conditions
			if cfg.maxDebugPerSec > 0 {
				for {
					current := debugCount.Load()
					if current >= int64(cfg.maxDebugPerSec) {
						return false
					}
					// Use CompareAndSwap to atomically increment
					if debugCount.CompareAndSwap(current, current+1) {
						// Successfully incremented, allow log
						break
					}
					// CAS failed, retry (another goroutine modified the value)
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
					// Use CompareAndSwap to atomically increment
					if infoCount.CompareAndSwap(current, current+1) {
						// Successfully incremented, allow log
						break
					}
					// CAS failed, retry (another goroutine modified the value)
				}
			}
		default:
			return true
		}
		return true
	}
	
	// Window changed: need to reset counters (requires lock)
	rateMu.Lock()
	// Double-check after acquiring lock (another goroutine might have reset it)
	if secWindow.Load() != nowSec {
		secWindow.Store(nowSec)
		infoCount.Store(0)
		debugCount.Store(0)
	}
	rateMu.Unlock()
	
	// Retry with updated window (use same logic as fast path, not recursive)
	// This avoids potential stack overflow and is more efficient
	currentWindow = secWindow.Load()
	if currentWindow == nowSec {
		// Get RNG from pool
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

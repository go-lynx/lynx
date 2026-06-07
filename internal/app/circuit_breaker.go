package app

import (
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreaker provides error handling and recovery.
//
// Concurrency design
// ──────────────────
// State transitions use atomic CAS on `state`; counters and lastFailure are
// guarded by a plain sync.Mutex (they are always written together).
//
// HalfOpen single-probe guarantee
// ─────────────────────────────────
// The `probing` atomic flag ensures at most one goroutine acts as the probe
// while the circuit is half-open.
//
//   Open → HalfOpen transition (CanExecute):
//     1. CAS probing 0 → 1.  Exactly one goroutine wins; all others return false.
//     2. CAS state Open → HalfOpen (the probe goroutine only).
//
//   Probe success (RecordResult nil):
//     state → Closed, then probing → 0.
//
//   Probe failure (RecordResult err):
//     state → Open, then probing → 0 (allows a fresh probe after the next timeout).
//
// Rule of thumb for future changes:
//   - Read/write `state` ONLY through atomicLoadState / atomicStoreState /
//     atomicCASState.  Never hold `mu` while calling those helpers.
//   - Read/write failureCount, successCount, lastFailure ONLY while holding `mu`.
//   - Read/write `probing` ONLY through atomic operations.
type CircuitBreaker struct {
	// state is accessed exclusively via atomic helpers; see rule above.
	state int32 // stores a CircuitState value

	// probing is 1 while a HalfOpen probe is in flight, 0 otherwise.
	// It is set to 1 by the goroutine that claims the probe slot (in CanExecute)
	// and reset to 0 by RecordResult once the probe completes.
	probing int32

	mu           sync.Mutex
	failureCount int
	successCount int
	lastFailure  time.Time
	threshold    int
	timeout      time.Duration
}

// CircuitState represents the state of the circuit breaker.
type CircuitState int32

const (
	CircuitStateClosed   CircuitState = iota // 0 – normal operation
	CircuitStateOpen                         // 1 – requests rejected
	CircuitStateHalfOpen                     // 2 – one probe allowed
)

// ── atomic helpers ────────────────────────────────────────────────────────────

func (cb *CircuitBreaker) atomicLoadState() CircuitState {
	return CircuitState(atomic.LoadInt32(&cb.state))
}

func (cb *CircuitBreaker) atomicStoreState(s CircuitState) {
	atomic.StoreInt32(&cb.state, int32(s))
}

// atomicCASState transitions from old → new atomically.
// Returns true if the swap happened (this goroutine "won").
func (cb *CircuitBreaker) atomicCASState(old, new CircuitState) bool {
	return atomic.CompareAndSwapInt32(&cb.state, int32(old), int32(new))
}

// ── public API ────────────────────────────────────────────────────────────────

// NewCircuitBreaker creates a new circuit breaker with the given threshold and timeout.
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 1
	}
	if timeout <= 0 {
		timeout = time.Second
	}
	cb := &CircuitBreaker{
		threshold: threshold,
		timeout:   timeout,
	}
	cb.atomicStoreState(CircuitStateClosed)
	return cb
}

// CanExecute checks whether the circuit breaker allows execution.
//
// Concurrency safety (HalfOpen single-probe)
// ─────────────────────────────────────────
// When the Open timeout expires, exactly one goroutine wins the CAS on `probing`
// (0 → 1) and becomes the probe.  All other concurrent callers return false until
// the probe completes and probing is reset to 0.
func (cb *CircuitBreaker) CanExecute() bool {
	switch cb.atomicLoadState() {
	case CircuitStateClosed:
		return true

	case CircuitStateOpen:
		// Read lastFailure under mu to avoid a data race with RecordResult.
		cb.mu.Lock()
		elapsed := time.Since(cb.lastFailure)
		cb.mu.Unlock()

		if elapsed < cb.timeout {
			return false
		}
		// Claim the probe slot before transitioning to HalfOpen.
		// The CAS is atomic so exactly one goroutine wins when multiple callers
		// race on a freshly-expired timeout.
		if !atomic.CompareAndSwapInt32(&cb.probing, 0, 1) {
			return false // another goroutine already owns the probe slot
		}
		cb.atomicCASState(CircuitStateOpen, CircuitStateHalfOpen)
		return true

	case CircuitStateHalfOpen:
		// A probe is already in flight (probing == 1); block all other callers.
		return false

	default:
		return false
	}
}

// RecordResult records the outcome of an operation and transitions state
// accordingly.  It also resets the probe slot when leaving HalfOpen.
func (cb *CircuitBreaker) RecordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailure = time.Now()

		switch cb.atomicLoadState() {
		case CircuitStateClosed:
			if cb.failureCount >= cb.threshold {
				cb.atomicStoreState(CircuitStateOpen)
			}
		case CircuitStateHalfOpen:
			// Probe failed: reopen the circuit, then release the probe slot so a
			// fresh probe can be attempted after the next timeout.
			cb.atomicStoreState(CircuitStateOpen)
			atomic.StoreInt32(&cb.probing, 0)
		}
	} else {
		cb.successCount++

		if cb.atomicLoadState() == CircuitStateHalfOpen {
			// Probe succeeded: close the circuit and release the probe slot.
			cb.atomicStoreState(CircuitStateClosed)
			cb.resetCounters()
			atomic.StoreInt32(&cb.probing, 0)
		}
	}
}

// resetCounters resets failure and success counts.
// MUST be called with cb.mu held.
func (cb *CircuitBreaker) resetCounters() {
	cb.failureCount = 0
	cb.successCount = 0
}

// GetState returns the current circuit breaker state.
func (cb *CircuitBreaker) GetState() CircuitState {
	return cb.atomicLoadState()
}

package lynx

import (
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreaker provides error handling and recovery.
//
// Concurrency design
// ──────────────────
// State transitions are the only write path that previously required a
// read-lock → write-lock "upgrade" – a pattern that is fundamentally
// unsafe in Go's sync.RWMutex (there is no atomic upgrade; the caller
// must release the read-lock first, creating a window where another
// goroutine can observe an inconsistent state or perform the same
// transition).
//
// The fix replaces the mutable `state` field with an atomic int32 and
// uses a compare-and-swap (CAS) to transition Open → HalfOpen.  Only
// one goroutine wins the CAS; all others see HalfOpen and return true
// without touching the state again.  The remaining counters and
// lastFailure are still guarded by a plain sync.Mutex because they are
// always written together.
//
// Rule of thumb for future changes:
//   - Read/write `state` ONLY through atomicLoadState / atomicStoreState /
//     atomicCASState.  Never hold `mu` while calling those helpers.
//   - Read/write failureCount, successCount, lastFailure ONLY while
//     holding `mu`.
type CircuitBreaker struct {
	// state is accessed exclusively via atomic helpers; see rule above.
	state int32 // stores a CircuitState value

	mu           sync.Mutex
	failureCount int
	successCount int
	lastFailure  time.Time
	threshold    int
	timeout      time.Duration
}

// CircuitState represents the state of circuit breaker.
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

// NewCircuitBreaker creates a new circuit breaker with the provided threshold and timeout.
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

// CanExecute checks if the circuit breaker allows execution.
//
// Concurrency safety
// ──────────────────
// The Open → HalfOpen transition is a single CAS.  Exactly one goroutine
// wins; all others observe HalfOpen and return true without a second
// state write.  There is no lock-upgrade race and no defer/manual-unlock
// mismatch.
func (cb *CircuitBreaker) CanExecute() bool {
	switch cb.atomicLoadState() {
	case CircuitStateClosed:
		return true

	case CircuitStateOpen:
		// Read lastFailure under mu to avoid a data race: RecordResult
		// writes it under the same lock.
		cb.mu.Lock()
		elapsed := time.Since(cb.lastFailure)
		cb.mu.Unlock()

		if elapsed >= cb.timeout {
			// Only one goroutine transitions Open → HalfOpen.
			// If the CAS fails, another goroutine already made the
			// transition (or RecordResult moved state elsewhere); in
			// either case the state is no longer Open, so allow execution.
			cb.atomicCASState(CircuitStateOpen, CircuitStateHalfOpen)
			return true
		}
		return false

	case CircuitStateHalfOpen:
		return true

	default:
		return false
	}
}

// RecordResult records the result of an operation.
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
			cb.atomicStoreState(CircuitStateOpen)
		}
	} else {
		cb.successCount++

		if cb.atomicLoadState() == CircuitStateHalfOpen {
			cb.atomicStoreState(CircuitStateClosed)
			cb.resetCounters()
		}
	}
}

// resetCounters resets the failure and success counts.
// MUST be called with cb.mu held.
func (cb *CircuitBreaker) resetCounters() {
	cb.failureCount = 0
	cb.successCount = 0
}

// GetState returns the current circuit breaker state.
func (cb *CircuitBreaker) GetState() CircuitState {
	return cb.atomicLoadState()
}

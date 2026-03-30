package lynx

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewCircuitBreaker_ClampsInvalidConfig(t *testing.T) {
	cb := NewCircuitBreaker(0, 0)
	if cb == nil {
		t.Fatal("expected circuit breaker")
	}
	if cb.threshold != 1 {
		t.Fatalf("expected threshold clamp to 1, got %d", cb.threshold)
	}
	if cb.timeout != time.Second {
		t.Fatalf("expected timeout clamp to 1s, got %v", cb.timeout)
	}
	if cb.GetState() != CircuitStateClosed {
		t.Fatalf("expected initial state closed, got %v", cb.GetState())
	}
}

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	cb := NewCircuitBreaker(2, 10*time.Millisecond)

	if !cb.CanExecute() {
		t.Fatal("expected closed breaker to allow execution")
	}

	cb.RecordResult(fmt.Errorf("failure-1"))
	if cb.GetState() != CircuitStateClosed {
		t.Fatalf("expected breaker to remain closed after first failure, got %v", cb.GetState())
	}

	cb.RecordResult(fmt.Errorf("failure-2"))
	if cb.GetState() != CircuitStateOpen {
		t.Fatalf("expected breaker to open after threshold failures, got %v", cb.GetState())
	}
	if cb.CanExecute() {
		t.Fatal("expected open breaker to reject execution before timeout")
	}

	time.Sleep(15 * time.Millisecond)
	if !cb.CanExecute() {
		t.Fatal("expected open breaker to transition to half-open after timeout")
	}
	if cb.GetState() != CircuitStateHalfOpen {
		t.Fatalf("expected half-open state after timeout, got %v", cb.GetState())
	}

	cb.RecordResult(nil)
	if cb.GetState() != CircuitStateClosed {
		t.Fatalf("expected successful half-open attempt to close breaker, got %v", cb.GetState())
	}
}

// TestCircuitBreaker_ConcurrentCanExecuteAfterTimeout verifies that when many
// goroutines call CanExecute simultaneously after the open timeout expires,
// the breaker transitions to HalfOpen exactly once (via CAS) and no panic or
// double-unlock occurs.  Run with -race to catch data races.
func TestCircuitBreaker_ConcurrentCanExecuteAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(1, 5*time.Millisecond)
	cb.RecordResult(fmt.Errorf("boom"))
	if cb.GetState() != CircuitStateOpen {
		t.Fatalf("expected breaker to open, got %v", cb.GetState())
	}

	time.Sleep(10 * time.Millisecond)

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = cb.CanExecute()
		}()
	}
	wg.Wait()

	// After the timeout all goroutines should see HalfOpen (or have raced
	// through it and been closed by RecordResult, but since we did not
	// call RecordResult here it must still be HalfOpen).
	if cb.GetState() != CircuitStateHalfOpen {
		t.Fatalf("expected breaker to stabilize in half-open, got %v", cb.GetState())
	}
}

// TestCircuitBreaker_NoPanicOnConcurrentTransition is a targeted regression
// test for the original double-unlock bug.  It checks that no panic occurs
// when hundreds of goroutines hammer CanExecute just as the timeout fires.
func TestCircuitBreaker_NoPanicOnConcurrentTransition(t *testing.T) {
	const rounds = 20
	for r := 0; r < rounds; r++ {
		cb := NewCircuitBreaker(1, time.Millisecond)
		cb.RecordResult(errors.New("err"))

		// Spin up goroutines before the timeout fires so some race the transition.
		const n = 64
		var wg sync.WaitGroup
		wg.Add(n)
		// Small sleep so the timeout is imminent but not necessarily expired.
		time.Sleep(500 * time.Microsecond)
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				_ = cb.CanExecute()
			}()
		}
		wg.Wait()
		// State must be Open or HalfOpen; Closed is impossible without a success.
		st := cb.GetState()
		if st != CircuitStateOpen && st != CircuitStateHalfOpen {
			t.Fatalf("round %d: unexpected state %v", r, st)
		}
	}
}

func TestCircuitBreaker_HalfOpen_FailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(1, 5*time.Millisecond)

	// Open the breaker
	cb.RecordResult(fmt.Errorf("initial failure"))
	if cb.GetState() != CircuitStateOpen {
		t.Fatalf("expected open state, got %v", cb.GetState())
	}

	// Wait for timeout to transition to half-open
	time.Sleep(10 * time.Millisecond)
	if !cb.CanExecute() {
		t.Fatal("expected half-open to allow execution")
	}

	// Record another failure in half-open state — should reopen
	cb.RecordResult(fmt.Errorf("failure in half-open"))
	if cb.GetState() != CircuitStateOpen {
		t.Fatalf("expected breaker to reopen on half-open failure, got %v", cb.GetState())
	}
}

func TestCircuitBreaker_SuccessDoesNotOpenClosedBreaker(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)
	for i := 0; i < 10; i++ {
		cb.RecordResult(nil)
	}
	if cb.GetState() != CircuitStateClosed {
		t.Fatalf("expected breaker to remain closed on repeated successes, got %v", cb.GetState())
	}
	if !cb.CanExecute() {
		t.Fatal("expected closed breaker to allow execution")
	}
}

func TestCircuitBreaker_ResetCountersOnClose(t *testing.T) {
	cb := NewCircuitBreaker(2, 5*time.Millisecond)

	// Accumulate one failure (below threshold, stays closed)
	cb.RecordResult(fmt.Errorf("partial failure"))
	if cb.failureCount != 1 {
		t.Fatalf("expected failure count 1, got %d", cb.failureCount)
	}

	// Open breaker
	cb.RecordResult(fmt.Errorf("threshold failure"))
	if cb.GetState() != CircuitStateOpen {
		t.Fatalf("expected open state, got %v", cb.GetState())
	}

	// Transition to half-open, then close with success
	time.Sleep(10 * time.Millisecond)
	_ = cb.CanExecute()
	cb.RecordResult(nil)

	if cb.GetState() != CircuitStateClosed {
		t.Fatalf("expected closed state after recovery, got %v", cb.GetState())
	}
	if cb.failureCount != 0 {
		t.Fatalf("expected failure count reset to 0 after close, got %d", cb.failureCount)
	}
	if cb.successCount != 0 {
		t.Fatalf("expected success count reset to 0 after close, got %d", cb.successCount)
	}
}

// TestCircuitBreaker_GetState_DataRace runs with -race to verify no data race
// when RecordResult and GetState are called concurrently.
func TestCircuitBreaker_GetState_DataRace(t *testing.T) {
	cb := NewCircuitBreaker(5, 5*time.Millisecond)
	var wg sync.WaitGroup
	const n = 50

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				cb.RecordResult(fmt.Errorf("err"))
			} else {
				_ = cb.GetState()
			}
		}(i)
	}
	wg.Wait()
	// No assertions needed — test verifies no data race with -race flag.
}

// TestCircuitBreaker_FullConcurrency hammers all three public methods
// concurrently to exercise every concurrent path.  Run with -race.
func TestCircuitBreaker_FullConcurrency(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Millisecond)
	var wg sync.WaitGroup
	const n = 100

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			switch idx % 3 {
			case 0:
				_ = cb.CanExecute()
			case 1:
				cb.RecordResult(fmt.Errorf("concurrent-err"))
			case 2:
				cb.RecordResult(nil)
			}
		}(i)
	}
	wg.Wait()
}

// TestCircuitBreaker_OpenRejectsBeforeTimeout asserts that an open breaker
// rejects calls before the timeout elapses (no accidental early transition).
func TestCircuitBreaker_OpenRejectsBeforeTimeout(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Second) // long timeout
	cb.RecordResult(errors.New("err"))
	if cb.GetState() != CircuitStateOpen {
		t.Fatalf("expected open state, got %v", cb.GetState())
	}
	for i := 0; i < 5; i++ {
		if cb.CanExecute() {
			t.Fatal("open breaker with long timeout must reject execution")
		}
	}
}

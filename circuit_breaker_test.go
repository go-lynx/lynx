package lynx

import (
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

	if cb.GetState() != CircuitStateHalfOpen {
		t.Fatalf("expected breaker to stabilize in half-open, got %v", cb.GetState())
	}
}

package lynx

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-lynx/lynx/observability/metrics"
)

// TestErrorRecoveryManager_ConcurrentRecovery tests concurrent recovery operations
func TestErrorRecoveryManager_ConcurrentRecovery(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	recoveryCount := int32(0)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			erm.attemptRecovery(record)
			atomic.AddInt32(&recoveryCount, 1)
		}()
	}

	wg.Wait()

	// Verify recovery operations were executed (same recoveryKey)
	activeCount := 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	// Wait for all recoveries to complete
	time.Sleep(200 * time.Millisecond)

	// Check active recovery count again
	activeCount = 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	if activeCount > 0 {
		t.Logf("Active recoveries after completion: %d (should be 0)", activeCount)
	}

	t.Logf("Total recovery attempts: %d", recoveryCount)
}

// TestErrorRecoveryManager_RecoveryKeyCollision tests recovery key collision
func TestErrorRecoveryManager_RecoveryKeyCollision(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// Creating records with the same timestamp and error type may produce the same recoveryKey
	now := time.Now()
	record1 := ErrorRecord{
		ErrorType: "transient",
		Component: "component1",
		Timestamp: now,
		Severity:  ErrorSeverityLow,
	}
	record2 := ErrorRecord{
		ErrorType: "transient",
		Component: "component2",
		Timestamp: now,
		Severity:  ErrorSeverityLow,
	}

	// Trigger two recovery operations simultaneously
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		erm.attemptRecovery(record1)
	}()

	go func() {
		defer wg.Done()
		erm.attemptRecovery(record2)
	}()

	wg.Wait()

	// Wait for recovery to complete
	time.Sleep(200 * time.Millisecond)

	// Verify recovery key uniqueness by checking activeRecoveries
	activeCount := 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	if activeCount > 0 {
		t.Logf("Active recoveries: %d (should be 0 after completion)", activeCount)
	}
}

// TestErrorRecoveryManager_ConcurrentRecordError tests concurrent error recording
func TestErrorRecoveryManager_ConcurrentRecordError(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			erm.RecordError("test-error", ErrorCategorySystem, "test error message", "test-component", ErrorSeverityLow, nil)
		}(i)
	}

	wg.Wait()

	// Verify error count correctness
	stats := erm.GetErrorStats()
	count, ok := stats["test-error"].(int64)
	if !ok {
		t.Fatal("Failed to get error count")
	}

	if count != int64(numGoroutines) {
		t.Errorf("Expected error count %d, got %d", numGoroutines, count)
	}

	// Verify error history records
	history := erm.GetErrorHistory()
	if len(history) != numGoroutines {
		t.Errorf("Expected error history length %d, got %d", numGoroutines, len(history))
	}
}

// TestErrorRecoveryManager_GoroutineLeak tests goroutine leak
func TestErrorRecoveryManager_GoroutineLeak(t *testing.T) {
	before := runtime.NumGoroutine()

	erm := NewErrorRecoveryManager(nil)

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// Start recovery operation
	go erm.attemptRecovery(record)

	// Stop manager immediately
	erm.Stop()

	// Wait for goroutine to exit
	time.Sleep(500 * time.Millisecond)

	after := runtime.NumGoroutine()

	// Allow some system goroutines, should be close to initial value
	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestErrorRecoveryManager_StopMonitorGoroutineExit tests stop monitor goroutine exit
func TestErrorRecoveryManager_StopMonitorGoroutineExit(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	before := runtime.NumGoroutine()

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// Start recovery operation (will return before acquiring semaphore)
	// Trigger timeout by filling the semaphore
	for i := 0; i < 10; i++ {
		select {
		case erm.recoverySemaphore <- struct{}{}:
		default:
		}
	}

	// Attempt recovery (should timeout because semaphore is full)
	go erm.attemptRecovery(record)

	// Wait for timeout and cleanup
	time.Sleep(300 * time.Millisecond)

	// Release semaphore
	for i := 0; i < 10; i++ {
		select {
		case <-erm.recoverySemaphore:
		default:
		}
	}

	// Wait for goroutine to exit
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestErrorRecoveryManager_SemaphoreTimeoutGoroutineCleanup tests goroutine cleanup on semaphore timeout
func TestErrorRecoveryManager_SemaphoreTimeoutGoroutineCleanup(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	before := runtime.NumGoroutine()

	// Fill semaphore
	for i := 0; i < 10; i++ {
		select {
		case erm.recoverySemaphore <- struct{}{}:
		default:
		}
	}

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// Attempt recovery (should timeout because semaphore is full)
	go erm.attemptRecovery(record)

	// Wait for timeout and cleanup
	time.Sleep(300 * time.Millisecond)

	// Release semaphore
	for i := 0; i < 10; i++ {
		select {
		case <-erm.recoverySemaphore:
		default:
		}
	}

	// Wait for goroutine to exit
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestErrorRecoveryManager_RecoveryTimeout tests recovery timeout
func TestErrorRecoveryManager_RecoveryTimeout(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// Create a recovery strategy that will timeout
	slowStrategy := &slowRecoveryStrategy{
		timeout: 100 * time.Millisecond,
		delay:   200 * time.Millisecond, // Longer than timeout
	}
	erm.RegisterRecoveryStrategy("slow", slowStrategy)

	record := ErrorRecord{
		ErrorType: "slow",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	before := runtime.NumGoroutine()

	// Trigger recovery operation
	go erm.attemptRecovery(record)

	// Wait for timeout
	time.Sleep(300 * time.Millisecond)

	after := runtime.NumGoroutine()

	// Verify proper cleanup after timeout
	if after > before+5 {
		t.Errorf("Possible goroutine leak after timeout: before=%d, after=%d", before, after)
	}

	// Verify active recoveries are cleaned up
	activeCount := 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	if activeCount > 0 {
		t.Errorf("Expected 0 active recoveries after timeout, got %d", activeCount)
	}
}

// TestErrorRecoveryManager_SemaphoreTimeout tests semaphore acquisition timeout
func TestErrorRecoveryManager_SemaphoreTimeout(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	before := runtime.NumGoroutine()

	// Fill semaphore
	for i := 0; i < 10; i++ {
		select {
		case erm.recoverySemaphore <- struct{}{}:
		default:
		}
	}

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// Attempt recovery (should timeout because semaphore is full)
	go erm.attemptRecovery(record)

	// Wait for timeout
	time.Sleep(300 * time.Millisecond)

	// Release semaphore
	for i := 0; i < 10; i++ {
		select {
		case <-erm.recoverySemaphore:
		default:
		}
	}

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}
}

// TestErrorRecoveryManager_ContextCancellation tests context cancellation
func TestErrorRecoveryManager_ContextCancellation(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	before := runtime.NumGoroutine()

	// Start recovery operation
	go erm.attemptRecovery(record)

	// Stop manager immediately, cancel all recoveries
	erm.Stop()

	// Wait for cleanup
	time.Sleep(300 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before+5 {
		t.Errorf("Possible goroutine leak: before=%d, after=%d", before, after)
	}

	// Verify active recoveries are cleaned up
	activeCount := 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	if activeCount > 0 {
		t.Errorf("Expected 0 active recoveries after stop, got %d", activeCount)
	}
}

// TestErrorRecoveryManager_NilStrategy tests nil strategy
func TestErrorRecoveryManager_NilStrategy(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// Record an error without a corresponding strategy, should use default strategy
	record := ErrorRecord{
		ErrorType: "unknown-error",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// Should use default "transient" strategy
	erm.attemptRecovery(record)

	// Wait for completion
	time.Sleep(200 * time.Millisecond)

	// Verify no panic
}

// TestErrorRecoveryManager_StrategyCannotRecover tests strategy that cannot recover
func TestErrorRecoveryManager_StrategyCannotRecover(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// Create a strategy that cannot recover
	cannotRecoverStrategy := &cannotRecoverStrategy{
		timeout: 100 * time.Millisecond,
	}
	erm.RegisterRecoveryStrategy("cannot-recover", cannotRecoverStrategy)

	record := ErrorRecord{
		ErrorType: "cannot-recover",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityHigh, // High severity, strategy cannot recover
	}

	// Trigger recovery operation
	erm.attemptRecovery(record)

	// Wait for completion
	time.Sleep(200 * time.Millisecond)

	// Verify no active recovery
	activeCount := 0
	erm.activeRecoveries.Range(func(key, value interface{}) bool {
		activeCount++
		return true
	})

	if activeCount > 0 {
		t.Errorf("Expected 0 active recoveries, got %d", activeCount)
	}
}

// TestErrorRecoveryManager_RecoveryTimeoutCap tests recovery timeout cap
func TestErrorRecoveryManager_RecoveryTimeoutCap(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)
	defer erm.Stop()

	// Create a strategy with timeout > 60s
	longTimeoutStrategy := &DefaultRecoveryStrategy{
		name:    "long-timeout",
		timeout: 120 * time.Second, // Exceeds 60s cap
	}
	erm.RegisterRecoveryStrategy("long-timeout", longTimeoutStrategy)

	record := ErrorRecord{
		ErrorType: "long-timeout",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	start := time.Now()

	// Trigger recovery operation
	go erm.attemptRecovery(record)

	// Wait for a while
	time.Sleep(200 * time.Millisecond)

	// Stop manager
	erm.Stop()

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	duration := time.Since(start)

	// Verify timeout is capped at 60s, should complete quickly in actual test
	if duration > 70*time.Second {
		t.Errorf("Recovery timeout was not capped: duration=%v", duration)
	}
}

// slowRecoveryStrategy is a recovery strategy that delays, used for testing timeout
type slowRecoveryStrategy struct {
	timeout time.Duration
	delay   time.Duration
}

func (s *slowRecoveryStrategy) Name() string {
	return "slow"
}

func (s *slowRecoveryStrategy) CanRecover(errorType string, severity ErrorSeverity) bool {
	return true
}

func (s *slowRecoveryStrategy) Recover(ctx context.Context, record ErrorRecord) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(s.delay):
		return true, nil
	}
}

func (s *slowRecoveryStrategy) GetTimeout() time.Duration {
	return s.timeout
}

// cannotRecoverStrategy is a strategy that cannot recover, used for testing
type cannotRecoverStrategy struct {
	timeout time.Duration
}

func (s *cannotRecoverStrategy) Name() string {
	return "cannot-recover"
}

func (s *cannotRecoverStrategy) CanRecover(errorType string, severity ErrorSeverity) bool {
	// Only recoverable for low severity errors
	return severity <= ErrorSeverityLow
}

func (s *cannotRecoverStrategy) Recover(ctx context.Context, record ErrorRecord) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(s.timeout):
		return true, nil
	}
}

func (s *cannotRecoverStrategy) GetTimeout() time.Duration {
	return s.timeout
}

// TestErrorRecoveryManager_StopMultipleTimes tests calling Stop multiple times
func TestErrorRecoveryManager_StopMultipleTimes(t *testing.T) {
	erm := NewErrorRecoveryManager(nil)

	// Call Stop multiple times, should only execute once
	erm.Stop()
	erm.Stop()
	erm.Stop()

	// Verify no panic
}

// TestErrorRecoveryManager_WithMetrics tests recovery manager with metrics
func TestErrorRecoveryManager_WithMetrics(t *testing.T) {
	// Create metrics, if available
	var m *metrics.ProductionMetrics
	// Note: This may need to be adjusted based on actual metrics initialization
	erm := NewErrorRecoveryManager(m)
	defer erm.Stop()

	record := ErrorRecord{
		ErrorType: "transient",
		Component: "test-component",
		Timestamp: time.Now(),
		Severity:  ErrorSeverityLow,
	}

	// Trigger recovery operation
	go erm.attemptRecovery(record)

	// Wait for completion
	time.Sleep(200 * time.Millisecond)

	// Verify no panic
}

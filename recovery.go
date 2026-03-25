// Package lynx provides the core application framework for building microservices.
//
// This file (recovery.go) contains error recovery and resilience mechanisms:
//   - CircuitBreaker: Prevents cascading failures
//   - ErrorRecoveryManager: Manages error detection and recovery
//   - Health monitoring and automatic recovery strategies
//   - Panic recovery with detailed diagnostics
package lynx

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/observability/metrics"
)

// ErrorRecoveryManager provides centralized error handling and recovery
type ErrorRecoveryManager struct {
	// Error tracking
	errorCounts     map[string]int64
	errorHistory    []ErrorRecord
	recoveryHistory []RecoveryRecord

	// Circuit breakers for different error types
	circuitBreakers map[string]*CircuitBreaker

	// Recovery strategies
	recoveryStrategies map[string]RecoveryStrategy

	// Configuration
	maxErrorHistory    int
	maxRecoveryHistory int
	errorThreshold     int64
	recoveryTimeout    time.Duration

	// Metrics
	metrics *metrics.ProductionMetrics

	// Internal state
	mu       sync.RWMutex
	stopChan chan struct{}
	stopOnce sync.Once // Protect against multiple Stop() calls

	// Concurrency control for recovery operations
	recoverySemaphore       chan struct{}
	maxConcurrentRecoveries int
	activeRecoveries        sync.Map // map[string]*recoveryState or context.CancelFunc - track active recoveries
}

// recoveryState tracks the state of an active recovery operation
// Fixed: Moved to package level to support proper type checking in Stop() method
type recoveryState struct {
	cancel  context.CancelFunc
	started bool
}

// Recovery semaphore acquisition timeout (configurable via environment or config)
const (
	defaultRecoverySemaphoreTimeout = 2 * time.Second // Increased from 1s for better reliability
)

// NewErrorRecoveryManager creates a new error recovery manager
func NewErrorRecoveryManager(metrics *metrics.ProductionMetrics) *ErrorRecoveryManager {
	maxConcurrent := 10 // Default max concurrent recoveries
	erm := &ErrorRecoveryManager{
		errorCounts:             make(map[string]int64),
		errorHistory:            make([]ErrorRecord, 0),
		recoveryHistory:         make([]RecoveryRecord, 0),
		circuitBreakers:         make(map[string]*CircuitBreaker),
		recoveryStrategies:      make(map[string]RecoveryStrategy),
		maxErrorHistory:         1000,
		maxRecoveryHistory:      500,
		errorThreshold:          10,
		recoveryTimeout:         30 * time.Second,
		metrics:                 metrics,
		stopChan:                make(chan struct{}),
		maxConcurrentRecoveries: maxConcurrent,
		recoverySemaphore:       make(chan struct{}, maxConcurrent),
	}

	// Register default recovery strategies
	erm.registerDefaultStrategies()

	return erm
}

// registerDefaultStrategies registers default recovery strategies
func (erm *ErrorRecoveryManager) registerDefaultStrategies() {
	// Retry strategy for transient errors
	retryStrategy := NewDefaultRecoveryStrategy("retry", 5*time.Second)
	erm.RegisterRecoveryStrategy("transient", retryStrategy)

	// Restart strategy for component errors
	restartStrategy := NewDefaultRecoveryStrategy("restart", 10*time.Second)
	erm.RegisterRecoveryStrategy("component", restartStrategy)

	// Fallback strategy for critical errors
	fallbackStrategy := NewDefaultRecoveryStrategy("fallback", 15*time.Second)
	erm.RegisterRecoveryStrategy("critical", fallbackStrategy)
}

// RegisterRecoveryStrategy registers a recovery strategy
func (erm *ErrorRecoveryManager) RegisterRecoveryStrategy(errorType string, strategy RecoveryStrategy) {
	erm.mu.Lock()
	defer erm.mu.Unlock()

	erm.recoveryStrategies[errorType] = strategy

	// Create circuit breaker for this error type
	erm.circuitBreakers[errorType] = NewCircuitBreaker(5, 60*time.Second)
}

// RecordError records an error with enhanced context and classification
func (erm *ErrorRecoveryManager) RecordError(errorType string, category ErrorCategory, message, component string, severity ErrorSeverity, context map[string]interface{}) {
	erm.mu.Lock()
	defer erm.mu.Unlock()

	// Enrich context information
	if context == nil {
		context = make(map[string]interface{})
	}

	// Add system information
	context["timestamp"] = time.Now().Unix()
	context["goroutines"] = runtime.NumGoroutine()
	context["memory_alloc"] = getMemoryStats()

	// Add environment information
	// Optimized: Cache environment variables to avoid repeated os.Getenv calls
	envOnce.Do(func() {
		cachedEnv = os.Getenv("ENV")
		cachedVersion = os.Getenv("APP_VERSION")
	})
	if cachedEnv != "" {
		context["environment"] = cachedEnv
	}
	if cachedVersion != "" {
		context["version"] = cachedVersion
	}

	// Safely extract environment and version with existence checks
	var environment, version string
	if envVal, ok := context["environment"]; ok {
		if envStr, ok := envVal.(string); ok {
			environment = envStr
		}
	}
	if verVal, ok := context["version"]; ok {
		if verStr, ok := verVal.(string); ok {
			version = verStr
		}
	}

	// Create error record with enhanced information
	record := ErrorRecord{
		Timestamp:   time.Now(),
		ErrorType:   errorType,
		Category:    category,
		Message:     message,
		Component:   component,
		Severity:    severity,
		Context:     context,
		Recovered:   false,
		StackTrace:  getStackTrace(),
		Environment: environment,
		Version:     version,
	}

	// Add to history
	// Optimized: Use copy instead of slice operation to prevent memory leak
	// Slice operation [1:] keeps the underlying array, causing memory leak
	if len(erm.errorHistory) >= erm.maxErrorHistory {
		// Use copy to move elements left, then overwrite last element
		// This prevents keeping reference to old underlying array
		copy(erm.errorHistory, erm.errorHistory[1:])
		erm.errorHistory[len(erm.errorHistory)-1] = record
	} else {
		erm.errorHistory = append(erm.errorHistory, record)
	}

	// Update error count
	erm.errorCounts[errorType]++

	// Record metrics with category
	if erm.metrics != nil {
		erm.metrics.RecordPluginError(component, errorType, message)
	}

	// Log error with enhanced context
	log.Errorf("Error recorded: type=%s, category=%s, component=%s, severity=%d, message=%s, context=%+v",
		errorType, category, component, severity, message, context)

	// Check if circuit breaker is open
	circuitBreaker := erm.circuitBreakers[errorType]
	if circuitBreaker != nil && !circuitBreaker.CanExecute() {
		log.Warnf("Circuit breaker is open for error type: %s", errorType)
		return
	}

	// Attempt recovery if severity allows
	if severity <= ErrorSeverityHigh {
		go erm.attemptRecovery(record)
	}
}

// attemptRecovery attempts to recover from an error
// Fixed: Simplified concurrent logic using context for goroutine lifecycle management
func (erm *ErrorRecoveryManager) attemptRecovery(record ErrorRecord) {
	// Generate recovery key first
	recoveryKey := fmt.Sprintf("%s:%s:%d", record.ErrorType, record.Component, record.Timestamp.Unix())

	// Fixed: sync.Map is thread-safe, no need for mutex protection
	// Check if recovery is already in progress (sync.Map is thread-safe)
	if _, exists := erm.activeRecoveries.Load(recoveryKey); exists {
		log.Debugf("Recovery already in progress for %s:%s, skipping duplicate attempt", record.ErrorType, record.Component)
		return
	}

	// Get strategy (use RLock for map access, not for sync.Map)
	erm.mu.RLock()
	strategy, exists := erm.recoveryStrategies[record.ErrorType]
	if !exists {
		strategy = erm.recoveryStrategies["transient"]
	}
	erm.mu.RUnlock()

	if strategy == nil {
		log.Warnf("No recovery strategy found for error type: %s", record.ErrorType)
		return
	}

	// Check if strategy can recover
	if !strategy.CanRecover(record.ErrorType, record.Severity) {
		log.Warnf("Recovery strategy %s cannot recover from error type: %s", strategy.Name(), record.ErrorType)
		return
	}

	// Create recovery context with timeout and cancellation
	// Fixed: Use parent context that respects stopChan to prevent goroutine leaks
	recoveryTimeout := strategy.GetTimeout()
	if recoveryTimeout <= 0 {
		recoveryTimeout = erm.recoveryTimeout
	}
	if recoveryTimeout > 60*time.Second {
		recoveryTimeout = 60 * time.Second // Cap at 60 seconds
	}

	// Create parent context that monitors stopChan with proper cleanup
	// Use context.WithCancel with a timeout to ensure goroutines don't leak
	parentCtx, parentCancel := context.WithCancel(context.Background())

	// Create timeout context from parent
	ctx, timeoutCancel := context.WithTimeout(parentCtx, recoveryTimeout)

	// Monitor stopChan in background with proper context cancellation
	// Use a single goroutine that monitors both stopChan and context cancellation
	stopMonitorDone := make(chan struct{}, 1)
	go func() {
		defer func() {
			// Always signal completion to prevent goroutine leak
			select {
			case stopMonitorDone <- struct{}{}:
			default:
			}
		}()
		select {
		case <-erm.stopChan:
			// Recovery manager is stopping, cancel context
			parentCancel()
		case <-ctx.Done():
			// Context cancelled (timeout or parent cancelled), exit cleanly
		}
	}()

	// Ensure cleanup happens
	defer func() {
		// Cancel parent context first to signal stop monitor
		parentCancel()
		timeoutCancel()
		// Wait for stop monitor goroutine to exit (with timeout)
		select {
		case <-stopMonitorDone:
			// Goroutine exited cleanly
		case <-time.After(50 * time.Millisecond):
			// Timeout - goroutine should exit on its own when context is cancelled
		}
	}()

	// Acquire semaphore to limit concurrent recoveries
	// Use configurable timeout instead of hardcoded value
	semaphoreTimeout := defaultRecoverySemaphoreTimeout
	select {
	case erm.recoverySemaphore <- struct{}{}:
		// Acquired semaphore, proceed with recovery
		defer func() { <-erm.recoverySemaphore }()
	case <-time.After(semaphoreTimeout):
		// Semaphore acquisition timeout - too many concurrent recoveries
		log.Warnf("Recovery semaphore timeout for %s:%s after %v, skipping recovery (too many concurrent recoveries)",
			record.ErrorType, record.Component, semaphoreTimeout)
		// Cancel contexts to ensure goroutines exit (defer will handle cleanup)
		return
	case <-ctx.Done():
		// Context cancelled before semaphore acquisition
		// Defer will handle cleanup
		return
	}

	// Store recovery state atomically (simplified - no complex CAS loop)
	// Fixed: Use parentCancel instead of timeoutCancel to ensure proper cleanup
	state := &recoveryState{
		cancel:  parentCancel, // Use parent cancel to ensure stop monitor goroutine exits
		started: true,
	}
	if _, loaded := erm.activeRecoveries.LoadOrStore(recoveryKey, state); loaded {
		// Another goroutine started recovery, cleanup and return
		// Release semaphore since we're not proceeding
		<-erm.recoverySemaphore
		// Defer will handle context cleanup
		return
	}

	// Set up proper cleanup at the end of recovery
	defer func() {
		erm.activeRecoveries.Delete(recoveryKey)
		// Context cleanup is handled by outer defer
	}()

	// Attempt recovery with timeout protection
	// Simplified: Single goroutine with proper context management
	startTime := time.Now()
	recoveryDone := make(chan struct {
		success bool
		err     error
	}, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				select {
				case recoveryDone <- struct {
					success bool
					err     error
				}{false, fmt.Errorf("panic during recovery: %v", r)}:
				case <-ctx.Done():
					// Context cancelled, ignore result
				}
			}
		}()
		success, err := strategy.Recover(ctx, record)
		select {
		case recoveryDone <- struct {
			success bool
			err     error
		}{success, err}:
		case <-ctx.Done():
			// Context cancelled, ignore result
		}
	}()

	var success bool
	var err error
	select {
	case result := <-recoveryDone:
		success = result.success
		err = result.err
	case <-ctx.Done():
		// Timeout or cancellation reached
		success = false
		err = fmt.Errorf("recovery cancelled or timeout after %v: %w", recoveryTimeout, ctx.Err())
		log.Warnf("Recovery cancelled or timeout for %s:%s after %v", record.ErrorType, record.Component, recoveryTimeout)
	}

	duration := time.Since(startTime)

	// Record recovery attempt
	recoveryRecord := RecoveryRecord{
		Timestamp: time.Now(),
		ErrorType: record.ErrorType,
		Component: record.Component,
		Strategy:  strategy.Name(),
		Success:   success,
		Duration:  duration,
		Message:   "",
	}

	if err != nil {
		recoveryRecord.Message = err.Error()
	}

	erm.mu.Lock()
	// Optimized: Use copy instead of slice operation to prevent memory leak
	// Slice operation [1:] keeps the underlying array, causing memory leak
	if len(erm.recoveryHistory) >= erm.maxRecoveryHistory {
		// Use copy to move elements left, then overwrite last element
		// This prevents keeping reference to old underlying array
		copy(erm.recoveryHistory, erm.recoveryHistory[1:])
		erm.recoveryHistory[len(erm.recoveryHistory)-1] = recoveryRecord
	} else {
		erm.recoveryHistory = append(erm.recoveryHistory, recoveryRecord)
	}

	// Update error record if recovery was successful
	if success {
		for i := range erm.errorHistory {
			if erm.errorHistory[i].Timestamp.Equal(record.Timestamp) &&
				erm.errorHistory[i].ErrorType == record.ErrorType &&
				erm.errorHistory[i].Component == record.Component {
				now := time.Now()
				erm.errorHistory[i].Recovered = true
				erm.errorHistory[i].RecoveryTime = &now
				break
			}
		}

		// Record circuit breaker success
		if circuitBreaker := erm.circuitBreakers[record.ErrorType]; circuitBreaker != nil {
			circuitBreaker.RecordResult(nil)
		}

		log.Infof("Recovery successful: type=%s, component=%s, strategy=%s, duration=%v",
			record.ErrorType, record.Component, strategy.Name(), duration)
	} else {
		// Record circuit breaker failure
		if circuitBreaker := erm.circuitBreakers[record.ErrorType]; circuitBreaker != nil {
			circuitBreaker.RecordResult(fmt.Errorf("recovery failed: %v", err))
		}

		log.Errorf("Recovery failed: type=%s, component=%s, strategy=%s, duration=%v, error=%v",
			record.ErrorType, record.Component, strategy.Name(), duration, err)
	}
	erm.mu.Unlock()

	// Record metrics
	if erm.metrics != nil {
		erm.metrics.RecordHealthCheck(fmt.Sprintf("recovery_%s", record.ErrorType), success, duration)
		if !success {
			erm.metrics.RecordHealthCheckError(fmt.Sprintf("recovery_%s", record.ErrorType), "recovery_failed")
		}
	}
}

// getStackTrace gets current stack trace
func getStackTrace() string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var trace strings.Builder
	for {
		frame, more := frames.Next()
		trace.WriteString(fmt.Sprintf("\n\t%s:%d", frame.File, frame.Line))
		if !more {
			break
		}
	}
	return trace.String()
}

// Optimized: Cache memory stats to avoid stop-the-world pauses during error recording
var (
	cachedMemoryAlloc atomic.Uint64
	memStatsOnce      sync.Once
	memStatsStop      chan struct{}
)

// Optimized: Cache environment variables to avoid repeated os.Getenv calls
var (
	cachedEnv     string
	cachedVersion string
	envOnce       sync.Once
)

// initMemoryStatsCache initializes the background goroutine for memory stats updates
func initMemoryStatsCache() {
	memStatsOnce.Do(func() {
		memStatsStop = make(chan struct{})
		// Initial update
		updateMemoryStats()
		// Start background goroutine to update memory stats periodically
		// Update every 1 second to balance accuracy and performance
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					updateMemoryStats()
				case <-memStatsStop:
					return
				}
			}
		}()
	})
}

// cleanupMemoryStatsCache stops the background goroutine for memory stats updates
// Fix Bug 2: Provides a way to gracefully shut down the memory stats goroutine
// This should be called during application shutdown to prevent goroutine leaks
func cleanupMemoryStatsCache() {
	if memStatsStop != nil {
		select {
		case <-memStatsStop:
			// Already closed
		default:
			close(memStatsStop)
		}
	}
}

// updateMemoryStats updates the cached memory statistics
// This function performs the expensive ReadMemStats operation
func updateMemoryStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	cachedMemoryAlloc.Store(m.Alloc)
}

// getMemoryStats gets current memory statistics from cache
// Optimized: Returns cached value to avoid stop-the-world pauses
func getMemoryStats() uint64 {
	// Initialize cache if not already done
	initMemoryStatsCache()
	return cachedMemoryAlloc.Load()
}

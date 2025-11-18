package app

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/app/observability/metrics"
)

// CircuitBreaker provides error handling and recovery
type CircuitBreaker struct {
	mu           sync.RWMutex
	state        CircuitState
	failureCount int
	successCount int
	lastFailure  time.Time
	threshold    int
	timeout      time.Duration
}

// CircuitState represents the state of circuit breaker
type CircuitState int

const (
	CircuitStateClosed CircuitState = iota
	CircuitStateOpen
	CircuitStateHalfOpen
)

// CanExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitStateClosed:
		return true
	case CircuitStateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailure) >= cb.timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = CircuitStateHalfOpen
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case CircuitStateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordResult records the result of an operation
func (cb *CircuitBreaker) RecordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailure = time.Now()

		if cb.state == CircuitStateClosed && cb.failureCount >= cb.threshold {
			cb.state = CircuitStateOpen
		} else if cb.state == CircuitStateHalfOpen {
			cb.state = CircuitStateOpen
		}
	} else {
		cb.successCount++

		if cb.state == CircuitStateHalfOpen {
			cb.state = CircuitStateClosed
			cb.resetCounters()
		}
	}
}

// resetCounters resets the circuit breaker counters
func (cb *CircuitBreaker) resetCounters() {
	cb.failureCount = 0
	cb.successCount = 0
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

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
	recoverySemaphore chan struct{}
	maxConcurrentRecoveries int
	activeRecoveries        sync.Map // map[string]context.CancelFunc - track active recoveries
}

// ErrorRecord represents a recorded error with enhanced context
type ErrorRecord struct {
	Timestamp    time.Time
	ErrorType    string
	Category     ErrorCategory
	Message      string
	Component    string
	Severity     ErrorSeverity
	Context      map[string]interface{}
	Recovered    bool
	RecoveryTime *time.Time
	// Additional fields
	StackTrace  string
	UserID      string
	RequestID   string
	Environment string
	Version     string
}

// RecoveryRecord represents a recovery attempt
type RecoveryRecord struct {
	Timestamp time.Time
	ErrorType string
	Component string
	Strategy  string
	Success   bool
	Duration  time.Duration
	Message   string
}

// ErrorSeverity represents error severity levels
type ErrorSeverity int

const (
	ErrorSeverityLow ErrorSeverity = iota
	ErrorSeverityMedium
	ErrorSeverityHigh
	ErrorSeverityCritical
)

// ErrorCategory represents error categories for better classification
type ErrorCategory string

const (
	ErrorCategoryNetwork    ErrorCategory = "network"
	ErrorCategoryDatabase   ErrorCategory = "database"
	ErrorCategoryConfig     ErrorCategory = "configuration"
	ErrorCategoryPlugin     ErrorCategory = "plugin"
	ErrorCategoryResource   ErrorCategory = "resource"
	ErrorCategorySecurity   ErrorCategory = "security"
	ErrorCategoryTimeout    ErrorCategory = "timeout"
	ErrorCategoryValidation ErrorCategory = "validation"
	ErrorCategorySystem     ErrorCategory = "system"
)

// RecoveryStrategy defines a recovery strategy
type RecoveryStrategy interface {
	Name() string
	CanRecover(errorType string, severity ErrorSeverity) bool
	Recover(ctx context.Context, record ErrorRecord) (bool, error)
	GetTimeout() time.Duration
}

// DefaultRecoveryStrategy implements a basic recovery strategy
type DefaultRecoveryStrategy struct {
	name    string
	timeout time.Duration
}

// NewDefaultRecoveryStrategy creates a new default recovery strategy
func NewDefaultRecoveryStrategy(name string, timeout time.Duration) *DefaultRecoveryStrategy {
	return &DefaultRecoveryStrategy{
		name:    name,
		timeout: timeout,
	}
}

// Name returns the strategy name
func (s *DefaultRecoveryStrategy) Name() string {
	return s.name
}

// CanRecover checks if this strategy can recover from the error
func (s *DefaultRecoveryStrategy) CanRecover(errorType string, severity ErrorSeverity) bool {
	// Default strategy can handle low and medium severity errors
	return severity <= ErrorSeverityMedium
}

// Recover attempts to recover from the error
func (s *DefaultRecoveryStrategy) Recover(ctx context.Context, record ErrorRecord) (bool, error) {
	// Default recovery: wait and retry
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(s.timeout):
		// Simulate recovery success for now
		return true, nil
	}
}

// GetTimeout returns the recovery timeout
func (s *DefaultRecoveryStrategy) GetTimeout() time.Duration {
	return s.timeout
}

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
	erm.circuitBreakers[errorType] = &CircuitBreaker{
		state:     CircuitStateClosed,
		threshold: 5,
		timeout:   60 * time.Second,
	}
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
	if env := os.Getenv("ENV"); env != "" {
		context["environment"] = env
	}
	if version := os.Getenv("APP_VERSION"); version != "" {
		context["version"] = version
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
	erm.errorHistory = append(erm.errorHistory, record)
	if len(erm.errorHistory) > erm.maxErrorHistory {
		erm.errorHistory = erm.errorHistory[1:]
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
func (erm *ErrorRecoveryManager) attemptRecovery(record ErrorRecord) {
	// Generate recovery key first
	recoveryKey := fmt.Sprintf("%s:%s:%d", record.ErrorType, record.Component, record.Timestamp.Unix())
	
	// CRITICAL: Atomically check and store recovery key to prevent TOCTOU race condition
	// Use a placeholder cancel function to mark that recovery is starting
	placeholderCancel := func() {} // No-op function
	if _, loaded := erm.activeRecoveries.LoadOrStore(recoveryKey, placeholderCancel); loaded {
		// Recovery already in progress for this error
		log.Debugf("Recovery already in progress for %s:%s, skipping duplicate attempt", record.ErrorType, record.Component)
		return
	}
	// Defer cleanup of the recovery key - will be disabled once we successfully start recovery
	shouldCleanupKey := true
	defer func() {
		if shouldCleanupKey {
			erm.activeRecoveries.Delete(recoveryKey)
		}
	}()

	erm.mu.RLock()
	strategy, exists := erm.recoveryStrategies[record.ErrorType]
	erm.mu.RUnlock()

	if !exists {
		// Use default strategy
		erm.mu.RLock()
		strategy = erm.recoveryStrategies["transient"]
		erm.mu.RUnlock()
		if strategy == nil {
			log.Warnf("No recovery strategy found for error type: %s", record.ErrorType)
			return
		}
	}

	// Check if strategy can recover
	if !strategy.CanRecover(record.ErrorType, record.Severity) {
		log.Warnf("Recovery strategy %s cannot recover from error type: %s", strategy.Name(), record.ErrorType)
		return
	}

	// Acquire semaphore to limit concurrent recoveries
	select {
	case erm.recoverySemaphore <- struct{}{}:
		// Acquired semaphore, proceed with recovery
		defer func() { <-erm.recoverySemaphore }()
	case <-time.After(1 * time.Second):
		// Semaphore acquisition timeout - too many concurrent recoveries
		log.Warnf("Recovery semaphore timeout for %s:%s, skipping recovery (too many concurrent recoveries)", 
			record.ErrorType, record.Component)
		return
	case <-erm.stopChan:
		// Recovery manager is stopping
		return
	}

	// Create recovery context with timeout and cancellation
	recoveryTimeout := strategy.GetTimeout()
	if recoveryTimeout <= 0 {
		recoveryTimeout = erm.recoveryTimeout
	}
	if recoveryTimeout > 60*time.Second {
		recoveryTimeout = 60 * time.Second // Cap at 60 seconds
	}

	ctx, cancel := context.WithTimeout(context.Background(), recoveryTimeout)
	defer cancel()

	// Replace placeholder with actual cancel function
	// Now that we've successfully started recovery, disable the cleanup defer
	// and set up proper cleanup at the end of recovery
	erm.activeRecoveries.Store(recoveryKey, cancel)
	shouldCleanupKey = false // Disable the early cleanup defer
	
	// Set up proper cleanup at the end of recovery
	defer erm.activeRecoveries.Delete(recoveryKey)

	// Attempt recovery with timeout protection
	startTime := time.Now()
	recoveryDone := make(chan struct {
		success bool
		err     error
	}, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				recoveryDone <- struct {
					success bool
					err     error
				}{false, fmt.Errorf("panic during recovery: %v", r)}
			}
		}()
		success, err := strategy.Recover(ctx, record)
		recoveryDone <- struct {
			success bool
			err     error
		}{success, err}
	}()

	var success bool
	var err error
	select {
	case result := <-recoveryDone:
		success = result.success
		err = result.err
	case <-ctx.Done():
		// Timeout reached
		success = false
		err = fmt.Errorf("recovery timeout after %v: %w", recoveryTimeout, ctx.Err())
		log.Warnf("Recovery timeout for %s:%s after %v", record.ErrorType, record.Component, recoveryTimeout)
	case <-erm.stopChan:
		// Recovery manager is stopping
		cancel()
		return
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
	erm.recoveryHistory = append(erm.recoveryHistory, recoveryRecord)
	if len(erm.recoveryHistory) > erm.maxRecoveryHistory {
		erm.recoveryHistory = erm.recoveryHistory[1:]
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

// getMemoryStats gets current memory statistics
func getMemoryStats() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

// GetErrorStats returns error statistics
func (erm *ErrorRecoveryManager) GetErrorStats() map[string]interface{} {
	erm.mu.RLock()
	defer erm.mu.RUnlock()

	stats := make(map[string]interface{})

	// Error counts by type
	stats["error_counts"] = erm.errorCounts

	// Recent errors (last 10)
	recentErrors := make([]map[string]interface{}, 0)
	for i := len(erm.errorHistory) - 1; i >= 0 && len(recentErrors) < 10; i-- {
		record := erm.errorHistory[i]
		recentErrors = append(recentErrors, map[string]interface{}{
			"timestamp":     record.Timestamp,
			"error_type":    record.ErrorType,
			"component":     record.Component,
			"severity":      record.Severity,
			"message":       record.Message,
			"recovered":     record.Recovered,
			"recovery_time": record.RecoveryTime,
		})
	}
	stats["recent_errors"] = recentErrors

	// Recovery statistics
	recoveryStats := make(map[string]interface{})
	totalRecoveries := len(erm.recoveryHistory)
	successfulRecoveries := 0

	for _, record := range erm.recoveryHistory {
		if record.Success {
			successfulRecoveries++
		}
	}

	recoveryStats["total"] = totalRecoveries
	recoveryStats["successful"] = successfulRecoveries
	recoveryStats["success_rate"] = 0.0
	if totalRecoveries > 0 {
		recoveryStats["success_rate"] = float64(successfulRecoveries) / float64(totalRecoveries)
	}

	stats["recovery_stats"] = recoveryStats

	// Circuit breaker states
	circuitBreakerStates := make(map[string]interface{})
	for errorType, cb := range erm.circuitBreakers {
		circuitBreakerStates[errorType] = map[string]interface{}{
			"state": cb.GetState(),
		}
	}
	stats["circuit_breaker_states"] = circuitBreakerStates

	return stats
}

// GetErrorHistory returns error history
func (erm *ErrorRecoveryManager) GetErrorHistory() []ErrorRecord {
	erm.mu.RLock()
	defer erm.mu.RUnlock()

	// Return a copy to avoid race conditions
	history := make([]ErrorRecord, len(erm.errorHistory))
	copy(history, erm.errorHistory)
	return history
}

// GetRecoveryHistory returns recovery history
func (erm *ErrorRecoveryManager) GetRecoveryHistory() []RecoveryRecord {
	erm.mu.RLock()
	defer erm.mu.RUnlock()

	// Return a copy to avoid race conditions
	history := make([]RecoveryRecord, len(erm.recoveryHistory))
	copy(history, erm.recoveryHistory)
	return history
}

// ClearHistory clears error and recovery history
func (erm *ErrorRecoveryManager) ClearHistory() {
	erm.mu.Lock()
	defer erm.mu.Unlock()

	erm.errorHistory = make([]ErrorRecord, 0)
	erm.recoveryHistory = make([]RecoveryRecord, 0)
	erm.errorCounts = make(map[string]int64)
}

// Stop stops the error recovery manager
func (erm *ErrorRecoveryManager) Stop() {
	// Use sync.Once to ensure Stop() can only be called once
	erm.stopOnce.Do(func() {
		// Signal stop to all recovery operations
		close(erm.stopChan)

		// Cancel all active recoveries
		erm.activeRecoveries.Range(func(key, value interface{}) bool {
			if cancel, ok := value.(context.CancelFunc); ok {
				cancel()
			}
			erm.activeRecoveries.Delete(key)
			return true
		})

		// Wait for all recovery operations to complete (with timeout)
		done := make(chan struct{})
		go func() {
			// Wait for semaphore to be fully released
		semaphoreWait:
			for i := 0; i < erm.maxConcurrentRecoveries; i++ {
				select {
				case erm.recoverySemaphore <- struct{}{}:
					// Acquired, release immediately
					<-erm.recoverySemaphore
				case <-time.After(5 * time.Second):
					// Timeout waiting for semaphore
					break semaphoreWait
				}
			}
			close(done)
		}()

		select {
		case <-done:
			// All recoveries completed
		case <-time.After(10 * time.Second):
			// Timeout - some recoveries may still be running
			log.Warnf("Error recovery manager stop timeout: some recoveries may still be running")
		}
	})
}

// IsHealthy returns the health status of the error recovery manager
func (erm *ErrorRecoveryManager) IsHealthy() bool {
	erm.mu.RLock()
	defer erm.mu.RUnlock()

	// Check if error count exceeds threshold
	for errorType, count := range erm.errorCounts {
		if count > erm.errorThreshold {
			log.Warnf("Error count for %s exceeds threshold: %d > %d", errorType, count, erm.errorThreshold)
			return false
		}
	}

	// Check circuit breaker states
	for errorType, cb := range erm.circuitBreakers {
		if cb.GetState() == CircuitStateOpen {
			log.Warnf("Circuit breaker is open for error type: %s", errorType)
			return false
		}
	}

	return true
}

// GetHealthReport returns a detailed health report
func (erm *ErrorRecoveryManager) GetHealthReport() map[string]interface{} {
	stats := erm.GetErrorStats()

	report := map[string]interface{}{
		"healthy":           erm.IsHealthy(),
		"error_stats":       stats,
		"error_threshold":   erm.errorThreshold,
		"recovery_timeout":  erm.recoveryTimeout,
		"max_error_history": erm.maxErrorHistory,
	}

	return report
}

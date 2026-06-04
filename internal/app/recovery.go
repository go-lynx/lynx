// Error recovery: ErrorRecoveryManager records classified errors and, when
// severity allows, runs per-error-type recovery strategies guarded by circuit
// breakers and a concurrency semaphore.
package app

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

// recoveryState tracks an active recovery so Stop() can cancel it.
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
	erm.RegisterRecoveryStrategy("transient", NewDefaultRecoveryStrategy("retry", 5*time.Second))
	erm.RegisterRecoveryStrategy("component", NewDefaultRecoveryStrategy("restart", 10*time.Second))
	erm.RegisterRecoveryStrategy("critical", NewDefaultRecoveryStrategy("fallback", 15*time.Second))
}

// RegisterRecoveryStrategy registers a recovery strategy
func (erm *ErrorRecoveryManager) RegisterRecoveryStrategy(errorType string, strategy RecoveryStrategy) {
	erm.mu.Lock()
	defer erm.mu.Unlock()

	erm.recoveryStrategies[errorType] = strategy

	// Each error type gets its own breaker so one failing type cannot trip others.
	erm.circuitBreakers[errorType] = NewCircuitBreaker(5, 60*time.Second)
}

// RecordError records an error with enhanced context and classification
func (erm *ErrorRecoveryManager) RecordError(errorType string, category ErrorCategory, message, component string, severity ErrorSeverity, context map[string]any) {
	erm.mu.Lock()
	defer erm.mu.Unlock()

	if context == nil {
		context = make(map[string]any)
	}

	context["timestamp"] = time.Now().Unix()
	context["goroutines"] = runtime.NumGoroutine()
	context["memory_alloc"] = getMemoryStats()

	// ENV/APP_VERSION are read once and cached; this runs on every error.
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

	// Ring-buffer eviction via copy (not [1:]) so the old backing array is not retained.
	if len(erm.errorHistory) >= erm.maxErrorHistory {
		copy(erm.errorHistory, erm.errorHistory[1:])
		erm.errorHistory[len(erm.errorHistory)-1] = record
	} else {
		erm.errorHistory = append(erm.errorHistory, record)
	}

	erm.errorCounts[errorType]++

	if erm.metrics != nil {
		erm.metrics.RecordPluginError(component, errorType, message)
	}

	log.Errorf("Error recorded: type=%s, category=%s, component=%s, severity=%d, message=%s, context=%+v",
		errorType, category, component, severity, message, context)

	// Skip recovery while the breaker is open to avoid hammering a failing component.
	circuitBreaker := erm.circuitBreakers[errorType]
	if circuitBreaker != nil && !circuitBreaker.CanExecute() {
		log.Warnf("Circuit breaker is open for error type: %s", errorType)
		return
	}

	if severity <= ErrorSeverityHigh {
		go erm.attemptRecovery(record)
	}
}

// attemptRecovery runs the recovery strategy for a recorded error in a bounded,
// cancellable goroutine. It de-duplicates concurrent attempts for the same error
// and respects the manager's stopChan so it cannot outlive Stop().
func (erm *ErrorRecoveryManager) attemptRecovery(record ErrorRecord) {
	recoveryKey := fmt.Sprintf("%s:%s:%d", record.ErrorType, record.Component, record.Timestamp.Unix())

	if _, exists := erm.activeRecoveries.Load(recoveryKey); exists {
		log.Debugf("Recovery already in progress for %s:%s, skipping duplicate attempt", record.ErrorType, record.Component)
		return
	}

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

	if !strategy.CanRecover(record.ErrorType, record.Severity) {
		log.Warnf("Recovery strategy %s cannot recover from error type: %s", strategy.Name(), record.ErrorType)
		return
	}

	recoveryTimeout := strategy.GetTimeout()
	if recoveryTimeout <= 0 {
		recoveryTimeout = erm.recoveryTimeout
	}
	if recoveryTimeout > 60*time.Second {
		recoveryTimeout = 60 * time.Second
	}

	// Parent context is cancelled either by timeout or by stopChan, so the
	// recovery goroutine and its stop monitor can never outlive the manager.
	parentCtx, parentCancel := context.WithCancel(context.Background())
	ctx, timeoutCancel := context.WithTimeout(parentCtx, recoveryTimeout)

	stopMonitorDone := make(chan struct{}, 1)
	go func() {
		defer func() {
			select {
			case stopMonitorDone <- struct{}{}:
			default:
			}
		}()
		select {
		case <-erm.stopChan:
			parentCancel()
		case <-ctx.Done():
		}
	}()

	defer func() {
		parentCancel()
		timeoutCancel()
		stopWaitTimer := time.NewTimer(50 * time.Millisecond)
		defer stopWaitTimer.Stop()
		select {
		case <-stopMonitorDone:
		case <-stopWaitTimer.C:
		}
	}()

	// Acquire semaphore to limit concurrent recoveries.
	// semaphoreHeld tracks whether THIS goroutine owns a semaphore slot so that
	// the deferred release only runs when we actually hold one.  Previously the
	// defer was registered unconditionally while a manual release was also issued
	// in the LoadOrStore-collision branch, causing a double-release.
	semaphoreHeld := false
	defer func() {
		if semaphoreHeld {
			<-erm.recoverySemaphore
		}
	}()

	semaphoreTimeout := defaultRecoverySemaphoreTimeout
	semaphoreTimer := time.NewTimer(semaphoreTimeout)
	defer semaphoreTimer.Stop()
	select {
	case erm.recoverySemaphore <- struct{}{}:
		semaphoreHeld = true
	case <-semaphoreTimer.C:
		log.Warnf("Recovery semaphore timeout for %s:%s after %v, skipping recovery (too many concurrent recoveries)",
			record.ErrorType, record.Component, semaphoreTimeout)
		return
	case <-ctx.Done():
		return
	}

	// cancel is parentCancel (not timeoutCancel) so Stop() also tears down the monitor goroutine.
	state := &recoveryState{
		cancel:  parentCancel,
		started: true,
	}
	if _, loaded := erm.activeRecoveries.LoadOrStore(recoveryKey, state); loaded {
		// Lost the race to another attempt for this key; the deferred semaphore
		// release (semaphoreHeld=true) frees the slot we just claimed.
		return
	}

	defer func() {
		erm.activeRecoveries.Delete(recoveryKey)
	}()

	startTime := time.Now()
	recoveryDone := make(chan struct {
		success bool
		err     error
	}, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr := normalizeRecoveredPanic(r)
				select {
				case recoveryDone <- struct {
					success bool
					err     error
				}{false, fmt.Errorf("panic during recovery: %w", panicErr)}:
				case <-ctx.Done():
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
		}
	}()

	var success bool
	var err error
	select {
	case result := <-recoveryDone:
		success = result.success
		err = result.err
	case <-ctx.Done():
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
	// Ring-buffer eviction via copy (not [1:]) so the old backing array is not retained.
	if len(erm.recoveryHistory) >= erm.maxRecoveryHistory {
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

func normalizeRecoveredPanic(value any) error {
	switch v := value.(type) {
	case nil:
		return nil
	case error:
		return v
	case string:
		return fmt.Errorf("%s", v)
	default:
		return fmt.Errorf("panic value (%T): %v", v, v)
	}
}

// Memory stats are sampled by a background goroutine and cached so that error
// recording never triggers a stop-the-world ReadMemStats on the hot path.
var (
	cachedMemoryAlloc atomic.Uint64
	memStatsOnce      sync.Once
	memStatsMu        sync.Mutex
	memStatsStop      chan struct{}
)

var (
	cachedEnv     string
	cachedVersion string
	envOnce       sync.Once
)

// initMemoryStatsCache initializes the background goroutine for memory stats updates
func initMemoryStatsCache() {
	memStatsMu.Lock()
	defer memStatsMu.Unlock()

	memStatsOnce.Do(func() {
		stop := make(chan struct{})
		memStatsStop = stop
		updateMemoryStats()
		// Resample every second: fresh enough for diagnostics, cheap enough to ignore.
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					updateMemoryStats()
				case <-stop:
					return // released by cleanupMemoryStatsCache during shutdown
				}
			}
		}()
	})
}

// cleanupMemoryStatsCache stops the background sampling goroutine. Called during
// application shutdown so the goroutine does not leak. Resets the sync.Once so a
// later getMemoryStats can restart sampling.
func cleanupMemoryStatsCache() {
	memStatsMu.Lock()
	defer memStatsMu.Unlock()

	if memStatsStop != nil {
		select {
		case <-memStatsStop:
		default:
			close(memStatsStop)
		}
		memStatsStop = nil
	}
	memStatsOnce = sync.Once{}
}

// updateMemoryStats performs the expensive ReadMemStats and caches the result.
func updateMemoryStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	cachedMemoryAlloc.Store(m.Alloc)
}

// getMemoryStats returns the most recently sampled Alloc value, starting the
// background sampler on first use.
func getMemoryStats() uint64 {
	initMemoryStatsCache()
	return cachedMemoryAlloc.Load()
}

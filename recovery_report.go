package lynx

import (
	"context"
	"time"

	"github.com/go-lynx/lynx/log"
)

// GetErrorStats returns error statistics.
func (erm *ErrorRecoveryManager) GetErrorStats() map[string]any {
	erm.mu.RLock()
	defer erm.mu.RUnlock()

	stats := make(map[string]any)

	errorCounts := make(map[string]int64, len(erm.errorCounts))
	for errorType, count := range erm.errorCounts {
		errorCounts[errorType] = count
		stats[errorType] = count
	}
	stats["error_counts"] = errorCounts

	recentErrors := make([]map[string]any, 0)
	for i := len(erm.errorHistory) - 1; i >= 0 && len(recentErrors) < 10; i-- {
		record := erm.errorHistory[i]
		recentErrors = append(recentErrors, map[string]any{
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

	recoveryStats := make(map[string]any)
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

	circuitBreakerStates := make(map[string]any)
	for errorType, cb := range erm.circuitBreakers {
		circuitBreakerStates[errorType] = map[string]any{
			"state": cb.GetState(),
		}
	}
	stats["circuit_breaker_states"] = circuitBreakerStates

	return stats
}

// GetErrorHistory returns error history.
func (erm *ErrorRecoveryManager) GetErrorHistory() []ErrorRecord {
	erm.mu.RLock()
	defer erm.mu.RUnlock()

	history := make([]ErrorRecord, len(erm.errorHistory))
	copy(history, erm.errorHistory)
	return history
}

// GetRecoveryHistory returns recovery history.
func (erm *ErrorRecoveryManager) GetRecoveryHistory() []RecoveryRecord {
	erm.mu.RLock()
	defer erm.mu.RUnlock()

	history := make([]RecoveryRecord, len(erm.recoveryHistory))
	copy(history, erm.recoveryHistory)
	return history
}

// ClearHistory clears error and recovery history.
func (erm *ErrorRecoveryManager) ClearHistory() {
	erm.mu.Lock()
	defer erm.mu.Unlock()

	erm.errorHistory = make([]ErrorRecord, 0)
	erm.recoveryHistory = make([]RecoveryRecord, 0)
	erm.errorCounts = make(map[string]int64)
}

// Stop stops the error recovery manager.
func (erm *ErrorRecoveryManager) Stop() {
	erm.stopOnce.Do(func() {
		close(erm.stopChan)

		erm.activeRecoveries.Range(func(key, value any) bool {
			if cancel, ok := value.(context.CancelFunc); ok {
				cancel()
			} else if state, ok := value.(*recoveryState); ok && state != nil && state.cancel != nil {
				state.cancel()
			}
			erm.activeRecoveries.Delete(key)
			return true
		})

		done := make(chan struct{})
		go func() {
		semaphoreWait:
			for i := 0; i < erm.maxConcurrentRecoveries; i++ {
				select {
				case erm.recoverySemaphore <- struct{}{}:
					<-erm.recoverySemaphore
				case <-time.After(5 * time.Second):
					break semaphoreWait
				}
			}
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			log.Warnf("Error recovery manager stop timeout: some recoveries may still be running")
		}
	})
}

// IsHealthy returns the health status of the error recovery manager.
func (erm *ErrorRecoveryManager) IsHealthy() bool {
	erm.mu.RLock()
	defer erm.mu.RUnlock()

	for errorType, count := range erm.errorCounts {
		if count > erm.errorThreshold {
			log.Warnf("Error count for %s exceeds threshold: %d > %d", errorType, count, erm.errorThreshold)
			return false
		}
	}

	for errorType, cb := range erm.circuitBreakers {
		if cb.GetState() == CircuitStateOpen {
			log.Warnf("Circuit breaker is open for error type: %s", errorType)
			return false
		}
	}

	return true
}

// GetHealthReport returns a detailed health report.
func (erm *ErrorRecoveryManager) GetHealthReport() map[string]any {
	stats := erm.GetErrorStats()

	report := map[string]any{
		"healthy":           erm.IsHealthy(),
		"error_stats":       stats,
		"error_threshold":   erm.errorThreshold,
		"recovery_timeout":  erm.recoveryTimeout,
		"max_error_history": erm.maxErrorHistory,
	}

	return report
}

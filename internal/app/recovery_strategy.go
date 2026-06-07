package app

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-lynx/lynx/log"
)

// RecoveryStrategy defines the contract for error recovery strategies.
type RecoveryStrategy interface {
	Name() string
	CanRecover(errorType string, severity ErrorSeverity) bool
	Recover(ctx context.Context, record ErrorRecord) (bool, error)
	GetTimeout() time.Duration
}

// ActionFunc is an optional user-supplied hook that strategies call during recovery.
// Return nil to indicate success, non-nil to indicate the attempt failed.
type ActionFunc func(ctx context.Context, record ErrorRecord) error

// ─── RetryRecoveryStrategy ────────────────────────────────────────────────────

// RetryRecoveryStrategy retries transient errors with exponential back-off and
// ±25% jitter.  If ActionFunc is set it is called on each attempt; without it
// the strategy waits through the back-off schedule (useful when the caller's
// own retry loop is the actual recovery action).
type RetryRecoveryStrategy struct {
	name       string
	timeout    time.Duration
	maxRetries int
	baseDelay  time.Duration
	// ActionFunc is called on each attempt. Optional.
	ActionFunc ActionFunc
}

// NewRetryRecoveryStrategy creates a RetryRecoveryStrategy with up to 3 attempts
// and exponential back-off bounded by timeout.
func NewRetryRecoveryStrategy(timeout time.Duration) *RetryRecoveryStrategy {
	return &RetryRecoveryStrategy{
		name:       "retry",
		timeout:    timeout,
		maxRetries: 3,
		baseDelay:  500 * time.Millisecond,
	}
}

func (s *RetryRecoveryStrategy) Name() string             { return s.name }
func (s *RetryRecoveryStrategy) GetTimeout() time.Duration { return s.timeout }

// CanRecover returns true for Low and Medium severity (transient errors).
func (s *RetryRecoveryStrategy) CanRecover(_ string, severity ErrorSeverity) bool {
	return severity <= ErrorSeverityMedium
}

// Recover performs up to maxRetries attempts with exponential back-off + jitter.
// Returns (true, nil) when an attempt succeeds or all retries are exhausted;
// (false, err) on context cancellation or when all ActionFunc calls fail.
func (s *RetryRecoveryStrategy) Recover(ctx context.Context, record ErrorRecord) (bool, error) {
	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		delay := s.backoffDelay(attempt)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false, ctx.Err()
		case <-timer.C:
		}

		if s.ActionFunc == nil {
			// No action hook: the back-off delay itself is the recovery work.
			// Signal "caller may now retry the operation".
			log.Infof("retry recovery: back-off %v complete for %s:%s (attempt %d/%d)",
				delay, record.ErrorType, record.Component, attempt, s.maxRetries)
			continue
		}

		if err := s.ActionFunc(ctx, record); err != nil {
			log.Warnf("retry recovery: attempt %d/%d failed for %s:%s: %v",
				attempt, s.maxRetries, record.ErrorType, record.Component, err)
			if attempt == s.maxRetries {
				return false, fmt.Errorf("all %d retry attempts failed; last error: %w", s.maxRetries, err)
			}
			continue
		}
		log.Infof("retry recovery: attempt %d/%d succeeded for %s:%s",
			attempt, s.maxRetries, record.ErrorType, record.Component)
		return true, nil
	}
	return true, nil
}

// backoffDelay computes base×2^(attempt-1) capped at timeout/maxRetries, ±25% jitter.
func (s *RetryRecoveryStrategy) backoffDelay(attempt int) time.Duration {
	base := s.baseDelay * (1 << uint(attempt-1))
	if cap := s.timeout / time.Duration(s.maxRetries); base > cap {
		base = cap
	}
	jitter := time.Duration(rand.Int63n(int64(base)/4 + 1))
	if rand.Intn(2) == 0 {
		return base + jitter
	}
	d := base - jitter
	if d < 0 {
		return base
	}
	return d
}

// ─── RestartRecoveryStrategy ──────────────────────────────────────────────────

// RestartRecoveryStrategy waits for a cooldown period then signals that the
// component should be restarted.  If ActionFunc is set it is invoked after the
// cooldown as the actual restart mechanism (e.g. unloading and re-loading a plugin).
type RestartRecoveryStrategy struct {
	name       string
	timeout    time.Duration
	// ActionFunc is invoked after the cooldown as the restart action. Optional.
	ActionFunc ActionFunc
}

// NewRestartRecoveryStrategy creates a RestartRecoveryStrategy. The cooldown
// before restart is timeout/2 (minimum 2 s).
func NewRestartRecoveryStrategy(timeout time.Duration) *RestartRecoveryStrategy {
	return &RestartRecoveryStrategy{
		name:    "restart",
		timeout: timeout,
	}
}

func (s *RestartRecoveryStrategy) Name() string             { return s.name }
func (s *RestartRecoveryStrategy) GetTimeout() time.Duration { return s.timeout }

// CanRecover returns true for Medium and High severity (component-level errors).
func (s *RestartRecoveryStrategy) CanRecover(_ string, severity ErrorSeverity) bool {
	return severity == ErrorSeverityMedium || severity == ErrorSeverityHigh
}

// Recover waits for the cooldown period then invokes ActionFunc if set.
// Returns (false, err) on context cancellation or action failure,
// (true, nil) when the restart cycle completes.
func (s *RestartRecoveryStrategy) Recover(ctx context.Context, record ErrorRecord) (bool, error) {
	cooldown := s.timeout / 2
	if cooldown <= 0 {
		cooldown = 2 * time.Second
	}
	log.Infof("restart recovery: waiting cooldown %v before restarting %s:%s",
		cooldown, record.ErrorType, record.Component)

	timer := time.NewTimer(cooldown)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-timer.C:
	}

	if s.ActionFunc != nil {
		if err := s.ActionFunc(ctx, record); err != nil {
			return false, fmt.Errorf("restart action failed for %s:%s: %w",
				record.ErrorType, record.Component, err)
		}
	}

	log.Infof("restart recovery: cooldown complete for %s:%s", record.ErrorType, record.Component)
	return true, nil
}

// ─── FallbackRecoveryStrategy ─────────────────────────────────────────────────

// FallbackRecoveryStrategy activates graceful degradation immediately.
// It is the last-resort strategy and can handle any severity level.
// If ActionFunc is set it executes the actual fallback (e.g. switching to a
// read replica, returning cached data, disabling a feature flag).
// For Critical severity without an ActionFunc the strategy returns failure so
// the error remains flagged as unrecovered — avoiding a misleading success log.
type FallbackRecoveryStrategy struct {
	name       string
	timeout    time.Duration
	// ActionFunc is called to execute the fallback action. Optional.
	ActionFunc ActionFunc
}

// NewFallbackRecoveryStrategy creates a FallbackRecoveryStrategy.
func NewFallbackRecoveryStrategy(timeout time.Duration) *FallbackRecoveryStrategy {
	return &FallbackRecoveryStrategy{
		name:    "fallback",
		timeout: timeout,
	}
}

func (s *FallbackRecoveryStrategy) Name() string             { return s.name }
func (s *FallbackRecoveryStrategy) GetTimeout() time.Duration { return s.timeout }

// CanRecover returns true for all severity levels.
func (s *FallbackRecoveryStrategy) CanRecover(_ string, _ ErrorSeverity) bool { return true }

// Recover activates the fallback immediately (no wait).
// With ActionFunc: calls the hook and reports its outcome.
// Without ActionFunc: returns (true, nil) for non-critical severity;
// (false, err) for Critical to signal manual intervention is required.
func (s *FallbackRecoveryStrategy) Recover(ctx context.Context, record ErrorRecord) (bool, error) {
	if s.ActionFunc != nil {
		if err := s.ActionFunc(ctx, record); err != nil {
			return false, fmt.Errorf("fallback action failed for %s:%s: %w",
				record.ErrorType, record.Component, err)
		}
		log.Infof("fallback recovery: action succeeded for %s:%s",
			record.ErrorType, record.Component)
		return true, nil
	}

	if record.Severity >= ErrorSeverityCritical {
		log.Errorf("fallback recovery: critical error in %s:%s requires manual intervention — %s",
			record.ErrorType, record.Component, record.Message)
		return false, fmt.Errorf("critical error in component %s requires manual intervention: %s",
			record.Component, record.Message)
	}

	log.Infof("fallback recovery: entering degraded mode for %s:%s",
		record.ErrorType, record.Component)
	return true, nil
}

// ─── DefaultRecoveryStrategy (backward compat) ────────────────────────────────

// DefaultRecoveryStrategy is retained for backward compatibility.
// New code should prefer RetryRecoveryStrategy, RestartRecoveryStrategy, or
// FallbackRecoveryStrategy.
//
// Deprecated: use the dedicated constructors.
type DefaultRecoveryStrategy struct {
	name    string
	timeout time.Duration
}

// NewDefaultRecoveryStrategy creates a DefaultRecoveryStrategy.
//
// Deprecated: prefer NewRetryRecoveryStrategy / NewRestartRecoveryStrategy /
// NewFallbackRecoveryStrategy.
func NewDefaultRecoveryStrategy(name string, timeout time.Duration) *DefaultRecoveryStrategy {
	return &DefaultRecoveryStrategy{name: name, timeout: timeout}
}

func (s *DefaultRecoveryStrategy) Name() string             { return s.name }
func (s *DefaultRecoveryStrategy) GetTimeout() time.Duration { return s.timeout }
func (s *DefaultRecoveryStrategy) CanRecover(_ string, severity ErrorSeverity) bool {
	return severity <= ErrorSeverityMedium
}

func (s *DefaultRecoveryStrategy) Recover(ctx context.Context, _ ErrorRecord) (bool, error) {
	timer := time.NewTimer(s.timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-timer.C:
		return true, nil
	}
}

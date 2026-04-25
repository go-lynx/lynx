package lynx

import (
	"context"
	"time"
)

// RecoveryStrategy defines a recovery strategy.
type RecoveryStrategy interface {
	Name() string
	CanRecover(errorType string, severity ErrorSeverity) bool
	Recover(ctx context.Context, record ErrorRecord) (bool, error)
	GetTimeout() time.Duration
}

// DefaultRecoveryStrategy implements a basic recovery strategy.
type DefaultRecoveryStrategy struct {
	name    string
	timeout time.Duration
}

// NewDefaultRecoveryStrategy creates a new default recovery strategy.
func NewDefaultRecoveryStrategy(name string, timeout time.Duration) *DefaultRecoveryStrategy {
	return &DefaultRecoveryStrategy{
		name:    name,
		timeout: timeout,
	}
}

// Name returns the strategy name.
func (s *DefaultRecoveryStrategy) Name() string {
	return s.name
}

// CanRecover checks if this strategy can recover from the error.
func (s *DefaultRecoveryStrategy) CanRecover(errorType string, severity ErrorSeverity) bool {
	return severity <= ErrorSeverityMedium
}

// Recover attempts to recover from the error.
func (s *DefaultRecoveryStrategy) Recover(ctx context.Context, record ErrorRecord) (bool, error) {
	timer := time.NewTimer(s.timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-timer.C:
		return true, nil
	}
}

// GetTimeout returns the recovery timeout.
func (s *DefaultRecoveryStrategy) GetTimeout() time.Duration {
	return s.timeout
}

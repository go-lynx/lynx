package base

import (
	"time"
)

// MetricsRecorder defines the interface for recording database metrics
type MetricsRecorder interface {
	// RecordConnectionPoolStats records connection pool statistics
	RecordConnectionPoolStats(stats *ConnectionPoolStats)

	// RecordHealthCheck records health check results
	RecordHealthCheck(success bool)

	// RecordQuery records SQL query duration and errors
	RecordQuery(duration time.Duration, err error, threshold time.Duration)

	// RecordTx records transaction duration and status
	RecordTx(duration time.Duration, committed bool)

	// IncConnectAttempt increments connection attempt counter
	IncConnectAttempt()

	// IncConnectRetry increments connection retry counter
	IncConnectRetry()

	// IncConnectSuccess increments connection success counter
	IncConnectSuccess()

	// IncConnectFailure increments connection failure counter
	IncConnectFailure()
}

// NoOpMetricsRecorder provides a no-operation implementation of MetricsRecorder
// This is useful when metrics recording is disabled or not implemented
type NoOpMetricsRecorder struct{}

// RecordConnectionPoolStats implements MetricsRecorder
func (n *NoOpMetricsRecorder) RecordConnectionPoolStats(stats *ConnectionPoolStats) {}

// RecordHealthCheck implements MetricsRecorder
func (n *NoOpMetricsRecorder) RecordHealthCheck(success bool) {}

// RecordQuery implements MetricsRecorder
func (n *NoOpMetricsRecorder) RecordQuery(duration time.Duration, err error, threshold time.Duration) {
}

// RecordTx implements MetricsRecorder
func (n *NoOpMetricsRecorder) RecordTx(duration time.Duration, committed bool) {}

// IncConnectAttempt implements MetricsRecorder
func (n *NoOpMetricsRecorder) IncConnectAttempt() {}

// IncConnectRetry implements MetricsRecorder
func (n *NoOpMetricsRecorder) IncConnectRetry() {}

// IncConnectSuccess implements MetricsRecorder
func (n *NoOpMetricsRecorder) IncConnectSuccess() {}

// IncConnectFailure implements MetricsRecorder
func (n *NoOpMetricsRecorder) IncConnectFailure() {}

// MetricsConfig defines configuration for metrics recording
type MetricsConfig struct {
	// Enabled determines if metrics recording is enabled
	Enabled bool

	// Namespace for Prometheus metrics
	Namespace string

	// Subsystem for Prometheus metrics
	Subsystem string

	// Labels to add to all metrics
	Labels map[string]string

	// SlowQueryThreshold defines the threshold for slow query detection
	SlowQueryThreshold time.Duration
}

// DefaultMetricsConfig returns default metrics configuration
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		Enabled:            true,
		Namespace:          "lynx",
		Subsystem:          "sql",
		Labels:             make(map[string]string),
		SlowQueryThreshold: 1 * time.Second,
	}
}

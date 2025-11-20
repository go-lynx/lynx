package base

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// QueryMonitor provides slow query monitoring and logging
type QueryMonitor struct {
	enabled   bool
	threshold time.Duration
	recorder  MetricsRecorder
}

// NewQueryMonitor creates a new query monitor
func NewQueryMonitor(enabled bool, threshold time.Duration, recorder MetricsRecorder) *QueryMonitor {
	return &QueryMonitor{
		enabled:   enabled,
		threshold: threshold,
		recorder:  recorder,
	}
}

// MonitorQuery wraps a database query with monitoring
func (m *QueryMonitor) MonitorQuery(ctx context.Context, db *sql.DB, query string, args []interface{}, fn func() error) error {
	if !m.enabled {
		return fn()
	}

	start := time.Now()
	err := fn()
	duration := time.Since(start)

	// Record query metrics
	if m.recorder != nil {
		m.recorder.RecordQuery(duration, err, m.threshold)
	}

	// Log slow queries
	if duration >= m.threshold {
		log.Warnf("Slow query detected: duration=%v, query=%s, args=%v, error=%v",
			duration, query, args, err)
	}

	return err
}

// MonitorQueryRow wraps a database query row with monitoring
func (m *QueryMonitor) MonitorQueryRow(ctx context.Context, db *sql.DB, query string, args []interface{}, scan func(*sql.Row) error) error {
	if !m.enabled {
		row := db.QueryRowContext(ctx, query, args...)
		return scan(row)
	}

	start := time.Now()
	row := db.QueryRowContext(ctx, query, args...)
	err := scan(row)
	duration := time.Since(start)

	// Record query metrics
	if m.recorder != nil {
		m.recorder.RecordQuery(duration, err, m.threshold)
	}

	// Log slow queries
	if duration >= m.threshold {
		log.Warnf("Slow query detected: duration=%v, query=%s, args=%v, error=%v",
			duration, query, args, err)
	}

	return err
}

// MonitorExec wraps a database exec with monitoring
func (m *QueryMonitor) MonitorExec(ctx context.Context, db *sql.DB, query string, args []interface{}) (sql.Result, error) {
	if !m.enabled {
		return db.ExecContext(ctx, query, args...)
	}

	start := time.Now()
	result, err := db.ExecContext(ctx, query, args...)
	duration := time.Since(start)

	// Record query metrics
	if m.recorder != nil {
		m.recorder.RecordQuery(duration, err, m.threshold)
	}

	// Log slow queries
	if duration >= m.threshold {
		log.Warnf("Slow query detected: duration=%v, query=%s, args=%v, error=%v",
			duration, query, args, err)
	}

	return result, err
}


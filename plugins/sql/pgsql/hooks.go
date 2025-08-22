//go:build lynx_pgsql_hooks
// +build lynx_pgsql_hooks

package pgsql

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-lynx/lynx/api/plugins/sql/pgsql/conf"
	"github.com/jackc/pgconn"
)

// Global metrics and thresholds (assigned by pgsql.go during initialization)
var (
	globalPgsqlMetrics  *PrometheusMetrics
	globalSlowThreshold = 200 * time.Millisecond
	globalPgsqlConf     *conf.Pgsql
)

type ctxKey string

const (
	ctxStartKey ctxKey = "lynx_pgsql_start"
)

type metricsHooks struct{}

// Before conforms to sqlhooks v2 interface
func (h *metricsHooks) Before(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	return context.WithValue(ctx, ctxStartKey, time.Now()), nil
}

// After conforms to sqlhooks v2 interface (no err parameter)
func (h *metricsHooks) After(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	start, _ := ctx.Value(ctxStartKey).(time.Time)
	if !start.IsZero() && globalPgsqlMetrics != nil {
		dur := time.Since(start)
		// Record as ok when no error information
		globalPgsqlMetrics.RecordQuery(parseOp(query), dur, nil, globalSlowThreshold, globalPgsqlConf, "")
	}
	return ctx, nil
}

// OnError captures errors to record error codes and latency
func (h *metricsHooks) OnError(ctx context.Context, err error, query string, args ...interface{}) error {
	start, _ := ctx.Value(ctxStartKey).(time.Time)
	if !start.IsZero() && globalPgsqlMetrics != nil {
		dur := time.Since(start)
		globalPgsqlMetrics.RecordQuery(parseOp(query), dur, err, globalSlowThreshold, globalPgsqlConf, extractSQLState(err))
	}
	return err
}

// Helper: Extract SQLSTATE
func extractSQLState(err error) string {
	if err == nil {
		return ""
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr != nil {
		return pgErr.SQLState()
	}
	return ""
}

// parseOp simply parses SQL operation name as op label
func parseOp(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return "query"
	}
	// Take the first word
	i := strings.IndexFunc(q, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' })
	if i <= 0 {
		return strings.ToLower(q)
	}
	return strings.ToLower(q[:i])
}

// NewMetricsHooks for pgsql.go registration use
func NewMetricsHooks() sqlhooks.Hooks {
	return &metricsHooks{}
}

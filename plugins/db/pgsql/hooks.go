//go:build lynx_pgsql_hooks
// +build lynx_pgsql_hooks

package pgsql

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgconn"
	"github.com/qustavo/sqlhooks/v2"
	"github.com/go-lynx/lynx/plugins/db/pgsql/conf"
)

// 全局指标与阈值（由 pgsql.go 在初始化时赋值）
var (
	globalPgsqlMetrics *PrometheusMetrics
	globalSlowThreshold = 200 * time.Millisecond
	globalPgsqlConf     *conf.Pgsql
)

type ctxKey string

const (
	ctxStartKey ctxKey = "lynx_pgsql_start"
)

type metricsHooks struct{}

// Before 符合 sqlhooks v2 接口
func (h *metricsHooks) Before(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	return context.WithValue(ctx, ctxStartKey, time.Now()), nil
}

// After 符合 sqlhooks v2 接口（无 err 参数）
func (h *metricsHooks) After(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	start, _ := ctx.Value(ctxStartKey).(time.Time)
	if !start.IsZero() && globalPgsqlMetrics != nil {
		dur := time.Since(start)
		// 无错误信息时记录为 ok
		globalPgsqlMetrics.RecordQuery(parseOp(query), dur, nil, globalSlowThreshold, globalPgsqlConf, "")
	}
	return ctx, nil
}

// OnError 捕获错误以记录错误码与时延
func (h *metricsHooks) OnError(ctx context.Context, err error, query string, args ...interface{}) error {
	start, _ := ctx.Value(ctxStartKey).(time.Time)
	if !start.IsZero() && globalPgsqlMetrics != nil {
		dur := time.Since(start)
		globalPgsqlMetrics.RecordQuery(parseOp(query), dur, err, globalSlowThreshold, globalPgsqlConf, extractSQLState(err))
	}
	return err
}

// Helper: 提取 SQLSTATE
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

// parseOp 简单解析 SQL 操作名作为 op 标签
func parseOp(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return "query"
	}
	// 取首个单词
	i := strings.IndexFunc(q, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' })
	if i <= 0 {
		return strings.ToLower(q)
	}
	return strings.ToLower(q[:i])
}

// NewMetricsHooks 供 pgsql.go 注册使用
func NewMetricsHooks() sqlhooks.Hooks {
	return &metricsHooks{}
}

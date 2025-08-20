//go:build lynx_pgsql_hooks
// +build lynx_pgsql_hooks

package pgsql

import (
	"database/sql"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/qustavo/sqlhooks/v2"
)

// 在启用 hooks 的构建下，使用 sqlhooks 包装 pgx 驱动并注册
func registerDriver() {
	// 注册名保持为 "postgres"，与 p.conf.Driver 默认一致
	sql.Register("postgres", sqlhooks.Wrap(stdlib.GetDefaultDriver(), NewMetricsHooks()))
}

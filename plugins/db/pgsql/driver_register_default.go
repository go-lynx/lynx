//go:build !lynx_pgsql_hooks
// +build !lynx_pgsql_hooks

package pgsql

import (
	"database/sql"

	"github.com/jackc/pgx/v5/stdlib"
)

// 默认构建下，注册原始 pgx 驱动
func registerDriver() {
	sql.Register("postgres", stdlib.GetDefaultDriver())
}

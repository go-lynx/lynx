//go:build !lynx_pgsql_hooks
// +build !lynx_pgsql_hooks

package pgsql

import (
	"database/sql"

	"github.com/jackc/pgx/v5/stdlib"
)

// In default build, register original pgx driver
func registerDriver() {
	sql.Register("postgres", stdlib.GetDefaultDriver())
}

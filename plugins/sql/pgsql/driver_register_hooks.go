//go:build lynx_pgsql_hooks
// +build lynx_pgsql_hooks

package pgsql

import (
	"database/sql"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/qustavo/sqlhooks/v2"
)

// In builds with hooks enabled, wrap pgx driver with sqlhooks and register
func registerDriver() {
	// Registration name remains "postgres", consistent with p.conf.Driver default
	sql.Register("postgres", sqlhooks.Wrap(stdlib.GetDefaultDriver(), NewMetricsHooks()))
}

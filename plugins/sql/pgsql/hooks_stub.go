package pgsql

import (
	"time"

	"github.com/go-lynx/lynx/plugins/sql/pgsql/conf"
)

// This file is a placeholder implementation for default builds, providing global variables to avoid compilation errors when hooks are not enabled.
// The real hooks implementation is in hooks.go (requires building with -tags lynx_pgsql_hooks).

var (
	globalPgsqlMetrics  *PrometheusMetrics
	globalSlowThreshold = 200 * time.Millisecond
	globalPgsqlConf     *conf.Pgsql
)

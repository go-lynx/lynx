package interfaces

import (
	"context"
	"database/sql"

	"github.com/go-lynx/lynx/plugins"
)

// SQLPlugin defines the core SQL plugin interface
// Keep it simple and provide freedom for users
type SQLPlugin interface {
	plugins.Plugin

	// GetDB returns the underlying database connection
	// Users can use this to integrate with any ORM or query builder
	GetDB() (*sql.DB, error)

	// GetDBWithContext returns the underlying database connection with context support
	// This allows for timeout and cancellation control
	GetDBWithContext(ctx context.Context) (*sql.DB, error)

	// GetDialect returns the database dialect (mysql, postgres, mssql, etc.)
	GetDialect() string

	// IsConnected returns whether the database is connected
	IsConnected() bool
}

// Config represents common database configuration
type Config struct {
	// Driver name (mysql, postgres, mssql, etc.)
	Driver string `json:"driver"`

	// Data Source Name (DSN) or connection string
	DSN string `json:"dsn"`

	// Connection pool settings
	MaxOpenConns    int `json:"max_open_conns"`
	MaxIdleConns    int `json:"max_idle_conns"`
	ConnMaxLifetime int `json:"conn_max_lifetime"`  // in seconds
	ConnMaxIdleTime int `json:"conn_max_idle_time"` // in seconds

	// Health check settings (optional)
	HealthCheckInterval int    `json:"health_check_interval"` // in seconds, 0 to disable
	HealthCheckQuery    string `json:"health_check_query"`    // custom query for health check

	// Connection retry settings
	RetryEnabled      bool `json:"retry_enabled"`       // enable connection retry on startup failure
	RetryMaxAttempts  int  `json:"retry_max_attempts"`  // maximum retry attempts (default: 3)
	RetryInitialDelay int  `json:"retry_initial_delay"` // initial retry delay in seconds (default: 1)
	RetryMaxDelay      int  `json:"retry_max_delay"`    // maximum retry delay in seconds (default: 30)
	RetryMultiplier    float64 `json:"retry_multiplier"` // exponential backoff multiplier (default: 2.0)

	// Connection pool monitoring and alerting
	MonitorEnabled        bool    `json:"monitor_enabled"`         // enable connection pool monitoring
	MonitorInterval       int     `json:"monitor_interval"`        // monitoring interval in seconds (default: 30)
	AlertThresholdUsage   float64 `json:"alert_threshold_usage"`  // alert when pool usage exceeds this percentage (default: 0.8 = 80%)
	AlertThresholdWait    int     `json:"alert_threshold_wait"`    // alert when wait duration exceeds this in seconds (default: 5)
	AlertThresholdWaitCount int64 `json:"alert_threshold_wait_count"` // alert when wait count exceeds this (default: 10)
}

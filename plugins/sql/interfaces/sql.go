package interfaces

import (
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
}

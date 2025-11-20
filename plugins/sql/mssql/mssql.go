package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/base"
	"github.com/go-lynx/lynx/plugins/sql/interfaces"
	"github.com/go-lynx/lynx/plugins/sql/mssql/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Plugin metadata constants
const (
	pluginName        = "mssql.client"
	pluginVersion     = "v2.0.0"
	pluginDescription = "Microsoft SQL Server client plugin for lynx framework"
	confPrefix        = "lynx.mssql"
)

// DBMssqlClient represents Microsoft SQL Server client plugin instance
type DBMssqlClient struct {
	*base.SQLPlugin
	config            *conf.Mssql
	closeChan         chan struct{}
	closeOnce         sync.Once // Protect against multiple close operations
	closed            bool
	prometheusMetrics *PrometheusMetrics
}

// NewMssqlClient creates a new Microsoft SQL Server client plugin instance
func NewMssqlClient() *DBMssqlClient {
	mssqlConf := &conf.Mssql{
		Driver:      "mssql",
		Source:      "", // Will be built from ServerConfig
		MinConn:     5,
		MaxConn:     20,
		MaxIdleTime: &durationpb.Duration{Seconds: 300},  // 5 minutes
		MaxLifeTime: &durationpb.Duration{Seconds: 3600}, // 1 hour
		ServerConfig: &conf.ServerConfig{
			InstanceName:           "localhost",
			Port:                   1433,
			Database:               "master",
			Encrypt:                false,
			TrustServerCertificate: false,
			ConnectionTimeout:      30,
			CommandTimeout:         30,
			ApplicationName:        "Lynx-MSSQL-Plugin",
			ConnectionPooling:      true,
			MaxPoolSize:            20,
			MinPoolSize:            5,
			PoolBlockingTimeout:    30,
			PoolLifetimeTimeout:    3600,
		},
	}

	c := &DBMssqlClient{
		config:    mssqlConf,
		closeChan: make(chan struct{}),
		closed:    false,
	}

	// Convert conf.Mssql to interfaces.Config
	baseConfig := convertToBaseConfig(mssqlConf)

	c.SQLPlugin = base.NewBaseSQLPlugin(
		plugins.GeneratePluginID("", pluginName, pluginVersion),
		pluginName,
		pluginDescription,
		pluginVersion,
		confPrefix,
		102, // Weight for MSSQL
		baseConfig,
	)
	return c
}

// Configure updates Microsoft SQL Server configuration
func (m *DBMssqlClient) Configure(c any) error {
	if mssqlConf, ok := c.(*conf.Mssql); ok {
		m.config = mssqlConf
		return nil
	}
	return plugins.ErrInvalidConfiguration
}

// InitializeResources initializes the plugin with configuration
func (m *DBMssqlClient) InitializeResources(rt plugins.Runtime) error {
	// Validate configuration
	// Validate configuration
	if m.config.Driver == "" {
		return fmt.Errorf("driver is required")
	}
	if m.config.Source == "" && m.config.ServerConfig == nil {
		return fmt.Errorf("source or server config is required")
	}

	// Build connection string if not provided
	if m.config.Source == "" {
		m.config.Source = buildDSN(m.config)
	}

	// Initialize SQL plugin
	if err := m.SQLPlugin.InitializeResources(rt); err != nil {
		return err
	}

	return nil
}

// StartupTasks initializes database connection and performs health check
func (m *DBMssqlClient) StartupTasks() error {
	log.Infof("initializing Microsoft SQL Server database connection")

	// Initialize Prometheus metrics
	m.initPrometheusMetrics()

	// Start SQL plugin
	if err := m.SQLPlugin.StartupTasks(); err != nil {
		return err
	}

	// Update statistics
	m.updateStats()

	log.Infof("Microsoft SQL Server database successfully initialized with connection pool: max_open=%d, max_idle=%d",
		m.config.MaxConn, m.config.MinConn)

	// Start background tasks
	go m.backgroundTasks()

	return nil
}

// CleanupTasks gracefully shuts down the plugin
func (m *DBMssqlClient) CleanupTasks() error {
	log.Infof("shutting down Microsoft SQL Server client plugin")

	// Signal background tasks to stop (protected against multiple calls)
	m.closeOnce.Do(func() {
		close(m.closeChan)
	})
	m.closed = true

	// Cleanup SQL plugin
	if err := m.SQLPlugin.CleanupTasks(); err != nil {
		return err
	}

	log.Infof("Microsoft SQL Server client plugin successfully shut down")
	return nil
}

// CheckHealth performs comprehensive health check on database connection
func (m *DBMssqlClient) CheckHealth() error {
	if err := m.SQLPlugin.CheckHealth(); err != nil {
		return err
	}

	m.updateStats()

	if m.prometheusMetrics != nil {
		m.prometheusMetrics.RecordHealthCheck(true, m.config)
	}

	return nil
}

// initPrometheusMetrics initializes Prometheus monitoring metrics
func (m *DBMssqlClient) initPrometheusMetrics() {
	promConfig := createPrometheusConfig(m.config)
	m.prometheusMetrics = NewPrometheusMetrics(promConfig)
}

// updateStats updates connection pool statistics
func (m *DBMssqlClient) updateStats() {
	stats := m.SQLPlugin.GetStats()
	if m.prometheusMetrics != nil {
		m.prometheusMetrics.RecordConnectionPoolStats(stats)
	}
}

// backgroundTasks runs background maintenance tasks
func (m *DBMssqlClient) backgroundTasks() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !m.closed {
				m.updateStats()
			}
		case <-m.closeChan:
			return
		}
	}
}

// GetMssqlConfig returns the current MSSQL configuration
func (m *DBMssqlClient) GetMssqlConfig() *conf.Mssql {
	return m.config
}

// TestConnection tests the database connection with a simple query
func (m *DBMssqlClient) TestConnection(ctx context.Context) error {
	db, err := m.SQLPlugin.GetDB()
	if err != nil || db == nil {
		return fmt.Errorf("database connection not initialized")
	}

	// Test with a simple query
	query := "SELECT 1"
	var result int
	err = db.QueryRowContext(ctx, query).Scan(&result)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("unexpected test result: %d", result)
	}

	return nil
}

// GetServerInfo retrieves SQL Server version and configuration information
func (m *DBMssqlClient) GetServerInfo(ctx context.Context) (map[string]interface{}, error) {
	db, err := m.SQLPlugin.GetDB()
	if err != nil || db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	info := make(map[string]interface{})

	// Get SQL Server version
	var version string
	err = db.QueryRowContext(ctx, "SELECT @@VERSION").Scan(&version)
	if err != nil {
		return nil, fmt.Errorf("failed to get SQL Server version: %w", err)
	}
	info["version"] = version

	// Get database name
	var dbName string
	err = db.QueryRowContext(ctx, "SELECT DB_NAME()").Scan(&dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to get database name: %w", err)
	}
	info["database"] = dbName

	// Get server name
	var serverName string
	err = db.QueryRowContext(ctx, "SELECT @@SERVERNAME").Scan(&serverName)
	if err != nil {
		return nil, fmt.Errorf("failed to get server name: %w", err)
	}
	info["server_name"] = serverName

	// Get connection count
	var connectionCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sys.dm_exec_connections").Scan(&connectionCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection count: %w", err)
	}
	info["connection_count"] = connectionCount

	return info, nil
}

// ExecuteStoredProcedure executes a stored procedure with parameters
func (m *DBMssqlClient) ExecuteStoredProcedure(ctx context.Context, procName string, args ...interface{}) (*sql.Rows, error) {
	db, err := m.SQLPlugin.GetDB()
	if err != nil || db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	// Build the stored procedure call
	query := fmt.Sprintf("EXEC %s", procName)
	if len(args) > 0 {
		placeholders := make([]string, len(args))
		for i := range args {
			placeholders[i] = "?"
		}
		query += " " + strings.Join(placeholders, ", ")
	}

	return db.QueryContext(ctx, query, args...)
}

// BeginTransaction starts a new database transaction
func (m *DBMssqlClient) BeginTransaction(ctx context.Context) (*sql.Tx, error) {
	db, err := m.SQLPlugin.GetDB()
	if err != nil || db == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	return db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	})
}

// IsConnected checks if the database is connected
func (m *DBMssqlClient) IsConnected() bool {
	return !m.closed && m.SQLPlugin.IsConnected()
}

// GetConnectionStats returns detailed connection statistics
func (m *DBMssqlClient) GetConnectionStats() map[string]interface{} {
	stats := m.SQLPlugin.GetStats()

	result := map[string]interface{}{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"max_idle_connections": stats.MaxIdleConnections,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration.String(),
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
		"driver":               m.config.Driver,
		"server_instance":      m.config.ServerConfig.InstanceName,
		"port":                 m.config.ServerConfig.Port,
		"database":             m.config.ServerConfig.Database,
		"encryption_enabled":   m.config.ServerConfig.Encrypt,
		"connection_pooling":   m.config.ServerConfig.ConnectionPooling,
		"application_name":     m.config.ServerConfig.ApplicationName,
	}

	return result
}

// convertToBaseConfig converts conf.Mssql to interfaces.Config
func convertToBaseConfig(mssqlConf *conf.Mssql) *interfaces.Config {
	// Build DSN from ServerConfig
	dsn := buildDSN(mssqlConf)

	// Convert duration fields
	var maxLifetime, maxIdleTime int
	if mssqlConf.MaxLifeTime != nil {
		maxLifetime = int(mssqlConf.MaxLifeTime.AsDuration().Seconds())
	}
	if mssqlConf.MaxIdleTime != nil {
		maxIdleTime = int(mssqlConf.MaxIdleTime.AsDuration().Seconds())
	}

	// Handle MaxIdleConns: if MaxIdleConn is 0 or not set, use 0 to let base plugin set default
	// If MaxIdleConn is explicitly set, use it; otherwise calculate a reasonable default
	maxIdleConns := int(mssqlConf.MaxIdleConn)
	if maxIdleConns == 0 && mssqlConf.MaxConn > 0 {
		// If MaxIdleConn is not set but MaxConn is, use a reasonable default (20% of MaxConn, min 1)
		maxIdleConns = int(mssqlConf.MaxConn) / 5
		if maxIdleConns < 1 {
			maxIdleConns = 1
		}
	}

	return &interfaces.Config{
		Driver:              mssqlConf.Driver,
		DSN:                 dsn,
		MaxOpenConns:        int(mssqlConf.MaxConn),
		MaxIdleConns:        maxIdleConns,
		ConnMaxLifetime:     maxLifetime,
		ConnMaxIdleTime:     maxIdleTime,
		HealthCheckInterval: 30, // Default 30 seconds
		HealthCheckQuery:    "SELECT 1",
	}
}

// buildDSN builds a DSN string from ServerConfig
func buildDSN(mssqlConf *conf.Mssql) string {
	if mssqlConf.ServerConfig == nil {
		return mssqlConf.Source
	}

	server := mssqlConf.ServerConfig.InstanceName
	port := mssqlConf.ServerConfig.Port
	database := mssqlConf.ServerConfig.Database

	// Build basic DSN
	dsn := fmt.Sprintf("server=%s;port=%d;database=%s", server, port, database)

	// Add optional parameters
	if mssqlConf.ServerConfig.Encrypt {
		dsn += ";encrypt=true"
	} else {
		dsn += ";encrypt=false"
	}

	if mssqlConf.ServerConfig.TrustServerCertificate {
		dsn += ";trustservercertificate=true"
	}

	if mssqlConf.ServerConfig.ConnectionTimeout > 0 {
		dsn += fmt.Sprintf(";connection timeout=%d", mssqlConf.ServerConfig.ConnectionTimeout)
	}

	if mssqlConf.ServerConfig.CommandTimeout > 0 {
		dsn += fmt.Sprintf(";command timeout=%d", mssqlConf.ServerConfig.CommandTimeout)
	}

	if mssqlConf.ServerConfig.ApplicationName != "" {
		dsn += fmt.Sprintf(";app name=%s", mssqlConf.ServerConfig.ApplicationName)
	}

	if mssqlConf.ServerConfig.ConnectionPooling {
		dsn += ";connection pooling=true"
		if mssqlConf.ServerConfig.MaxPoolSize > 0 {
			dsn += fmt.Sprintf(";max pool size=%d", mssqlConf.ServerConfig.MaxPoolSize)
		}
		if mssqlConf.ServerConfig.MinPoolSize > 0 {
			dsn += fmt.Sprintf(";min pool size=%d", mssqlConf.ServerConfig.MinPoolSize)
		}
		if mssqlConf.ServerConfig.PoolBlockingTimeout > 0 {
			dsn += fmt.Sprintf(";pool blocking timeout=%d", mssqlConf.ServerConfig.PoolBlockingTimeout)
		}
		if mssqlConf.ServerConfig.PoolLifetimeTimeout > 0 {
			dsn += fmt.Sprintf(";pool lifetime timeout=%d", mssqlConf.ServerConfig.PoolLifetimeTimeout)
		}
	}

	return dsn
}

package base

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/interfaces"
)

var (
	ErrNotConnected  = errors.New("database not connected")
	ErrAlreadyClosed = errors.New("database already closed")
)

// ConnectionPoolStats represents database connection pool statistics
type ConnectionPoolStats struct {
	MaxOpenConnections int64         // Maximum number of open connections
	OpenConnections    int64         // Number of established connections
	InUse              int64         // Number of connections currently in use
	Idle               int64         // Number of idle connections
	MaxIdleConnections int64         // Maximum number of idle connections
	WaitCount          int64         // Total number of connections waited for
	WaitDuration       time.Duration // Total time blocked waiting for a new connection
	MaxIdleClosed      int64         // Total number of connections closed due to SetMaxIdleConns
	MaxLifetimeClosed  int64         // Total number of connections closed due to SetConnMaxLifetime
}

// BaseSQLPlugin provides common functionality for all SQL plugins
type BaseSQLPlugin struct {
	*plugins.BasePlugin

	// Configuration
	config     *interfaces.Config
	confPrefix string

	// Database connection
	db      *sql.DB
	dialect string

	// Connection state
	mu        sync.RWMutex
	connected atomic.Bool
	closing   atomic.Bool

	// Health check
	healthChecker *HealthChecker

	// Metrics recording
	metricsRecorder MetricsRecorder

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// NewBaseSQLPlugin creates a new base SQL plugin
func NewBaseSQLPlugin(
	id, name, desc, version, confPrefix string,
	weight int,
	config *interfaces.Config,
) *BaseSQLPlugin {
	ctx, cancel := context.WithCancel(context.Background())

	return &BaseSQLPlugin{
		BasePlugin:      plugins.NewBasePlugin(id, name, desc, version, confPrefix, weight),
		config:          config,
		confPrefix:      confPrefix,
		metricsRecorder: &NoOpMetricsRecorder{}, // Default to no-op metrics
		ctx:             ctx,
		cancel:          cancel,
	}
}

// InitializeResources initializes plugin resources
func (p *BaseSQLPlugin) InitializeResources(rt plugins.Runtime) error {
	// Load configuration
	if err := rt.GetConfig().Value(p.confPrefix).Scan(p.config); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Set default values
	if p.config.MaxOpenConns == 0 {
		p.config.MaxOpenConns = 25
	}
	if p.config.MaxIdleConns == 0 {
		p.config.MaxIdleConns = 5
	}

	return nil
}

// StartupTasks performs startup initialization
func (p *BaseSQLPlugin) StartupTasks() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connected.Load() {
		return errors.New("already connected")
	}

	log.Infof("Initializing database connection for %s", p.Name())

	// Open database connection
	db, err := sql.Open(p.config.Driver, p.config.DSN)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(p.config.MaxOpenConns)
	db.SetMaxIdleConns(p.config.MaxIdleConns)

	if p.config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(p.config.ConnMaxLifetime) * time.Second)
	}
	if p.config.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(time.Duration(p.config.ConnMaxIdleTime) * time.Second)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	p.db = db
	p.dialect = p.getDialectFromDriver(p.config.Driver)
	p.connected.Store(true)

	// Start health checker if configured
	if p.config.HealthCheckInterval > 0 {
		p.healthChecker = NewHealthChecker(
			p,
			time.Duration(p.config.HealthCheckInterval)*time.Second,
			p.config.HealthCheckQuery,
		)
		p.healthChecker.Start(p.ctx)
	}

	log.Infof("Database connection established for %s", p.Name())
	return nil
}

// CleanupTasks performs cleanup on shutdown
func (p *BaseSQLPlugin) CleanupTasks() error {
	if !p.closing.CompareAndSwap(false, true) {
		return ErrAlreadyClosed
	}

	log.Infof("Shutting down database connection for %s", p.Name())

	// Stop background tasks
	p.cancel()

	// Stop health checker
	if p.healthChecker != nil {
		p.healthChecker.Stop()
	}

	// Close database connection
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.db != nil {
		if err := p.db.Close(); err != nil {
			log.Warnf("Error closing database connection for %s: %v", p.Name(), err)
		} else {
			log.Infof("Database connection closed for %s", p.Name())
		}
	}

	p.connected.Store(false)
	return nil
}

// GetDB returns the database connection
func (p *BaseSQLPlugin) GetDB() (*sql.DB, error) {
	if !p.IsConnected() {
		return nil, ErrNotConnected
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.db, nil
}

// GetDialect returns the database dialect
func (p *BaseSQLPlugin) GetDialect() string {
	return p.dialect
}

// SetMetricsRecorder sets the metrics recorder for this plugin
func (p *BaseSQLPlugin) SetMetricsRecorder(recorder MetricsRecorder) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metricsRecorder = recorder
}

// GetMetricsRecorder returns the current metrics recorder
func (p *BaseSQLPlugin) GetMetricsRecorder() MetricsRecorder {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metricsRecorder
}

// CheckHealth performs a health check
func (p *BaseSQLPlugin) CheckHealth() error {
	if !p.IsConnected() {
		p.metricsRecorder.RecordHealthCheck(false)
		return ErrNotConnected
	}

	db, err := p.GetDB()
	if err != nil {
		p.metricsRecorder.RecordHealthCheck(false)
		return err
	}

	// Use custom health check query if configured
	query := "SELECT 1"
	if p.config.HealthCheckQuery != "" {
		query = p.config.HealthCheckQuery
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	var result int
	if err := db.QueryRowContext(ctx, query).Scan(&result); err != nil {
		p.metricsRecorder.RecordHealthCheck(false)
		return fmt.Errorf("health check failed: %w", err)
	}

	p.metricsRecorder.RecordHealthCheck(true)
	return nil
}

// IsConnected checks if database is connected
func (p *BaseSQLPlugin) IsConnected() bool {
	return p.connected.Load() && !p.closing.Load()
}

// GetStats returns connection pool statistics
func (p *BaseSQLPlugin) GetStats() *ConnectionPoolStats {
	if !p.IsConnected() || p.db == nil {
		return &ConnectionPoolStats{}
	}

	stats := p.db.Stats()
	return &ConnectionPoolStats{
		MaxOpenConnections: int64(stats.MaxOpenConnections),
		OpenConnections:    int64(stats.OpenConnections),
		InUse:              int64(stats.InUse),
		Idle:               int64(stats.Idle),
		MaxIdleConnections: int64(p.config.MaxIdleConns), // Use config value instead
		WaitCount:          int64(stats.WaitCount),
		WaitDuration:       stats.WaitDuration,
		MaxIdleClosed:      int64(stats.MaxIdleClosed),
		MaxLifetimeClosed:  int64(stats.MaxLifetimeClosed),
	}
}

// getDialectFromDriver determines the dialect from the driver name
func (p *BaseSQLPlugin) getDialectFromDriver(driver string) string {
	dialectMap := map[string]string{
		"mysql":      "mysql",
		"postgres":   "postgres",
		"pgx":        "postgres",
		"mssql":      "mssql",
		"sqlserver":  "mssql",
		"sqlite3":    "sqlite",
		"sqlite":     "sqlite",
		"clickhouse": "clickhouse",
	}

	if dialect, ok := dialectMap[driver]; ok {
		return dialect
	}
	return driver
}

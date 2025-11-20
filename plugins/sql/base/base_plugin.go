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

// SQLPlugin provides common functionality for all SQL plugins
type SQLPlugin struct {
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

	// Pool monitoring
	poolMonitor *PoolMonitor

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
) *SQLPlugin {
	ctx, cancel := context.WithCancel(context.Background())

	return &SQLPlugin{
		BasePlugin:      plugins.NewBasePlugin(id, name, desc, version, confPrefix, weight),
		config:          config,
		confPrefix:      confPrefix,
		metricsRecorder: &NoOpMetricsRecorder{}, // Default to no-op metrics
		ctx:             ctx,
		cancel:          cancel,
	}
}

// InitializeResources initializes plugin resources
func (p *SQLPlugin) InitializeResources(rt plugins.Runtime) error {
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

	// Set default retry values
	if p.config.RetryMaxAttempts == 0 {
		p.config.RetryMaxAttempts = 3
	}
	if p.config.RetryInitialDelay == 0 {
		p.config.RetryInitialDelay = 1
	}
	if p.config.RetryMaxDelay == 0 {
		p.config.RetryMaxDelay = 30
	}
	if p.config.RetryMultiplier == 0 {
		p.config.RetryMultiplier = 2.0
	}

	// Set default monitoring values
	if p.config.MonitorInterval == 0 {
		p.config.MonitorInterval = 30
	}
	if p.config.AlertThresholdUsage == 0 {
		p.config.AlertThresholdUsage = 0.8
	}
	if p.config.AlertThresholdWait == 0 {
		p.config.AlertThresholdWait = 5
	}
	if p.config.AlertThresholdWaitCount == 0 {
		p.config.AlertThresholdWaitCount = 10
	}

	// Validate configuration
	if err := p.validateConfig(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	return nil
}

// validateConfig validates the configuration for correctness
func (p *SQLPlugin) validateConfig() error {
	if p.config.Driver == "" {
		return fmt.Errorf("driver is required")
	}
	if p.config.DSN == "" {
		return fmt.Errorf("DSN is required")
	}
	if p.config.MaxIdleConns > p.config.MaxOpenConns {
		return fmt.Errorf("max_idle_conns (%d) cannot be greater than max_open_conns (%d)",
			p.config.MaxIdleConns, p.config.MaxOpenConns)
	}
	if p.config.MaxOpenConns <= 0 {
		return fmt.Errorf("max_open_conns must be greater than 0")
	}
	if p.config.MaxIdleConns < 0 {
		return fmt.Errorf("max_idle_conns cannot be negative")
	}
	if p.config.RetryMaxAttempts < 0 {
		return fmt.Errorf("retry_max_attempts cannot be negative")
	}
	if p.config.AlertThresholdUsage < 0 || p.config.AlertThresholdUsage > 1 {
		return fmt.Errorf("alert_threshold_usage must be between 0 and 1")
	}
	return nil
}

// StartupTasks performs startup initialization with retry support
func (p *SQLPlugin) StartupTasks() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connected.Load() {
		return errors.New("already connected")
	}

	log.Infof("Initializing database connection for %s", p.Name())

	// Attempt connection with retry if enabled
	var db *sql.DB
	var err error

	if p.config.RetryEnabled {
		db, err = p.connectWithRetry()
	} else {
		db, err = p.connect()
	}

	if err != nil {
		return err
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

	// Start pool monitor if enabled
	if p.config.MonitorEnabled {
		thresholds := &PoolThresholds{
			UsagePercentage: p.config.AlertThresholdUsage,
			WaitDuration:     time.Duration(p.config.AlertThresholdWait) * time.Second,
			WaitCount:        p.config.AlertThresholdWaitCount,
		}
		p.poolMonitor = NewPoolMonitor(
			p,
			time.Duration(p.config.MonitorInterval)*time.Second,
			thresholds,
		)
		p.poolMonitor.Start(p.ctx)
	}

	log.Infof("Database connection established for %s", p.Name())
	return nil
}

// connect performs a single connection attempt
// This method ensures proper resource cleanup on failure
func (p *SQLPlugin) connect() (*sql.DB, error) {
	// Record connection attempt if not retrying
	if !p.config.RetryEnabled {
		p.metricsRecorder.IncConnectAttempt()
	}

	// Open database connection
	// Note: sql.Open() does not immediately create connections, it just validates the DSN
	db, err := sql.Open(p.config.Driver, p.config.DSN)
	if err != nil {
		if !p.config.RetryEnabled {
			p.metricsRecorder.IncConnectFailure()
		}
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool before testing connection
	db.SetMaxOpenConns(p.config.MaxOpenConns)
	db.SetMaxIdleConns(p.config.MaxIdleConns)

	if p.config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(p.config.ConnMaxLifetime) * time.Second)
	}
	if p.config.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(time.Duration(p.config.ConnMaxIdleTime) * time.Second)
	}

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		// Ensure we close the db on ping failure to prevent resource leaks
		// Even though sql.Open() doesn't create connections immediately,
		// closing ensures any resources are properly released
		closeErr := db.Close()
		if closeErr != nil {
			log.Warnf("Error closing database connection after ping failure: %v", closeErr)
		}
		if !p.config.RetryEnabled {
			p.metricsRecorder.IncConnectFailure()
		}
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if !p.config.RetryEnabled {
		p.metricsRecorder.IncConnectSuccess()
	}
	return db, nil
}

// connectWithRetry attempts connection with exponential backoff retry
func (p *SQLPlugin) connectWithRetry() (*sql.DB, error) {
	var lastErr error
	delay := time.Duration(p.config.RetryInitialDelay) * time.Second

	p.metricsRecorder.IncConnectAttempt()

	for attempt := 0; attempt <= p.config.RetryMaxAttempts; attempt++ {
		// Check if context is cancelled before retrying
		select {
		case <-p.ctx.Done():
			return nil, fmt.Errorf("connection cancelled: %w", p.ctx.Err())
		default:
		}

		if attempt > 0 {
			log.Infof("Retrying database connection for %s (attempt %d/%d) after %v",
				p.Name(), attempt, p.config.RetryMaxAttempts, delay)
			p.metricsRecorder.IncConnectRetry()

			// Use select with context to allow cancellation during sleep
			select {
			case <-p.ctx.Done():
				return nil, fmt.Errorf("connection cancelled during retry: %w", p.ctx.Err())
			case <-time.After(delay):
				// Continue with retry
			}
		}

		db, err := p.connect()
		if err == nil {
			if attempt > 0 {
				log.Infof("Database connection succeeded for %s after %d retries", p.Name(), attempt)
			}
			p.metricsRecorder.IncConnectSuccess()
			return db, nil
		}

		lastErr = err

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * p.config.RetryMultiplier)
		maxDelay := time.Duration(p.config.RetryMaxDelay) * time.Second
		if delay > maxDelay {
			delay = maxDelay
		}
	}

	p.metricsRecorder.IncConnectFailure()
	return nil, fmt.Errorf("failed to connect after %d attempts: %w", p.config.RetryMaxAttempts+1, lastErr)
}

// CleanupTasks performs cleanup on shutdown
func (p *SQLPlugin) CleanupTasks() error {
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

	// Stop pool monitor
	if p.poolMonitor != nil {
		p.poolMonitor.Stop()
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
func (p *SQLPlugin) GetDB() (*sql.DB, error) {
	return p.GetDBWithContext(context.Background())
}

// GetDBWithContext returns the database connection with context support
func (p *SQLPlugin) GetDBWithContext(ctx context.Context) (*sql.DB, error) {
	if !p.IsConnected() {
		return nil, ErrNotConnected
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.db, nil
}

// GetDialect returns the database dialect
// This method is thread-safe
func (p *SQLPlugin) GetDialect() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.dialect
}

// SetMetricsRecorder sets the metrics recorder for this plugin
func (p *SQLPlugin) SetMetricsRecorder(recorder MetricsRecorder) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metricsRecorder = recorder
}

// GetMetricsRecorder returns the current metrics recorder
func (p *SQLPlugin) GetMetricsRecorder() MetricsRecorder {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metricsRecorder
}

// CheckHealth performs a health check
func (p *SQLPlugin) CheckHealth() error {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
func (p *SQLPlugin) IsConnected() bool {
	return p.connected.Load() && !p.closing.Load()
}

// GetStats returns connection pool statistics
// This method is thread-safe and can be called concurrently
func (p *SQLPlugin) GetStats() *ConnectionPoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.connected.Load() || p.closing.Load() || p.db == nil {
		return &ConnectionPoolStats{}
	}

	stats := p.db.Stats()
	maxIdleConns := int64(p.config.MaxIdleConns)
	if maxIdleConns == 0 {
		maxIdleConns = 5 // Default value
	}

	return &ConnectionPoolStats{
		MaxOpenConnections: int64(stats.MaxOpenConnections),
		OpenConnections:    int64(stats.OpenConnections),
		InUse:              int64(stats.InUse),
		Idle:               int64(stats.Idle),
		MaxIdleConnections: maxIdleConns,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration,
		MaxIdleClosed:      stats.MaxIdleClosed,
		MaxLifetimeClosed:  stats.MaxLifetimeClosed,
	}
}

// getDialectFromDriver determines the dialect from the driver name
func (p *SQLPlugin) getDialectFromDriver(driver string) string {
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

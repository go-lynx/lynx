package pgsql

import (
	"context"
	"fmt"
	"strings"
	"time"

	esql "entgo.io/ent/dialect/sql"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/db/pgsql/conf"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Plugin metadata
const (
	// Plugin name
	pluginName = "pgsql.client"
	// Plugin version
	pluginVersion = "v2.0.0"
	// Plugin description
	pluginDescription = "pgsql client plugin for lynx framework"
	// Configuration prefix
	confPrefix = "lynx.pgsql"
	// Default configuration constants
	defaultDriver      = "postgres"
	defaultSource      = "postgres://admin:123456@127.0.0.1:5432/demo?sslmode=disable"
	defaultMinConn     = 10
	defaultMaxConn     = 20
	defaultMaxIdleTime = 10 * time.Second
	defaultMaxLifeTime = 300 * time.Second
	// Health check timeout
	healthCheckTimeout = 5 * time.Second
	// Maximum retry attempts
	maxRetryAttempts = 3
	// Retry interval
	retryInterval = 2 * time.Second
)

// DBPgsqlClient represents a PgSQL client plugin instance
type DBPgsqlClient struct {
	// Inherit base plugin
	*plugins.BasePlugin
	// Database driver
	dri *esql.Driver
	// PgSQL configuration
	conf *conf.Pgsql
	// Connection pool statistics
	stats *ConnectionPoolStats
	// Close signal channel
	closeChan chan struct{}
	// Whether it's closed
	closed bool
	// Prometheus monitoring metrics
	prometheusMetrics *PrometheusMetrics
}

// ConnectionPoolStats connection pool statistics
type ConnectionPoolStats struct {
	MaxOpenConnections int
	OpenConnections    int
	InUse              int
	Idle               int
	MaxIdleConnections int
	WaitCount          int64
	WaitDuration       time.Duration
	MaxIdleClosed      int64
	MaxLifetimeClosed  int64
}

// NewPgsqlClient creates a new PgSQL client plugin instance
// Returns a pointer to the DBPgsqlClient struct
func NewPgsqlClient() *DBPgsqlClient {
	return &DBPgsqlClient{
		BasePlugin: plugins.NewBasePlugin(
			// Generate plugin ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			pluginName,
			// Plugin description
			pluginDescription,
			// Plugin version
			pluginVersion,
			// Configuration prefix
			confPrefix,
			// Weight
			101,
		),
		closeChan: make(chan struct{}),
		stats:     &ConnectionPoolStats{},
	}
}

// validateConfig validates the validity of configuration parameters
func (p *DBPgsqlClient) validateConfig() error {
	if p.conf == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Validate connection string format
	if p.conf.Source != "" {
		if !strings.Contains(p.conf.Source, "://") {
			return fmt.Errorf("invalid connection string format, expected format: postgres://user:password@host:port/dbname")
		}
	}

	// Validate connection pool configuration
	if p.conf.MinConn < 0 {
		return fmt.Errorf("min_conn cannot be negative")
	}
	if p.conf.MaxConn <= 0 {
		return fmt.Errorf("max_conn must be positive")
	}
	if p.conf.MinConn > p.conf.MaxConn {
		return fmt.Errorf("min_conn (%d) cannot be greater than max_conn (%d)", p.conf.MinConn, p.conf.MaxConn)
	}

	// Validate time configuration
	if p.conf.MaxIdleTime != nil {
		if p.conf.MaxIdleTime.AsDuration() < 0 {
			return fmt.Errorf("max_idle_time cannot be negative")
		}
	}
	if p.conf.MaxLifeTime != nil {
		if p.conf.MaxLifeTime.AsDuration() < 0 {
			return fmt.Errorf("max_life_time cannot be negative")
		}
	}

	return nil
}

// setDefaultConfig sets default configuration
func (p *DBPgsqlClient) setDefaultConfig() {
	if p.conf.Driver == "" {
		p.conf.Driver = defaultDriver
	}
	if p.conf.Source == "" {
		p.conf.Source = defaultSource
	}
	if p.conf.MinConn == 0 {
		p.conf.MinConn = defaultMinConn
	}
	if p.conf.MaxConn == 0 {
		p.conf.MaxConn = defaultMaxConn
	}
	if p.conf.MaxIdleTime == nil {
		p.conf.MaxIdleTime = durationpb.New(defaultMaxIdleTime)
	}
	if p.conf.MaxLifeTime == nil {
		p.conf.MaxLifeTime = durationpb.New(defaultMaxLifeTime)
	}

	// Prometheus monitoring configuration can be set via environment variables or configuration files
	// Use default configuration for now
}

// InitializeResources scans and loads PgSQL configuration from runtime configuration
// Parameter rt is the runtime environment
// Returns error information, returns corresponding error if configuration loading fails
func (p *DBPgsqlClient) InitializeResources(rt plugins.Runtime) error {
	// Initialize an empty configuration structure
	p.conf = &conf.Pgsql{}

	// Scan and load PgSQL configuration from runtime configuration
	err := rt.GetConfig().Value(confPrefix).Scan(p.conf)
	if err != nil {
		log.Errorf("failed to scan pgsql configuration: %v", err)
		return fmt.Errorf("failed to load pgsql configuration: %w", err)
	}

	// Set default configuration
	p.setDefaultConfig()

	// Validate configuration
	if err := p.validateConfig(); err != nil {
		log.Errorf("invalid pgsql configuration: %v", err)
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	log.Infof("pgsql configuration loaded successfully: driver=%s, min_conn=%d, max_conn=%d",
		p.conf.Driver, p.conf.MinConn, p.conf.MaxConn)
	return nil
}

// connectWithRetry database connection with retry mechanism
func (p *DBPgsqlClient) connectWithRetry() (*esql.Driver, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetryAttempts; attempt++ {
		log.Infof("attempting to connect to database (attempt %d/%d)", attempt, maxRetryAttempts)

		// Record connection attempt/retry metrics
		if p.prometheusMetrics != nil {
			p.prometheusMetrics.IncConnectAttempt(p.conf)
			if attempt > 1 {
				p.prometheusMetrics.IncConnectRetry(p.conf)
			}
		}

		// Register database driver (based on build tags, hooks wrapper may be enabled)
		registerDriver()

		// Open database connection
		drv, err := esql.Open(p.conf.Driver, p.conf.Source)
		if err != nil {
			lastErr = err
			log.Warnf("connection attempt %d failed: %v", attempt, err)

			if attempt < maxRetryAttempts {
				log.Infof("retrying in %v...", retryInterval)
				time.Sleep(retryInterval)
				continue
			}
			// Final failure, record failure metrics
			if p.prometheusMetrics != nil {
				p.prometheusMetrics.IncConnectFailure(p.conf)
			}
			return nil, fmt.Errorf("failed to connect after %d attempts: %w", maxRetryAttempts, lastErr)
		}

		// Configure connection pool
		db := drv.DB()
		db.SetMaxIdleConns(int(p.conf.MinConn))
		db.SetMaxOpenConns(int(p.conf.MaxConn))
		db.SetConnMaxIdleTime(p.conf.MaxIdleTime.AsDuration())
		db.SetConnMaxLifetime(p.conf.MaxLifeTime.AsDuration())

		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			err := drv.Close()
			if err != nil {
				return nil, err
			}
			lastErr = err
			log.Warnf("connection test failed on attempt %d: %v", attempt, err)

			if attempt < maxRetryAttempts {
				log.Infof("retrying in %v...", retryInterval)
				time.Sleep(retryInterval)
				continue
			}
			// Final failure, record failure metrics
			if p.prometheusMetrics != nil {
				p.prometheusMetrics.IncConnectFailure(p.conf)
			}
			return nil, fmt.Errorf("connection test failed after %d attempts: %w", maxRetryAttempts, lastErr)
		}

		log.Infof("database connection established successfully on attempt %d", attempt)
		if p.prometheusMetrics != nil {
			p.prometheusMetrics.IncConnectSuccess(p.conf)
		}
		return drv, nil
	}

	// Theoretically won't reach here (already returned in the loop), fallback failure metrics
	if p.prometheusMetrics != nil {
		p.prometheusMetrics.IncConnectFailure(p.conf)
	}
	return nil, fmt.Errorf("failed to establish database connection: %w", lastErr)
}

// initPrometheusMetrics initializes Prometheus monitoring metrics
func (p *DBPgsqlClient) initPrometheusMetrics() {
	// Create Prometheus configuration
	promConfig := createPrometheusConfig(p.conf)

	// Create Prometheus metrics
	p.prometheusMetrics = NewPrometheusMetrics(promConfig)
	// Assign global metrics/configuration pointer for optional hooks or other modules
	globalPgsqlMetrics = p.prometheusMetrics
	globalPgsqlConf = p.conf
}

// MetricsGatherer returns the Prometheus Gatherer for this plugin (for unified registration aggregation)
func (p *DBPgsqlClient) MetricsGatherer() prometheus.Gatherer {
	if p == nil || p.prometheusMetrics == nil {
		return nil
	}
	return p.prometheusMetrics.GetGatherer()
}

// updateStats updates connection pool statistics
func (p *DBPgsqlClient) updateStats() {
	if p.dri == nil {
		return
	}

	db := p.dri.DB()
	stats := db.Stats()
	p.stats.MaxOpenConnections = stats.MaxOpenConnections
	p.stats.OpenConnections = stats.OpenConnections
	p.stats.InUse = stats.InUse
	p.stats.Idle = stats.Idle
	p.stats.MaxIdleConnections = int(p.conf.MinConn) // Use configured minimum connections
	p.stats.WaitCount = stats.WaitCount
	p.stats.WaitDuration = stats.WaitDuration
	p.stats.MaxIdleClosed = stats.MaxIdleClosed
	p.stats.MaxLifetimeClosed = stats.MaxLifetimeClosed

	// Update Prometheus metrics
	if p.prometheusMetrics != nil {
		p.prometheusMetrics.UpdateMetrics(p.stats, p.conf)
	}
}

// StartupTasks initializes database connection and performs health check
// Returns error information, returns corresponding error if connection or health check fails
func (p *DBPgsqlClient) StartupTasks() error {
	log.Infof("initializing pgsql database connection")

	// Initialize Prometheus monitoring first (ensure connection phase metrics can be recorded)
	p.initPrometheusMetrics()

	// Connect to database with retry mechanism
	drv, err := p.connectWithRetry()
	if err != nil {
		log.Errorf("failed to initialize database connection: %v", err)
		return fmt.Errorf("database initialization failed: %w", err)
	}

	// Assign database driver to instance
	p.dri = drv

	// Update statistics
	p.updateStats()

	log.Infof("pgsql database successfully initialized with connection pool: max_open=%d, max_idle=%d",
		p.conf.MaxConn, p.conf.MinConn)
	return nil
}

// CleanupTasks gracefully closes database connection
// Returns error information, returns corresponding error if closing connection fails
func (p *DBPgsqlClient) CleanupTasks() error {
	if p.dri == nil || p.closed {
		return nil
	}

	log.Infof("closing pgsql database connection")

	// Mark as closed
	p.closed = true
	close(p.closeChan)

	// Gracefully close connection
	if err := p.dri.Close(); err != nil {
		log.Errorf("failed to close database connection: %v", err)
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	log.Infof("pgsql database connection closed successfully")
	return nil
}

// Configure updates PgSQL configuration
// This function receives a parameter of any type, attempts to convert it to *conf.Pgsql type, and updates configuration if conversion succeeds
func (p *DBPgsqlClient) Configure(c any) error {
	// Try to convert the incoming configuration to *conf.Pgsql type
	if pgsqlConf, ok := c.(*conf.Pgsql); ok {
		// Save old configuration for rollback
		oldConf := p.conf
		p.conf = pgsqlConf

		// Set default configuration
		p.setDefaultConfig()

		// Validate new configuration
		if err := p.validateConfig(); err != nil {
			// Configuration invalid, rollback to old configuration
			p.conf = oldConf
			log.Errorf("invalid new configuration, rolling back: %v", err)
			return fmt.Errorf("configuration validation failed: %w", err)
		}

		log.Infof("pgsql configuration updated successfully")
		return nil
	}

	// Conversion failed, return invalid configuration error
	return plugins.ErrInvalidConfiguration
}

// CheckHealth performs comprehensive health check on database connection
// This function checks connection pool status and database connection health
func (p *DBPgsqlClient) CheckHealth() error {
	if p.dri == nil {
		return fmt.Errorf("database driver is not initialized")
	}

	// Update statistics
	p.updateStats()

	// Check connection pool status
	if p.stats.OpenConnections >= p.stats.MaxOpenConnections {
		log.Warnf("connection pool is at maximum capacity: %d/%d",
			p.stats.OpenConnections, p.stats.MaxOpenConnections)
	}

	// Check connection waiting situation
	if p.stats.WaitCount > 0 {
		log.Warnf("connection pool has wait count: %d, total wait duration: %v",
			p.stats.WaitCount, p.stats.WaitDuration)
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()

	// Perform database connection health check
	err := p.dri.DB().PingContext(ctx)

	// Record Prometheus health check metrics
	if p.prometheusMetrics != nil {
		p.prometheusMetrics.RecordHealthCheck(err == nil, p.conf)
	}

	if err != nil {
		log.Errorf("database health check failed: %v", err)
		return fmt.Errorf("database health check failed: %w", err)
	}

	log.Debugf("database health check passed, pool stats: open=%d, in_use=%d, idle=%d",
		p.stats.OpenConnections, p.stats.InUse, p.stats.Idle)
	return nil
}

// GetStats gets connection pool statistics
func (p *DBPgsqlClient) GetStats() *ConnectionPoolStats {
	if p.dri == nil {
		return nil
	}
	p.updateStats()
	return p.stats
}

// GetConfig gets current configuration
func (p *DBPgsqlClient) GetConfig() *conf.Pgsql {
	return p.conf
}

// IsConnected checks if connected
func (p *DBPgsqlClient) IsConnected() bool {
	return p.dri != nil && !p.closed
}

// GetDriver gets database driver
func (p *DBPgsqlClient) GetDriver() *esql.Driver {
	return p.dri
}

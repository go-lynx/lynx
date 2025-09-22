package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/nosql/mongodb/conf"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Initialize initializes the MongoDB plugin
func (p *PlugMongoDB) Initialize(plugin plugins.Plugin, rt plugins.Runtime) error {
	err := p.BasePlugin.Initialize(plugin, rt)
	if err != nil {
		log.Error(err)
		return err
	}

	// Get configuration from runtime
	cfg := rt.GetConfig()
	if cfg == nil {
		return fmt.Errorf("failed to get config from runtime")
	}

	// Parse configuration
	if err := p.parseConfig(cfg); err != nil {
		return fmt.Errorf("failed to parse mongodb config: %w", err)
	}

	// Create MongoDB client
	if err := p.createClient(); err != nil {
		return fmt.Errorf("failed to create mongodb client: %w", err)
	}

	// Start metrics collection
	if p.conf.EnableMetrics {
		p.startMetricsCollection()
	}

	// Start health check
	if p.conf.EnableHealthCheck {
		p.startHealthCheck()
	}

	log.Info("mongodb plugin initialized successfully")
	return nil
}

// Start starts the MongoDB plugin
func (p *PlugMongoDB) Start(plugin plugins.Plugin) error {
	err := p.BasePlugin.Start(plugin)
	if err != nil {
		log.Error(err)
		return err
	}

	// Test connection
	if err := p.testConnection(); err != nil {
		return fmt.Errorf("failed to test mongodb connection: %w", err)
	}

	// Ensure shared quit channel is initialized when any background task is enabled
	if p.statsQuit == nil && (p.conf.EnableMetrics || p.conf.EnableHealthCheck) {
		p.statsQuit = make(chan struct{})
	}

	log.Info("mongodb plugin started successfully")
	return nil
}

// Stop stops the MongoDB plugin
func (p *PlugMongoDB) Stop(plugin plugins.Plugin) error {
	err := p.BasePlugin.Stop(plugin)
	if err != nil {
		log.Error(err)
		return err
	}

	// Stop metrics collection
	if p.conf.EnableMetrics {
		p.stopMetricsCollection()
	}

	// Stop health check
	if p.conf.EnableHealthCheck {
		p.stopHealthCheck()
	}

	// Close client connection with timeout to avoid blocking shutdown
	if p.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.client.Disconnect(ctx); err != nil {
			log.Errorf("failed to disconnect mongodb client: %v", err)
		}
	}

	log.Info("mongodb plugin stopped successfully")
	return nil
}

// CleanupTasks implements the plugin cleanup interface
func (p *PlugMongoDB) CleanupTasks() error {
	return p.CleanupTasksContext(context.Background())
}

// CleanupTasksContext implements context-aware cleanup with proper timeout handling
func (p *PlugMongoDB) CleanupTasksContext(parentCtx context.Context) error {
	log.Info("cleaning up mongodb plugin")

	// Stop metrics collection
	if p.metricsCancel != nil {
		p.metricsCancel()
		p.metricsCancel = nil
	}

	// Stop health check
	if p.healthCancel != nil {
		p.healthCancel()
		p.healthCancel = nil
	}

	// Close client connection with context-aware timeout
	if p.client != nil {
		ctx, cancel := p.createTimeoutContext(parentCtx, 5*time.Second)
		defer cancel()
		if err := p.client.Disconnect(ctx); err != nil {
			log.Errorf("failed to disconnect mongodb client: %v", err)
			return err
		}
	}

	log.Info("mongodb plugin cleaned up successfully")
	return nil
}

// createTimeoutContext creates a context with timeout, respecting parent context deadline
func (p *PlugMongoDB) createTimeoutContext(parentCtx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if deadline, ok := parentCtx.Deadline(); ok {
		// Parent context has deadline, check if it's sooner than our timeout
		if time.Until(deadline) < timeout {
			return parentCtx, func() {} // Use parent context, no-op cancel
		}
	}
	return context.WithTimeout(parentCtx, timeout)
}

// parseConfig parses configuration
func (p *PlugMongoDB) parseConfig(cfg config.Config) error {
	// Read mongodb configuration from config
	var mongodbConf conf.MongoDB
	if err := cfg.Scan(&mongodbConf); err != nil {
		return err
	}
	p.conf = &mongodbConf

	// Set default values
	if p.conf.Uri == "" {
		p.conf.Uri = "mongodb://localhost:27017"
	}
	if p.conf.Database == "" {
		p.conf.Database = "test"
	}
	if p.conf.MaxPoolSize == 0 {
		p.conf.MaxPoolSize = 100
	}
	if p.conf.MinPoolSize == 0 {
		p.conf.MinPoolSize = 5
	}
	if p.conf.ConnectTimeout == nil {
		p.conf.ConnectTimeout = durationpb.New(30 * time.Second)
	}
	if p.conf.ServerSelectionTimeout == nil {
		p.conf.ServerSelectionTimeout = durationpb.New(30 * time.Second)
	}
	if p.conf.SocketTimeout == nil {
		p.conf.SocketTimeout = durationpb.New(30 * time.Second)
	}
	if p.conf.HeartbeatInterval == nil {
		p.conf.HeartbeatInterval = durationpb.New(10 * time.Second)
	}
	if p.conf.HealthCheckInterval == nil {
		p.conf.HealthCheckInterval = durationpb.New(30 * time.Second)
	}
	if p.conf.ReadConcernLevel == "" {
		p.conf.ReadConcernLevel = "local"
	}
	if p.conf.WriteConcernW == 0 {
		p.conf.WriteConcernW = 1
	}
	if p.conf.WriteConcernTimeout == nil {
		p.conf.WriteConcernTimeout = durationpb.New(5 * time.Second)
	}

	return nil
}

// createClient creates the MongoDB client
func (p *PlugMongoDB) createClient() error {
	// Parse timeout values
	connectTimeout := p.conf.ConnectTimeout.AsDuration()
	serverSelectionTimeout := p.conf.ServerSelectionTimeout.AsDuration()
	socketTimeout := p.conf.SocketTimeout.AsDuration()
	heartbeatInterval := p.conf.HeartbeatInterval.AsDuration()

	// Build client options
	clientOptions := options.Client().ApplyURI(p.conf.Uri)

	// Set connection pool configuration
	clientOptions.SetMaxPoolSize(p.conf.MaxPoolSize)
	clientOptions.SetMinPoolSize(p.conf.MinPoolSize)

	// Set timeout configuration
	clientOptions.SetConnectTimeout(connectTimeout)
	clientOptions.SetServerSelectionTimeout(serverSelectionTimeout)
	clientOptions.SetSocketTimeout(socketTimeout)
	clientOptions.SetHeartbeatInterval(heartbeatInterval)

	// Set authentication information
	if p.conf.Username != "" && p.conf.Password != "" {
		clientOptions.SetAuth(options.Credential{
			Username:   p.conf.Username,
			Password:   p.conf.Password,
			AuthSource: p.conf.AuthSource,
		})
	}

	// Set TLS configuration
	if p.conf.EnableTls {
		tlsOpts := make(map[string]interface{})
		if p.conf.TlsCertFile != "" {
			tlsOpts["certFile"] = p.conf.TlsCertFile
		}
		if p.conf.TlsKeyFile != "" {
			tlsOpts["keyFile"] = p.conf.TlsKeyFile
		}
		if p.conf.TlsCaFile != "" {
			tlsOpts["caFile"] = p.conf.TlsCaFile
		}
		if len(tlsOpts) > 0 {
			tlsConfig, err := options.BuildTLSConfig(tlsOpts)
			if err != nil {
				return fmt.Errorf("failed to build TLS config: %w", err)
			}
			clientOptions.SetTLSConfig(tlsConfig)
		}
	}

	// Set compression configuration
	if p.conf.EnableCompression {
		clientOptions.SetCompressors([]string{"zlib", "snappy"})
	}

	// Set retry writes
	if p.conf.EnableRetryWrites {
		clientOptions.SetRetryWrites(true)
	}

	// Set read concern
	if p.conf.EnableReadConcern {
		var rc *readconcern.ReadConcern
		switch p.conf.ReadConcernLevel {
		case "local":
			rc = readconcern.Local()
		case "majority":
			rc = readconcern.Majority()
		case "linearizable":
			rc = readconcern.Linearizable()
		case "snapshot":
			rc = readconcern.Snapshot()
		default:
			rc = readconcern.Local()
		}
		clientOptions.SetReadConcern(rc)
	}

	// Set write concern
	if p.conf.EnableWriteConcern {
		writeConcernTimeout := p.conf.WriteConcernTimeout.AsDuration()

		wc := writeconcern.New(
			writeconcern.W(int(p.conf.WriteConcernW)),
			writeconcern.WTimeout(writeConcernTimeout),
		)
		clientOptions.SetWriteConcern(wc)
	}

	// Create client with timeout to avoid startup hang
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return err
	}

	p.client = client

	// Get database instance
	p.database = client.Database(p.conf.Database)

	return nil
}

// testConnection tests the connection
func (p *PlugMongoDB) testConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send ping request
	if err := p.client.Ping(ctx, nil); err != nil {
		return err
	}

	return nil
}

// startMetricsCollection starts metrics collection
func (p *PlugMongoDB) startMetricsCollection() {
	// Use health check interval for metrics collection or default to 30 seconds
	var interval time.Duration
	if p.conf.HealthCheckInterval != nil {
		interval = p.conf.HealthCheckInterval.AsDuration()
	} else {
		interval = 30 * time.Second
	}

	// Ensure quit channel exists
	if p.statsQuit == nil {
		p.statsQuit = make(chan struct{})
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.metricsCancel = cancel

	p.statsWG.Add(1)
	go func() {
		defer p.statsWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				p.collectMetrics()
			case <-ctx.Done():
				return
			case <-p.statsQuit:
				return
			}
		}
	}()
}

// stopMetricsCollection stops metrics collection
func (p *PlugMongoDB) stopMetricsCollection() {
	if p.statsQuit != nil {
		p.closeStatsQuitOnce()
		p.statsWG.Wait()
	}
}

// collectMetrics collects metrics
func (p *PlugMongoDB) collectMetrics() {
	// Here you can collect MongoDB metrics
	// For example: connection pool status, operation statistics, performance metrics, etc.
	ctx, cancel := p.createTimeoutContext(context.Background(), 5*time.Second)
	defer cancel()

	// Get database statistics
	stats := p.database.RunCommand(ctx, map[string]interface{}{
		"dbStats": 1,
	})
	if stats.Err() != nil {
		log.Errorf("failed to get database stats: %v", stats.Err())
		return
	}

	// You can send metrics to the monitoring system here
	log.Debug("mongodb metrics collected")
}

// startHealthCheck starts health check
func (p *PlugMongoDB) startHealthCheck() {
	interval := p.conf.HealthCheckInterval.AsDuration()

	// Ensure quit channel exists
	if p.statsQuit == nil {
		p.statsQuit = make(chan struct{})
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.healthCancel = cancel

	p.statsWG.Add(1)
	go func() {
		defer p.statsWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := p.checkHealth(); err != nil {
					log.Errorf("mongodb health check failed: %v", err)
				}
			case <-ctx.Done():
				return
			case <-p.statsQuit:
				return
			}
		}
	}()
}

// stopHealthCheck stops health check
func (p *PlugMongoDB) stopHealthCheck() {
	if p.statsQuit != nil {
		p.closeStatsQuitOnce()
		// Wait for all goroutines (metrics + health) to exit, set timeout to avoid infinite wait
		done := make(chan struct{})
		go func() {
			p.statsWG.Wait()
			close(done)
		}()
		select {
		case <-done:
			log.Infof("mongodb background tasks stopped successfully")
		case <-time.After(10 * time.Second):
			log.Warnf("timeout waiting for mongodb background tasks to stop")
		}
	}
}

// closeStatsQuitOnce closes statsQuit only once in a thread-safe way
func (p *PlugMongoDB) closeStatsQuitOnce() {
	p.statsMu.Lock()
	defer p.statsMu.Unlock()
	if !p.statsClosed && p.statsQuit != nil {
		close(p.statsQuit)
		p.statsClosed = true
	}
}

// checkHealth executes health check
func (p *PlugMongoDB) checkHealth() error {
	ctx, cancel := p.createTimeoutContext(context.Background(), 5*time.Second)
	defer cancel()

	// Send ping request
	if err := p.client.Ping(ctx, nil); err != nil {
		return err
	}

	return nil
}

// GetClient gets the MongoDB client
func (p *PlugMongoDB) GetClient() *mongo.Client {
	return p.client
}

// GetDatabase gets the MongoDB database instance
func (p *PlugMongoDB) GetDatabase() *mongo.Database {
	return p.database
}

// GetCollection gets the collection instance
func (p *PlugMongoDB) GetCollection(collectionName string) *mongo.Collection {
	if p.database == nil {
		return nil
	}
	return p.database.Collection(collectionName)
}

// GetConnectionStats gets connection statistics
func (p *PlugMongoDB) GetConnectionStats() map[string]any {
	stats := make(map[string]any)

	if p.client != nil {
		// Get client statistics
		stats["client_initialized"] = true
		stats["database"] = p.conf.Database
		stats["max_pool_size"] = p.conf.MaxPoolSize
		stats["min_pool_size"] = p.conf.MinPoolSize
		stats["compression_enabled"] = p.conf.EnableCompression
		stats["tls_enabled"] = p.conf.EnableTls
	} else {
		stats["client_initialized"] = false
	}

	return stats
}

package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/nosql/mongodb/conf"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// Initialize initializes the MongoDB plugin
func (p *PlugMongoDB) Initialize(plugin plugins.Plugin, rt plugins.Runtime) error {
	p.BasePlugin.Initialize(plugin, rt)

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

	p.logger.Info("mongodb plugin initialized successfully")
	return nil
}

// Start starts the MongoDB plugin
func (p *PlugMongoDB) Start(plugin plugins.Plugin) error {
	p.BasePlugin.Start(plugin)

	// Test connection
	if err := p.testConnection(); err != nil {
		return fmt.Errorf("failed to test mongodb connection: %w", err)
	}

	p.logger.Info("mongodb plugin started successfully")
	return nil
}

// Stop stops the MongoDB plugin
func (p *PlugMongoDB) Stop(plugin plugins.Plugin) error {
	p.BasePlugin.Stop(plugin)

	// Stop metrics collection
	if p.conf.EnableMetrics {
		p.stopMetricsCollection()
	}

	// Stop health check
	if p.conf.EnableHealthCheck {
		p.stopHealthCheck()
	}

	// Close client connection
	if p.client != nil {
		if err := p.client.Disconnect(context.Background()); err != nil {
			p.logger.Errorf("failed to disconnect mongodb client: %v", err)
		}
	}

	p.logger.Info("mongodb plugin stopped successfully")
	return nil
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
	if p.conf.URI == "" {
		p.conf.URI = "mongodb://localhost:27017"
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
	if p.conf.ConnectTimeout == "" {
		p.conf.ConnectTimeout = "30s"
	}
	if p.conf.ServerSelectionTimeout == "" {
		p.conf.ServerSelectionTimeout = "30s"
	}
	if p.conf.SocketTimeout == "" {
		p.conf.SocketTimeout = "30s"
	}
	if p.conf.HeartbeatInterval == "" {
		p.conf.HeartbeatInterval = "10s"
	}
	if p.conf.HealthCheckInterval == "" {
		p.conf.HealthCheckInterval = "30s"
	}
	if p.conf.ReadConcernLevel == "" {
		p.conf.ReadConcernLevel = "local"
	}
	if p.conf.WriteConcernW == 0 {
		p.conf.WriteConcernW = 1
	}
	if p.conf.WriteConcernTimeout == "" {
		p.conf.WriteConcernTimeout = "5s"
	}

	return nil
}

// createClient creates the MongoDB client
func (p *PlugMongoDB) createClient() error {
	// Parse timeout values
	connectTimeout, err := time.ParseDuration(p.conf.ConnectTimeout)
	if err != nil {
		return fmt.Errorf("invalid connect timeout: %w", err)
	}

	serverSelectionTimeout, err := time.ParseDuration(p.conf.ServerSelectionTimeout)
	if err != nil {
		return fmt.Errorf("invalid server selection timeout: %w", err)
	}

	socketTimeout, err := time.ParseDuration(p.conf.SocketTimeout)
	if err != nil {
		return fmt.Errorf("invalid socket timeout: %w", err)
	}

	heartbeatInterval, err := time.ParseDuration(p.conf.HeartbeatInterval)
	if err != nil {
		return fmt.Errorf("invalid heartbeat interval: %w", err)
	}

	// Build client options
	clientOptions := options.Client().ApplyURI(p.conf.URI)

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
	if p.conf.EnableTLS {
		tlsConfig := options.TLS()
		if p.conf.TLSCertFile != "" {
			tlsConfig.SetClientCertificateKeyFile(p.conf.TLSCertFile)
		}
		if p.conf.TLSKeyFile != "" {
			tlsConfig.SetClientCertificateKeyFile(p.conf.TLSKeyFile)
		}
		if p.conf.TLSCAFile != "" {
			tlsConfig.SetCAFile(p.conf.TLSCAFile)
		}
		clientOptions.SetTLSConfig(tlsConfig)
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
		var rc readconcern.ReadConcern
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
		writeConcernTimeout, err := time.ParseDuration(p.conf.WriteConcernTimeout)
		if err != nil {
			return fmt.Errorf("invalid write concern timeout: %w", err)
		}

		wc := writeconcern.New(
			writeconcern.W(p.conf.WriteConcernW),
			writeconcern.WTimeout(writeConcernTimeout),
		)
		clientOptions.SetWriteConcern(wc)
	}

	// Create client
	client, err := mongo.Connect(context.Background(), clientOptions)
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
	p.statsQuit = make(chan struct{})
	p.statsWG.Add(1)

	go func() {
		defer p.statsWG.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				p.collectMetrics()
			case <-p.statsQuit:
				return
			}
		}
	}()
}

// stopMetricsCollection stops metrics collection
func (p *PlugMongoDB) stopMetricsCollection() {
	if p.statsQuit != nil {
		close(p.statsQuit)
		p.statsWG.Wait()
	}
}

// collectMetrics collects metrics
func (p *PlugMongoDB) collectMetrics() {
	// Here you can collect MongoDB metrics
	// For example: connection pool status, operation statistics, performance metrics, etc.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get database statistics
	stats := p.database.RunCommand(ctx, map[string]interface{}{
		"dbStats": 1,
	})
	if stats.Err() != nil {
		p.logger.Errorf("failed to get database stats: %v", stats.Err())
		return
	}

	// You can send metrics to the monitoring system here
	p.logger.Debug("mongodb metrics collected")
}

// startHealthCheck starts health check
func (p *PlugMongoDB) startHealthCheck() {
	interval, err := time.ParseDuration(p.conf.HealthCheckInterval)
	if err != nil {
		p.logger.Errorf("invalid health check interval: %v", err)
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := p.checkHealth(); err != nil {
					p.logger.Errorf("mongodb health check failed: %v", err)
				}
			case <-p.statsQuit:
				return
			}
		}
	}()
}

// stopHealthCheck stops health check
func (p *PlugMongoDB) stopHealthCheck() {
	// Health check uses the same quit channel
}

// checkHealth executes health check
func (p *PlugMongoDB) checkHealth() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
		stats["tls_enabled"] = p.conf.EnableTLS
	} else {
		stats["client_initialized"] = false
	}

	return stats
}

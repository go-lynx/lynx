package grpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/service/grpc/conf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/durationpb"
)

// ClientPlugin represents the gRPC client plugin implementation
type ClientPlugin struct {
	*plugins.BasePlugin
	conf        *conf.GrpcClient
	connections map[string]*grpc.ClientConn
	mu          sync.RWMutex
	discovery   registry.Discovery
	metrics     *ClientMetrics
	retryHandler *RetryHandler
}

// ClientConfig represents configuration for a specific gRPC client connection
type ClientConfig struct {
	ServiceName    string
	Endpoint       string
	Discovery      registry.Discovery
	TLS            bool
	TLSAuthType    int32
	Timeout        time.Duration
	KeepAlive      time.Duration
	MaxRetries     int
	RetryBackoff   time.Duration
	MaxConnections int
	Middleware     []middleware.Middleware
	NodeFilter     selector.NodeFilter
}

// NewGrpcClientPlugin creates a new gRPC client plugin instance
func NewGrpcClientPlugin() *ClientPlugin {
	return &ClientPlugin{
		BasePlugin: plugins.NewBasePlugin(
			plugins.GeneratePluginID("", "grpc.client", "v2.0.0"),
			"grpc.client",
			"gRPC client plugin for Lynx framework",
			"v2.0.0",
			"lynx.grpc.client",
			20, // Higher weight than server plugin
		),
		connections:  make(map[string]*gGrpc.ClientConn),
		metrics:      NewClientMetrics(),
		retryHandler: NewRetryHandler(),
	}
}

// InitializeResources initializes the gRPC client plugin
func (c *ClientPlugin) InitializeResources(rt plugins.Runtime) error {
	c.conf = &conf.GrpcClient{}

	// Load configuration
	err := rt.GetConfig().Value("lynx.grpc.client").Scan(c.conf)
	if err != nil {
		return err
	}

	// Set default configuration
	if c.conf.DefaultTimeout == nil {
		c.conf.DefaultTimeout = &durationpb.Duration{Seconds: 10}
	}
	if c.conf.DefaultKeepAlive == nil {
		c.conf.DefaultKeepAlive = &durationpb.Duration{Seconds: 30}
	}
	if c.conf.MaxRetries == 0 {
		c.conf.MaxRetries = 3
	}
	if c.conf.RetryBackoff == nil {
		c.conf.RetryBackoff = &durationpb.Duration{Seconds: 1}
	}
	if c.conf.MaxConnections == 0 {
		c.conf.MaxConnections = 10
	}

	// Get discovery from control plane
	// Note: This needs to be injected via dependency injection
	// For now, we'll set it to nil and handle it later
	c.discovery = nil

	return nil
}

// StartupTasks starts the gRPC client plugin
func (c *ClientPlugin) StartupTasks() error {
	log.Infof("Starting gRPC client plugin")
	
	// Initialize metrics
	c.metrics.Initialize()
	
	// Initialize retry handler
	c.retryHandler.Initialize(c.conf.MaxRetries, c.conf.RetryBackoff.AsDuration())
	
	log.Infof("gRPC client plugin started successfully")
	return nil
}

// CleanupTasks cleans up gRPC client resources
func (c *ClientPlugin) CleanupTasks() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	log.Infof("Cleaning up gRPC client connections")
	
	// Close all connections
	for serviceName, conn := range c.connections {
		if conn != nil {
			if err := conn.Close(); err != nil {
				log.Errorf("Failed to close gRPC connection for service %s: %v", serviceName, err)
			}
		}
	}
	
	// Clear connections map
	c.connections = make(map[string]*gGrpc.ClientConn)
	
	log.Infof("gRPC client plugin cleanup completed")
	return nil
}

// CheckHealth checks the health of gRPC client connections
func (c *ClientPlugin) CheckHealth() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.connections) == 0 {
		return fmt.Errorf("no gRPC client connections available")
	}

	// Check each connection
	for serviceName, conn := range c.connections {
		if conn == nil {
			return fmt.Errorf("gRPC connection for service %s is nil", serviceName)
		}
		
		state := conn.GetState()
		if state != gGrpc.Ready && state != gGrpc.Idle {
			return fmt.Errorf("gRPC connection for service %s is not ready, state: %s", serviceName, state.String())
		}
	}

	return nil
}

// Configure updates the plugin configuration
func (c *ClientPlugin) Configure(config any) error {
	if config == nil {
		return nil
	}
	
	clientConfig, ok := config.(*conf.GrpcClient)
	if !ok {
		return fmt.Errorf("invalid configuration type for gRPC client plugin")
	}
	
	c.conf = clientConfig
	return nil
}

// GetConnection returns a gRPC client connection for the specified service
func (c *ClientPlugin) GetConnection(serviceName string) (*grpc.ClientConn, error) {
	c.mu.RLock()
	conn, exists := c.connections[serviceName]
	c.mu.RUnlock()

	if exists && conn != nil {
		// Check if connection is still healthy
		state := conn.GetState()
		if state == grpc.Ready || state == grpc.Idle {
			return conn, nil
		}
		// Connection is not healthy, remove it
		c.mu.Lock()
		delete(c.connections, serviceName)
		c.mu.Unlock()
	}

	// Create new connection
	return c.createConnection(serviceName)
}

// CreateConnection creates a new gRPC client connection
func (c *ClientPlugin) CreateConnection(config ClientConfig) (*grpc.ClientConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if connection already exists
	if conn, exists := c.connections[config.ServiceName]; exists && conn != nil {
		state := conn.GetState()
		if state == grpc.Ready || state == grpc.Idle {
			return conn, nil
		}
		// Remove unhealthy connection
		delete(c.connections, config.ServiceName)
	}

	// Create new connection
	conn, err := c.buildConnection(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection for service %s: %w", config.ServiceName, err)
	}

	// Store connection
	c.connections[config.ServiceName] = conn
	
	// Record metrics
	c.metrics.RecordConnectionCreated(config.ServiceName)
	
	log.Infof("Created gRPC connection for service: %s", config.ServiceName)
	return conn, nil
}

// createConnection creates a connection using default configuration
func (c *ClientPlugin) createConnection(serviceName string) (*grpc.ClientConn, error) {
	config := ClientConfig{
		ServiceName:    serviceName,
		Discovery:      c.discovery,
		TLS:            c.conf.GetTlsEnable(),
		TLSAuthType:    c.conf.GetTlsAuthType(),
		Timeout:        c.conf.DefaultTimeout.AsDuration(),
		KeepAlive:      c.conf.DefaultKeepAlive.AsDuration(),
		MaxRetries:     int(c.conf.MaxRetries),
		RetryBackoff:   c.conf.RetryBackoff.AsDuration(),
		MaxConnections: int(c.conf.MaxConnections),
		Middleware:     c.getDefaultMiddleware(),
	}

	return c.CreateConnection(config)
}

// buildConnection builds a gRPC client connection with the given configuration
func (c *ClientPlugin) buildConnection(config ClientConfig) (*grpc.ClientConn, error) {
	// Build client options
	opts := []kratosgrpc.ClientOption{
		kratosgrpc.WithEndpoint("discovery:///" + config.ServiceName),
		kratosgrpc.WithDiscovery(config.Discovery),
		kratosgrpc.WithMiddleware(config.Middleware...),
		kratosgrpc.WithNodeFilter(config.NodeFilter),
	}

	// Add TLS configuration if enabled
	if config.TLS {
		tlsConfig, err := c.buildTLSConfig(config)
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS config: %w", err)
		}
		opts = append(opts, kratosgrpc.WithTLSConfig(tlsConfig))
	}

	// Add keep-alive configuration
	if config.KeepAlive > 0 {
		opts = append(opts, kratosgrpc.WithOptions(
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                config.KeepAlive,
				Timeout:             config.KeepAlive / 3,
				PermitWithoutStream: true,
			}),
		))
	}

	// Add timeout configuration
	if config.Timeout > 0 {
		opts = append(opts, kratosgrpc.WithTimeout(config.Timeout))
	}

	// Create connection
	var conn *grpc.ClientConn
	var err error

	if config.TLS {
		conn, err = kratosgrpc.Dial(context.Background(), opts...)
	} else {
		conn, err = kratosgrpc.DialInsecure(context.Background(), opts...)
	}

	if err != nil {
		return nil, err
	}

	return conn, nil
}

// buildTLSConfig builds TLS configuration for the client
func (c *ClientPlugin) buildTLSConfig(config ClientConfig) (*credentials.TransportCredentials, error) {
	// This would integrate with the existing TLS certificate management
	// For now, return a basic insecure credentials
	// In a real implementation, this would load certificates from the certificate manager
	return nil, nil
}

// getDefaultMiddleware returns default middleware for gRPC clients
func (c *ClientPlugin) getDefaultMiddleware() []middleware.Middleware {
	return []middleware.Middleware{
		logging.Client(log.Logger),
		tracing.Client(),
		c.getMetricsMiddleware(),
		c.getRetryMiddleware(),
	}
}

// getMetricsMiddleware returns metrics middleware for gRPC clients
func (c *ClientPlugin) getMetricsMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			start := time.Now()
			
			resp, err := handler(ctx, req)
			
			duration := time.Since(start)
			status := "success"
			if err != nil {
				status = "error"
			}
			
			// Record metrics
			c.metrics.RecordRequest(duration, status)
			
			return resp, err
		}
	}
}

// getRetryMiddleware returns retry middleware for gRPC clients
func (c *ClientPlugin) getRetryMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			return c.retryHandler.ExecuteWithRetry(ctx, handler, req)
		}
	}
}

// CloseConnection closes a specific gRPC connection
func (c *ClientPlugin) CloseConnection(serviceName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, exists := c.connections[serviceName]
	if !exists {
		return fmt.Errorf("connection for service %s not found", serviceName)
	}

	if conn != nil {
		if err := conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection for service %s: %w", serviceName, err)
		}
	}

	delete(c.connections, serviceName)
	c.metrics.RecordConnectionClosed(serviceName)
	
	log.Infof("Closed gRPC connection for service: %s", serviceName)
	return nil
}

// GetConnectionCount returns the number of active connections
func (c *ClientPlugin) GetConnectionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.connections)
}

// GetConnectionStatus returns the status of all connections
func (c *ClientPlugin) GetConnectionStatus() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := make(map[string]string)
	for serviceName, conn := range c.connections {
		if conn != nil {
			status[serviceName] = conn.GetState().String()
		} else {
			status[serviceName] = "nil"
		}
	}
	return status
}

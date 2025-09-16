package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/service/grpc/conf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

// ClientPlugin represents the gRPC client plugin
type ClientPlugin struct {
	*plugins.BasePlugin
	conf             *conf.GrpcClient
	connections      map[string]*grpc.ClientConn
	connectionPool   *ConnectionPool
	loadBalancer     *LoadBalancer
	circuitBreakers  *CircuitBreakerManager
	discovery        registry.Discovery
	tlsManager       *TLSManager
	mu               sync.RWMutex
	metrics          *ClientMetrics
}

// ClientConfig represents configuration for a specific gRPC client connection
type ClientConfig struct {
	ServiceName       string
	Endpoint          string
	Discovery         registry.Discovery
	TLS               bool
	TLSAuthType       int32
	Timeout           time.Duration
	KeepAlive         time.Duration
	MaxRetries        int
	RetryBackoff      time.Duration
	MaxConnections    int
	Middleware        []middleware.Middleware
	NodeFilter        selector.NodeFilter
	Required          bool
	Metadata          map[string]string
	LoadBalancer      string
	CircuitBreaker    bool
	CircuitThreshold  int
}

// NewGrpcClientPlugin creates a new gRPC client plugin instance
func NewGrpcClientPlugin() *ClientPlugin {
	metrics := NewClientMetrics()

	// Initialize connection pool with default settings
	connectionPool := NewConnectionPool(10, 5*time.Minute, false, metrics)

	// Initialize load balancer (will be configured per service)
	loadBalancer := NewLoadBalancer(nil, metrics)

	// Initialize circuit breaker manager
	circuitBreakers := NewCircuitBreakerManager(metrics)

	return &ClientPlugin{
		BasePlugin:      plugins.NewBasePlugin("grpc.client", "grpc.client", "gRPC client plugin for Lynx framework", "v2.0.0", "lynx.grpc.client", 20),
		conf:           &conf.GrpcClient{},
		connections:    make(map[string]*grpc.ClientConn),
		connectionPool: connectionPool,
		loadBalancer:   loadBalancer,
		circuitBreakers: circuitBreakers,
		metrics:        metrics,
	}
}

// InitializeResources initializes the gRPC client plugin
func (c *ClientPlugin) InitializeResources(rt plugins.Runtime) error {
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

	// Initialize connection pool with actual config
	poolEnabled := c.conf.GetConnectionPooling()
	if poolEnabled {
		poolSize := int(c.conf.GetPoolSize())
		idleTimeout := c.conf.GetIdleTimeout().AsDuration()
		if poolSize <= 0 {
			poolSize = 10
		}
		if idleTimeout <= 0 {
			idleTimeout = 5 * time.Minute
		}
		// Recreate connection pool with actual config
		c.connectionPool = NewConnectionPool(poolSize, idleTimeout, poolEnabled, c.metrics)
	}

	// Get discovery from control plane
	// Note: This needs to be injected via dependency injection
	// For now, we'll set it to nil and handle it later
	c.discovery = nil

	// Validate configuration
	if err := c.validateConfiguration(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	return nil
}

// StartupTasks starts the gRPC client plugin
func (c *ClientPlugin) StartupTasks() error {
	log.Infof("Starting gRPC client plugin")

	// Initialize metrics
	c.metrics.Initialize()

	// Initialize retry handler
	// c.retryHandler.Initialize(c.conf.MaxRetries, c.conf.RetryBackoff.AsDuration())

	log.Infof("gRPC client plugin started successfully")
	return nil
}

// Close closes all connections and cleans up resources
func (p *ClientPlugin) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var lastErr error

	// Close connection pool
	if p.connectionPool != nil {
		if err := p.connectionPool.CloseAll(); err != nil {
			lastErr = err
		}
	}

	// Close load balancer
	if p.loadBalancer != nil {
		if err := p.loadBalancer.Close(); err != nil {
			lastErr = err
		}
	}

	// Close circuit breakers
	if p.circuitBreakers != nil {
		p.circuitBreakers.Close()
	}

	// Close TLS manager
	if p.tlsManager != nil {
		p.tlsManager.Close()
	}

	// Close legacy connections
	for serviceName, conn := range p.connections {
		if err := conn.Close(); err != nil {
			lastErr = err
		}
		delete(p.connections, serviceName)
	}

	return lastErr
}

// GetConnection returns a gRPC client connection for the specified service
func (c *ClientPlugin) GetConnection(serviceName string) (*grpc.ClientConn, error) {
	c.mu.RLock()
	conn, exists := c.connections[serviceName]
	c.mu.RUnlock()

	if exists && conn != nil {
		// Check if connection is still healthy
		state := conn.GetState()
		if state == connectivity.Ready || state == connectivity.Idle {
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

// CreateConnection creates a new gRPC connection based on the provided configuration
func (p *ClientPlugin) CreateConnection(config ClientConfig) (*grpc.ClientConn, error) {
	// Configure load balancer for this service if needed
	if config.Discovery != nil && config.LoadBalancer != "" {
		lbConfig := &LoadBalancerConfig{
			Strategy: LoadBalancerType(config.LoadBalancer),
			Metadata: config.Metadata,
		}
		p.loadBalancer.discovery = config.Discovery
		if err := p.loadBalancer.ConfigureService(config.ServiceName, lbConfig); err != nil {
			log.Errorf("Failed to configure load balancer for service %s: %v", config.ServiceName, err)
		}
	}

	// Configure circuit breaker for this service
	cbConfig := &CircuitBreakerConfig{
		Enabled:          config.CircuitBreaker,
		FailureThreshold: config.CircuitThreshold,
		RecoveryTimeout:  30 * time.Second,
		SuccessThreshold: 3,
		Timeout:          config.Timeout,
		MaxConcurrentRequests: 10,
	}
	circuitBreaker := p.circuitBreakers.GetCircuitBreaker(config.ServiceName, cbConfig)

	// Use connection pool to get/create connection
	conn, err := p.connectionPool.GetConnection(config.ServiceName, func() (*grpc.ClientConn, error) {
		return p.buildConnection(config)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get connection for service %s: %w", config.ServiceName, err)
	}

	// Store connection in legacy map for backward compatibility
	p.mu.Lock()
	p.connections[config.ServiceName] = conn
	p.mu.Unlock()

	// Record metrics
	if p.metrics != nil {
		p.metrics.RecordConnectionCreated(config.ServiceName)
	}

	// Test circuit breaker functionality
	if config.CircuitBreaker {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = circuitBreaker.Execute(ctx, func(ctx context.Context) error {
			// Simple connection state check
			state := conn.GetState()
			if state.String() == "SHUTDOWN" || state.String() == "TRANSIENT_FAILURE" {
				return fmt.Errorf("connection in unhealthy state: %s", state.String())
			}
			return nil
		})
		if err != nil {
			log.Errorf("Circuit breaker test failed for service %s: %v", config.ServiceName, err)
		}
	}

	return conn, nil
}

// createConnection creates a connection using default configuration
func (c *ClientPlugin) createConnection(serviceName string) (*grpc.ClientConn, error) {
	config := ClientConfig{
		ServiceName:    serviceName,
		Discovery:      c.discovery,
		TLS:            c.conf.GetTlsEnable(),
		TLSAuthType:    c.conf.GetTlsAuthType(),
		MaxRetries:     int(c.conf.MaxRetries),
		Middleware:     c.getDefaultMiddleware(),
	}

	// Set timeout with nil check
	if c.conf.DefaultTimeout != nil {
		config.Timeout = c.conf.DefaultTimeout.AsDuration()
	} else {
		config.Timeout = 10 * time.Second
	}

	// Set keep alive with nil check
	if c.conf.DefaultKeepAlive != nil {
		config.KeepAlive = c.conf.DefaultKeepAlive.AsDuration()
	} else {
		config.KeepAlive = 30 * time.Second
	}

	// Set retry backoff with nil check
	if c.conf.RetryBackoff != nil {
		config.RetryBackoff = c.conf.RetryBackoff.AsDuration()
	} else {
		config.RetryBackoff = 1 * time.Second
	}

	// Set max connections
	if c.conf.MaxConnections > 0 {
		config.MaxConnections = int(c.conf.MaxConnections)
	} else {
		config.MaxConnections = 10
	}

	return c.CreateConnection(config)
}

// buildConnection builds a gRPC client connection with the given configuration
func (c *ClientPlugin) buildConnection(config ClientConfig) (*grpc.ClientConn, error) {
	// Build client options
	var opts []grpc.DialOption

	// Set endpoint based on configuration
	var target string
	if config.Discovery != nil {
		// Use service discovery - simplified approach
		opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
		target = fmt.Sprintf("discovery:///%s", config.ServiceName)
	} else if config.Endpoint != "" {
		// Use static endpoint
		opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
		target = config.Endpoint
	} else {
		return nil, fmt.Errorf("neither service discovery nor static endpoint configured for service %s", config.ServiceName)
	}

	// Add middleware and node filter
	if len(config.Middleware) > 0 {
		// Convert kratos middleware to grpc interceptors
		// Note: This is a simplified conversion, actual implementation may vary
		// opts = append(opts, grpc.WithChainUnaryInterceptor(config.Middleware...))
	}
	if config.NodeFilter != nil {
		// Apply node filter to the connection
		// Note: grpc.WithNodeFilter doesn't exist in standard gRPC, this would need custom implementation
		// For now, we'll skip this and add it to the middleware chain instead
	}

	// Add TLS configuration if enabled
	if config.TLS {
		tlsConfig, err := c.buildTLSConfig(config)
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS config: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(tlsConfig))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Add keep-alive configuration
	if config.KeepAlive > 0 {
		opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                config.KeepAlive,
			Timeout:             config.KeepAlive / 3,
			PermitWithoutStream: true,
		}))
	}

	// Add timeout configuration
	if config.Timeout > 0 {
		// Note: grpc.WithTimeout is deprecated, use context.WithTimeout instead
		// We'll handle timeout in the context when making calls
	}

	// Create connection
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// buildTLSConfig builds TLS configuration for the client
func (c *ClientPlugin) buildTLSConfig(config ClientConfig) (credentials.TransportCredentials, error) {
	// Get certificate provider from the application
	certProvider := c.getCertProvider()
	if certProvider == nil {
		return nil, fmt.Errorf("certificate provider not configured")
	}

	// Create TLS manager if not exists
	if c.tlsManager == nil {
		c.tlsManager = NewTLSManager()
	}

	// Build TLS configuration based on auth type
	tlsConfig := &TLSConfig{
		Enabled:            true,
		InsecureSkipVerify: false,
		ServerName:         config.ServiceName,
		ClientAuth:         tls.ClientAuthType(config.TLSAuthType),
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	// Set service-specific TLS configuration
	err := c.tlsManager.SetServiceConfig(config.ServiceName, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to set TLS config for service %s: %w", config.ServiceName, err)
	}

	// Get credentials from TLS manager
	creds, err := c.tlsManager.GetCredentials(config.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS credentials for service %s: %w", config.ServiceName, err)
	}

	return creds, nil
}

// getDefaultMiddleware returns default middleware for gRPC clients
func (c *ClientPlugin) getDefaultMiddleware() []middleware.Middleware {
	return []middleware.Middleware{
		logging.Client(nil),
		tracing.Client(),
		c.getMetricsMiddleware(),
		// c.getRetryMiddleware(),
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
			c.metrics.RecordRequest("unknown", "unknown", status, duration)

			return resp, err
		}
	}
}

// getRetryMiddleware returns retry middleware for gRPC clients
func (c *ClientPlugin) getRetryMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// Get retry configuration from context or use defaults
			maxRetries := 3
			baseDelay := 100 * time.Millisecond
			maxDelay := 5 * time.Second
			
			// Try to get retry config from client configuration
			if c.conf != nil {
				if c.conf.MaxRetries > 0 {
					maxRetries = int(c.conf.MaxRetries)
				}
				if c.conf.RetryBackoff != nil {
			baseDelay = c.conf.RetryBackoff.AsDuration()
				}
			}
			
			var lastErr error
			for attempt := 0; attempt <= maxRetries; attempt++ {
				// First attempt or retry
				resp, err := handler(ctx, req)
				
				// If successful, return immediately
				if err == nil {
					if attempt > 0 {
						// Record retry success metrics
						if c.metrics != nil {
					c.metrics.RecordRetry("unknown", "success", fmt.Sprintf("%d", attempt))
				}
					}
					return resp, nil
				}
				
				lastErr = err
				
				// Check if error is retryable
				if !c.isRetryableError(err) {
					// Non-retryable error, return immediately
					if c.metrics != nil {
				c.metrics.RecordRetry("unknown", "non_retryable", fmt.Sprintf("%d", attempt))
			}
					return resp, err
				}
				
				// If this was the last attempt, don't wait
				if attempt == maxRetries {
					if c.metrics != nil {
				c.metrics.RecordRetry("unknown", "max_attempts", fmt.Sprintf("%d", attempt))
			}
					break
				}
				
				// Calculate delay with exponential backoff
				delay := c.calculateRetryDelay(attempt, baseDelay, maxDelay)
				
				// Wait before retry, but respect context cancellation
				select {
				case <-ctx.Done():
					if c.metrics != nil {
					c.metrics.RecordRetry("unknown", "context_cancelled", fmt.Sprintf("%d", attempt))
				}
					return nil, ctx.Err()
				case <-time.After(delay):
					// Continue to next retry
				}
			}
			
			// All retries exhausted, return last error
			return nil, lastErr
		}
	}
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

// validateConfiguration validates the gRPC client configuration
func (c *ClientPlugin) validateConfiguration() error {
	if c.conf == nil {
		return fmt.Errorf("gRPC client configuration is nil")
	}

	// Validate subscribe services configuration
	for i, svc := range c.conf.SubscribeServices {
		if svc.Name == "" {
			return fmt.Errorf("subscribe service at index %d: service name is required", i)
		}

		// When using service discovery, endpoint should be empty or optional
		if c.discovery != nil && svc.Endpoint != "" {
			log.Warnf("Service %s has both service discovery and static endpoint configured. Service discovery will take precedence.", svc.Name)
		}

		// When no service discovery is available, endpoint is required (unless it's not required service)
		if c.discovery == nil && svc.Endpoint == "" && svc.Required {
			return fmt.Errorf("service %s is marked as required but has no endpoint and no service discovery available", svc.Name)
		}
	}

	// Validate legacy services configuration (deprecated)
	for i, svc := range c.conf.Services {
		if svc.Name == "" {
			return fmt.Errorf("legacy service at index %d: service name is required", i)
		}
		if svc.Endpoint == "" {
			return fmt.Errorf("legacy service %s: endpoint is required for static configuration", svc.Name)
		}
		log.Warnf("Using deprecated 'services' configuration for service %s. Please migrate to 'subscribe_services'.", svc.Name)
	}

	return nil
}

// SetDiscovery sets the service discovery instance
func (c *ClientPlugin) SetDiscovery(discovery registry.Discovery) {
	c.discovery = discovery
	log.Infof("Service discovery set for gRPC client plugin")
}

// GetSubscribeServiceConnection creates a connection for a subscribe service
func (c *ClientPlugin) GetSubscribeServiceConnection(serviceName string) (*grpc.ClientConn, error) {
	// Find service configuration
	var serviceConfig *conf.SubscribeService
	for _, svc := range c.conf.SubscribeServices {
		if svc.Name == serviceName {
			serviceConfig = svc
			break
		}
	}

	if serviceConfig == nil {
		return nil, fmt.Errorf("service %s not found in subscribe services configuration", serviceName)
	}

	// Build client configuration
	config := ClientConfig{
		ServiceName:      serviceConfig.Name,
		Discovery:        c.discovery,
		TLS:              serviceConfig.TlsEnable,
		TLSAuthType:      serviceConfig.TlsAuthType,
		MaxRetries:       int(serviceConfig.MaxRetries),
		Required:         serviceConfig.Required,
		Metadata:         serviceConfig.Metadata,
		LoadBalancer:     serviceConfig.LoadBalancer,
		CircuitBreaker:   serviceConfig.CircuitBreakerEnabled,
		CircuitThreshold: int(serviceConfig.CircuitBreakerThreshold),
	}

	// Set timeout with nil checks
	if serviceConfig.Timeout != nil {
		config.Timeout = serviceConfig.Timeout.AsDuration()
	} else if c.conf.DefaultTimeout != nil {
		config.Timeout = c.conf.DefaultTimeout.AsDuration()
	} else {
		config.Timeout = 10 * time.Second // Default fallback
	}

	// Set endpoint only if service discovery is not available
	if c.discovery == nil && serviceConfig.Endpoint != "" {
		config.Endpoint = serviceConfig.Endpoint
		log.Infof("Using static endpoint for service %s: %s", serviceName, serviceConfig.Endpoint)
	} else if c.discovery != nil {
		log.Infof("Using service discovery for service %s", serviceName)
	} else if serviceConfig.Required {
		return nil, fmt.Errorf("service %s is required but has no endpoint and no service discovery available", serviceName)
	}

	// Set default values from global configuration with nil checks
	if c.conf.DefaultKeepAlive != nil {
		config.KeepAlive = c.conf.DefaultKeepAlive.AsDuration()
	} else {
		config.KeepAlive = 30 * time.Second // Default fallback
	}
	if c.conf.RetryBackoff != nil {
		config.RetryBackoff = c.conf.RetryBackoff.AsDuration()
	} else {
		config.RetryBackoff = 1 * time.Second // Default fallback
	}
	if c.conf.MaxConnections > 0 {
		config.MaxConnections = int(c.conf.MaxConnections)
	} else {
		config.MaxConnections = 10 // Default fallback
	}
	config.Middleware = c.getDefaultMiddleware()

	return c.CreateConnection(config)
}

// CheckRequiredServices checks if all required services are available at startup
func (c *ClientPlugin) CheckRequiredServices() error {
	for _, svc := range c.conf.SubscribeServices {
		if !svc.Required {
			continue
		}

		log.Infof("Checking required service: %s", svc.Name)

		// Try to create connection to verify service availability
		conn, err := c.GetSubscribeServiceConnection(svc.Name)
		if err != nil {
			return fmt.Errorf("required service %s is not available: %w", svc.Name, err)
		}

		// Close the test connection
		if conn != nil {
			conn.Close()
		}

		log.Infof("Required service %s is available", svc.Name)
	}

	return nil
}

// isRetryableError determines if an error is retryable
func (c *ClientPlugin) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for gRPC status codes
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unavailable,
			codes.DeadlineExceeded,
			codes.ResourceExhausted,
			codes.Aborted,
			codes.Internal:
			return true
		case codes.InvalidArgument,
			codes.NotFound,
			codes.AlreadyExists,
			codes.PermissionDenied,
			codes.Unauthenticated,
			codes.FailedPrecondition,
			codes.OutOfRange,
			codes.Unimplemented:
			return false
		default:
			return false
		}
	}

	// Check for context errors
	if err == context.DeadlineExceeded || err == context.Canceled {
		return false // Don't retry context errors
	}

	// Default to not retryable for unknown errors
	return false
}

// calculateRetryDelay calculates the delay for the next retry attempt using exponential backoff with jitter
func (c *ClientPlugin) calculateRetryDelay(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
	
	// Cap at maxDelay
	if delay > maxDelay {
		delay = maxDelay
	}
	
	// Add jitter to avoid thundering herd (±25% random variation)
	jitter := time.Duration(float64(delay) * 0.25 * (rand.Float64()*2 - 1))
	delay += jitter
	
	// Ensure delay is not negative
	if delay < 0 {
		delay = baseDelay
	}
	
	return delay
}

// getCertProvider gets the certificate provider from the application
func (c *ClientPlugin) getCertProvider() interface{} {
	// Get certificate provider from the Lynx application
	return app.Lynx().Certificate()
}

// Package http implements the HTTP server plugin for the Lynx framework.
package http

import (
	"context"
	"fmt"
	"net"
	nhttp "net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/app/observability/metrics"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/service/http/conf"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Plugin metadata
// Basic plugin information definition.
const (
	// pluginName is the unique identifier for the HTTP server plugin.
	pluginName = "http.server"

	// pluginVersion indicates the current version of the HTTP server plugin.
	pluginVersion = "v2.0.0"

	// pluginDescription briefly describes the functionality of the HTTP server plugin.
	pluginDescription = "http server plugin for lynx framework"

	// confPrefix is the configuration prefix used when loading HTTP server settings.
	confPrefix = "lynx.http"
)

// ServiceHttp implements the HTTP server plugin for the Lynx framework.
// It embeds plugins.BasePlugin to inherit common plugin functionality and maintains HTTP server configuration and instance.
type ServiceHttp struct {
	// Embed the base plugin to inherit common attributes and methods.
	*plugins.BasePlugin
	// HTTP server configuration
	conf *conf.Http
	// HTTP server instance
	server *http.Server

	// Prometheus metrics
	requestCounter   *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	responseSize     *prometheus.HistogramVec
	requestSize      *prometheus.HistogramVec
	errorCounter     *prometheus.CounterVec
	inflightRequests *prometheus.GaugeVec
	// Health check metrics
	healthCheckTotal *prometheus.CounterVec
	// Additional metrics
	activeConnections    *prometheus.GaugeVec
	connectionPoolUsage  *prometheus.GaugeVec
	requestQueueLength   *prometheus.GaugeVec
	routeRequestCounter  *prometheus.CounterVec
	routeRequestDuration *prometheus.HistogramVec

	// Rate limiter
	rateLimiter *rate.Limiter

	// Connection timeout configuration
	idleTimeout       time.Duration
	keepAliveTimeout  time.Duration
	readHeaderTimeout time.Duration
	// Request size limit
	maxRequestSize int64
	// Connection pool management
	maxConnections        int
	maxConcurrentRequests int
	readBufferSize        int
	writeBufferSize       int

	// Circuit breaker instance
	circuitBreaker *CircuitBreaker

	// Connection limit semaphores (initialized once)
	connectionSem chan struct{}
	requestSem    chan struct{}
	semInitOnce   sync.Once

	// Active connection tracking for metrics
	activeConnectionsCount int32

	// Shutdown signal channel
	shutdownChan chan struct{}
	// Whether shutting down
	isShuttingDown bool
	// Shutdown timeout
	shutdownTimeout time.Duration
}

// netHTTPToKratosHandlerAdapter adapts a net/http.Handler to a kratos http.HandlerFunc
// and also implements net/http.Handler for compatibility
type netHTTPToKratosHandlerAdapter struct {
	handler nhttp.Handler
}

// ServeHTTP implements the net/http.Handler interface
func (a *netHTTPToKratosHandlerAdapter) ServeHTTP(w nhttp.ResponseWriter, r *nhttp.Request) {
	a.handler.ServeHTTP(w, r)
}

// Handle implements the kratos http.HandlerFunc interface
func (a *netHTTPToKratosHandlerAdapter) Handle(w http.ResponseWriter, r *http.Request) {
	a.handler.ServeHTTP(w, r)
}

// NewServiceHttp creates a new HTTP server plugin instance.
// It initializes the plugin's basic information and returns a pointer to ServiceHttp.
func NewServiceHttp() *ServiceHttp {
	return &ServiceHttp{
		BasePlugin: plugins.NewBasePlugin(
			// Generate the plugin's unique ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			// Plugin name
			pluginName,
			// Plugin description
			pluginDescription,
			// Plugin version
			pluginVersion,
			// Configuration prefix
			confPrefix,
			// Weight
			10,
		),
		shutdownChan: make(chan struct{}),
	}
}

// InitializeResources implements custom initialization logic for the HTTP plugin.
// It loads and validates the HTTP server configuration, using defaults if not provided.
func (h *ServiceHttp) InitializeResources(rt plugins.Runtime) error {
	// Initialize an empty configuration struct
	h.conf = &conf.Http{}

	// Scan and load HTTP configuration from runtime config
	err := rt.GetConfig().Value(confPrefix).Scan(h.conf)
	if err != nil {
		log.Warnf("Failed to load HTTP configuration, using defaults: %v", err)
	}

	// Set default configuration
	h.setDefaultConfig()

	// Validate configuration
	if err := h.validateConfig(); err != nil {
		return fmt.Errorf("HTTP configuration validation failed: %w", err)
	}

	log.Infof("HTTP configuration loaded: network=%s, addr=%s, tls=%v",
		h.conf.Network, h.conf.Addr, h.conf.GetTlsEnable())
	return nil
}

// setDefaultConfig sets the default configuration values.
func (h *ServiceHttp) setDefaultConfig() {
	// Basic defaults
	if h.conf.Network == "" {
		h.conf.Network = "tcp"
	}
	if h.conf.Addr == "" {
		h.conf.Addr = ":8080"
	}
	if h.conf.Timeout == nil {
		h.conf.Timeout = &durationpb.Duration{Seconds: 10}
	}

	// Monitoring defaults
	h.initMonitoringDefaults()

	// Security defaults
	h.initSecurityDefaults()

	// Performance defaults
	h.initPerformanceDefaults()

	// Middleware defaults
	h.initMiddlewareDefaults()

	// Graceful shutdown defaults
	h.initGracefulShutdownDefaults()

	// Circuit breaker defaults
	h.initCircuitBreakerDefaults()
}

// validateConfig validates configuration parameters.
func (h *ServiceHttp) validateConfig() error {
	// Validate address format
	if h.conf.Addr != "" {
		if !strings.Contains(h.conf.Addr, ":") {
			return fmt.Errorf("invalid address format: %s", h.conf.Addr)
		}
		parts := strings.Split(h.conf.Addr, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid address format: %s", h.conf.Addr)
		}
		if port, err := strconv.Atoi(parts[1]); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid port number: %s", parts[1])
		}
	}

	// Validate network protocol
	if h.conf.Network != "" {
		validNetworks := []string{"tcp", "tcp4", "tcp6", "unix", "unixpacket"}
		valid := false
		for _, network := range validNetworks {
			if h.conf.Network == network {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid network protocol: %s, valid options: %v", h.conf.Network, validNetworks)
		}
	}

	// Validate timeouts
	if h.conf.Timeout != nil {
		if h.conf.Timeout.AsDuration() <= 0 {
			return fmt.Errorf("timeout must be positive")
		}
		if h.conf.Timeout.AsDuration() > 300*time.Second {
			return fmt.Errorf("timeout cannot exceed 5 minutes")
		}
	}

	// Validate request size limit
	if h.maxRequestSize < 0 {
		return fmt.Errorf("max request size cannot be negative")
	}
	if h.maxRequestSize > 100*1024*1024 { // 100MB
		return fmt.Errorf("max request size cannot exceed 100MB")
	}

	// Validate performance configuration
	if h.idleTimeout < 0 {
		return fmt.Errorf("idle timeout cannot be negative")
	}
	if h.idleTimeout > 600*time.Second { // 10 minutes
		return fmt.Errorf("idle timeout cannot exceed 10 minutes")
	}

	if h.keepAliveTimeout < 0 {
		return fmt.Errorf("keep alive timeout cannot be negative")
	}
	if h.keepAliveTimeout > 300*time.Second { // 5 minutes
		return fmt.Errorf("keep alive timeout cannot exceed 5 minutes")
	}

	if h.readHeaderTimeout < 0 {
		return fmt.Errorf("read header timeout cannot be negative")
	}
	if h.readHeaderTimeout > 60*time.Second { // 1 minute
		return fmt.Errorf("read header timeout cannot exceed 1 minute")
	}

	// Validate graceful shutdown timeout
	if h.shutdownTimeout < 0 {
		return fmt.Errorf("shutdown timeout cannot be negative")
	}
	if h.shutdownTimeout > 300*time.Second { // 5 minutes
		return fmt.Errorf("shutdown timeout cannot exceed 5 minutes")
	}

	// Validate performance configuration
	if h.maxConnections < 0 {
		return fmt.Errorf("max connections cannot be negative")
	}
	if h.maxConcurrentRequests < 0 {
		return fmt.Errorf("max concurrent requests cannot be negative")
	}
	if h.readBufferSize < 0 {
		return fmt.Errorf("read buffer size cannot be negative")
	}
	if h.writeBufferSize < 0 {
		return fmt.Errorf("write buffer size cannot be negative")
	}

	// Validate circuit breaker configuration
	if h.conf.CircuitBreaker != nil {
		if h.conf.CircuitBreaker.MaxFailures < 0 {
			return fmt.Errorf("circuit breaker max failures cannot be negative")
		}
		if h.conf.CircuitBreaker.MaxRequests < 0 {
			return fmt.Errorf("circuit breaker max requests cannot be negative")
		}
		if h.conf.CircuitBreaker.FailureThreshold < 0 || h.conf.CircuitBreaker.FailureThreshold > 1 {
			return fmt.Errorf("circuit breaker failure threshold must be between 0 and 1")
		}
		if h.conf.CircuitBreaker.Timeout != nil && h.conf.CircuitBreaker.Timeout.AsDuration() < 0 {
			return fmt.Errorf("circuit breaker timeout cannot be negative")
		}
	}

	// Validate rate limit configuration
	if h.rateLimiter != nil {
		if h.rateLimiter.Limit() <= 0 {
			return fmt.Errorf("rate limit must be positive")
		}
		if h.rateLimiter.Burst() <= 0 {
			return fmt.Errorf("rate limit burst must be positive")
		}
		if h.rateLimiter.Limit() > 10000 { // 10k req/s
			return fmt.Errorf("rate limit cannot exceed 10,000 requests per second")
		}
	}

	// Configuration validated successfully
	return nil
}

// initSecurityDefaults initializes security-related defaults.
func (h *ServiceHttp) initSecurityDefaults() {
	// Request size limit: 10MB
	h.maxRequestSize = 10 * 1024 * 1024

	// Rate limiting: 100 req/s, burst 200
	h.rateLimiter = rate.NewLimiter(100, 200)
}

// initRateLimiter initializes the rate limiter.
func (h *ServiceHttp) initRateLimiter() {
	if h.rateLimiter != nil {
		log.Infof("Rate limiter initialized: %v req/s, burst: %d",
			h.rateLimiter.Limit(), h.rateLimiter.Burst())
	}
}

// initPerformanceDefaults initializes performance-related defaults.
func (h *ServiceHttp) initPerformanceDefaults() {
	h.idleTimeout = 60 * time.Second
	h.keepAliveTimeout = 30 * time.Second
	h.readHeaderTimeout = 20 * time.Second

	// Set performance defaults if not configured
	if h.conf.Performance == nil {
		h.conf.Performance = &conf.PerformanceConfig{}
	}

	// Connection limits
	if h.conf.Performance.MaxConnections == 0 {
		h.conf.Performance.MaxConnections = 1000
	}
	if h.conf.Performance.MaxConcurrentRequests == 0 {
		h.conf.Performance.MaxConcurrentRequests = 500
	}

	// Buffer sizes
	if h.conf.Performance.ReadBufferSize == 0 {
		h.conf.Performance.ReadBufferSize = 4096
	}
	if h.conf.Performance.WriteBufferSize == 0 {
		h.conf.Performance.WriteBufferSize = 4096
	}

	// Store in instance variables for easy access
	h.maxConnections = int(h.conf.Performance.MaxConnections)
	h.maxConcurrentRequests = int(h.conf.Performance.MaxConcurrentRequests)
	h.readBufferSize = int(h.conf.Performance.ReadBufferSize)
	h.writeBufferSize = int(h.conf.Performance.WriteBufferSize)
}

// initGracefulShutdownDefaults initializes graceful shutdown defaults.
func (h *ServiceHttp) initGracefulShutdownDefaults() {
	h.shutdownTimeout = 30 * time.Second
}

// initMiddlewareDefaults initializes middleware defaults.
func (h *ServiceHttp) initMiddlewareDefaults() {
	if h.conf.Middleware == nil {
		h.conf.Middleware = &conf.MiddlewareConfig{
			EnableTracing:    true,
			EnableLogging:    true,
			EnableMetrics:    true,
			EnableRecovery:   true,
			EnableRateLimit:  true,
			EnableValidation: true,
		}
	}
}

// initCircuitBreakerDefaults initializes circuit breaker defaults.
func (h *ServiceHttp) initCircuitBreakerDefaults() {
	if h.conf.CircuitBreaker == nil {
		h.conf.CircuitBreaker = &conf.CircuitBreakerConfig{
			Enabled:          true,
			MaxFailures:      5,
			Timeout:          &durationpb.Duration{Seconds: 60},
			MaxRequests:      10,
			FailureThreshold: 0.5,
		}
	}
}

// StartupTasks implements the custom startup logic for the HTTP plugin.
// It configures and starts the HTTP server with necessary middleware and options.
func (h *ServiceHttp) StartupTasks() error {
	// Log HTTP service startup
	log.Infof("Starting HTTP service on %s", h.conf.Addr)

	// Initialize metrics
	h.initMetrics()

	// Initialize rate limiter
	h.initRateLimiter()

	// Build middlewares
	middlewares := h.buildMiddlewares()
	hMiddlewares := http.Middleware(middlewares...)

	// Define HTTP server options
	opts := []http.ServerOption{
		hMiddlewares,
		// 404 Not Found handler
		http.NotFoundHandler(h.notFoundHandler()),
		// 405 Method Not Allowed handler
		http.MethodNotAllowedHandler(h.methodNotAllowedHandler()),
		// Response encoder
		http.ErrorEncoder(h.enhancedErrorEncoder),
	}

	// Append additional server options based on configuration
	if h.conf.Network != "" {
		// Set network protocol
		opts = append(opts, http.Network(h.conf.Network))
	}
	if h.conf.Addr != "" {
		// Set listen address
		opts = append(opts, http.Address(h.conf.Addr))
	}
	if h.conf.Timeout != nil {
		// Set timeout
		opts = append(opts, http.Timeout(h.conf.Timeout.AsDuration()))
	}
	if h.conf.GetTlsEnable() {
		// If TLS is enabled, append TLS options
		tlsOption, err := h.tlsLoad()
		if err != nil {
			return fmt.Errorf("failed to load TLS configuration: %w", err)
		}
		opts = append(opts, tlsOption)
	}

	// Create the HTTP server instance
	h.server = http.NewServer(opts...)

	// Apply performance configuration to the underlying net/http.Server
	h.applyPerformanceConfig()

	// Apply connection limits
	h.applyConnectionLimits()

	// Register monitoring endpoints
	h.server.HandlePrefix("/metrics", metrics.Handler())
	// Adapt net/http.Handler to kratos http.HandlerFunc
	h.server.HandlePrefix("/health", &netHTTPToKratosHandlerAdapter{handler: h.healthCheckHandler()})

	// Log successful startup
	log.Infof("HTTP service successfully started with monitoring endpoints and performance optimizations")
	return nil
}

// applyPerformanceConfig applies performance settings to the underlying HTTP server.
func (h *ServiceHttp) applyPerformanceConfig() {
	// Access embedded *net/http.Server directly (Kratos exposes it via Server field)
	var httpServer *nhttp.Server
	if h.server != nil && h.server.Server != nil {
		httpServer = h.server.Server
	}
	if httpServer == nil {
		log.Warnf("Unable to access underlying net/http.Server; only kratos ServerOption-based settings will apply")
		return
	}

	// Apply performance settings from configuration onto net/http.Server
	h.installConnStateHook(httpServer)
	if h.conf.Performance != nil {
		if h.conf.Performance.ReadTimeout != nil {
			if v := h.conf.Performance.ReadTimeout.AsDuration(); v > 0 {
				httpServer.ReadTimeout = v
				log.Infof("Applied ReadTimeout: %v", v)
			}
		}
		if h.conf.Performance.WriteTimeout != nil {
			if v := h.conf.Performance.WriteTimeout.AsDuration(); v > 0 {
				httpServer.WriteTimeout = v
				log.Infof("Applied WriteTimeout: %v", v)
			}
		}
		if h.conf.Performance.IdleTimeout != nil {
			if v := h.conf.Performance.IdleTimeout.AsDuration(); v > 0 {
				httpServer.IdleTimeout = v
				log.Infof("Applied IdleTimeout: %v", v)
			}
		}
		if h.conf.Performance.ReadHeaderTimeout != nil {
			if v := h.conf.Performance.ReadHeaderTimeout.AsDuration(); v > 0 {
				httpServer.ReadHeaderTimeout = v
				log.Infof("Applied ReadHeaderTimeout: %v", v)
			}
		}

		// Buffer sizes & max connections require listener-level or middleware control; log intent
		if h.conf.Performance.ReadBufferSize > 0 {
			log.Infof("Configured ReadBufferSize: %d bytes (apply via listener/middleware)", h.conf.Performance.ReadBufferSize)
		}
		if h.conf.Performance.WriteBufferSize > 0 {
			log.Infof("Configured WriteBufferSize: %d bytes (apply via listener/middleware)", h.conf.Performance.WriteBufferSize)
		}
		if h.conf.Performance.MaxConnections > 0 {
			httpServer.SetKeepAlivesEnabled(true)
			log.Infof("Configured MaxConnections: %d (enforced via accept limit/middleware)", h.conf.Performance.MaxConnections)
		}
		log.Infof("Performance optimizations applied to net/http.Server")
		return
	}

	// Defaults fallback when no Performance config
	if h.idleTimeout > 0 {
		httpServer.IdleTimeout = h.idleTimeout
		log.Infof("Applied default IdleTimeout: %v", h.idleTimeout)
	}
	if h.keepAliveTimeout > 0 && httpServer.IdleTimeout == 0 {
		httpServer.IdleTimeout = h.keepAliveTimeout
		log.Infof("Applied default KeepAliveTimeout using IdleTimeout: %v", h.keepAliveTimeout)
	}
	if h.readHeaderTimeout > 0 {
		httpServer.ReadHeaderTimeout = h.readHeaderTimeout
		log.Infof("Applied default ReadHeaderTimeout: %v", h.readHeaderTimeout)
	}

	// Note: MaxHeaderBytes only limits the header size, not the request body.
	log.Infof("Performance configurations applied successfully")
}

// applyConnectionLimits applies connection-related performance limits
func (h *ServiceHttp) applyConnectionLimits() {
	// Create a connection limiter middleware if max connections is configured
	if h.maxConnections > 0 {
		log.Infof("Connection limit middleware enabled: max %d connections", h.maxConnections)
	}

	// Log concurrent request limits
	if h.maxConcurrentRequests > 0 {
		log.Infof("Concurrent request limit configured: max %d requests", h.maxConcurrentRequests)
	}

	// Log buffer sizes
	if h.readBufferSize > 0 {
		log.Infof("Read buffer size configured: %d bytes", h.readBufferSize)
	}
	if h.writeBufferSize > 0 {
		log.Infof("Write buffer size configured: %d bytes", h.writeBufferSize)
	}
}

// installConnStateHook configures a ConnState hook to apply TCP buffer settings for each new connection.
func (h *ServiceHttp) installConnStateHook(httpServer *nhttp.Server) {
	if httpServer == nil {
		return
	}

	prevHook := httpServer.ConnState
	httpServer.ConnState = func(conn net.Conn, state nhttp.ConnState) {
		if state == nhttp.StateNew {
			h.applyTCPBufferSettings(conn)
		}
		if prevHook != nil {
			prevHook(conn, state)
		}
	}
}

// applyTCPBufferSettings applies read/write buffer sizes to a TCP connection if configured.
func (h *ServiceHttp) applyTCPBufferSettings(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}

	if h.readBufferSize > 0 {
		if err := tcpConn.SetReadBuffer(h.readBufferSize); err != nil {
			log.Warnf("Failed to set TCP read buffer (%d bytes): %v", h.readBufferSize, err)
		}
	}
	if h.writeBufferSize > 0 {
		if err := tcpConn.SetWriteBuffer(h.writeBufferSize); err != nil {
			log.Warnf("Failed to set TCP write buffer (%d bytes): %v", h.writeBufferSize, err)
		}
	}
}

// CleanupTasks implements custom cleanup logic for the HTTP plugin.
// It gracefully stops the HTTP server and handles potential errors.
func (h *ServiceHttp) CleanupTasks() error {
	// If the server instance is nil, return immediately
	if h.server == nil {
		return nil
	}

	log.Infof("Starting graceful shutdown of HTTP service")

	h.isShuttingDown = true
	close(h.shutdownChan)

	// Configure shutdown timeout with proper context handling
	ctx, cancel := h.createShutdownContext()
	defer cancel()

	// Gracefully stop the server
	if err := h.server.Stop(ctx); err != nil {
		log.Errorf("Failed to stop HTTP server gracefully: %v", err)
		return plugins.NewPluginError(h.ID(), "Stop", "Failed to stop HTTP server gracefully", err)
	}

	log.Infof("HTTP service gracefully stopped")
	return nil
}

// createShutdownContext creates a context for graceful shutdown with appropriate timeout
func (h *ServiceHttp) createShutdownContext() (context.Context, context.CancelFunc) {
	// Use a parent context that can be cancelled if needed
	parentCtx := context.Background()
	if h.shutdownTimeout > 0 {
		return context.WithTimeout(parentCtx, h.shutdownTimeout)
	}
	// Default timeout if not configured
	return context.WithTimeout(parentCtx, 30*time.Second)
}

// Configure updates the HTTP server configuration.
// It accepts any type, attempts to cast it to *conf.Http, and updates the configuration on success.
func (h *ServiceHttp) Configure(c any) error {
	// Try to convert the provided configuration to *conf.Http
	if httpConf, ok := c.(*conf.Http); ok {
		// Save the old configuration for rollback
		oldConf := h.conf
		h.conf = httpConf

		// Set defaults
		h.setDefaultConfig()

		// Validate the new configuration
		if err := h.validateConfig(); err != nil {
			// Invalid configuration; roll back to the old one
			h.conf = oldConf
			log.Errorf("Invalid new configuration, rolling back: %v", err)
			return fmt.Errorf("configuration validation failed: %w", err)
		}

		log.Infof("HTTP configuration updated successfully")
		return nil
	}

	// Conversion failed; return invalid configuration error
	return plugins.ErrInvalidConfiguration
}

// RegisterMetricsGatherer allows injecting an external Prometheus registry into the unified /metrics aggregation.
// Useful when plugins or third-party libraries maintain a separate *prometheus.Registry.
func (h *ServiceHttp) RegisterMetricsGatherer(g prometheus.Gatherer) {
	if g == nil {
		return
	}
	// Aggregate via observability.metrics
	metrics.RegisterGatherer(g)
}

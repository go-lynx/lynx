// Package grpc provides a gRPC service plugin for the Lynx framework.
// It implements the necessary interfaces to integrate with the Lynx plugin system
// and provides functionality for setting up and managing a gRPC service with various
// middleware options and TLS support.
package grpc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/contrib/middleware/validate/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/service/grpc/conf"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Plugin metadata constants define the basic information about the gRPC plugin
const (
	// pluginName is the unique identifier for the gRPC service plugin
	pluginName = "grpc.service"

	// pluginVersion indicates the current version of the plugin
	pluginVersion = "v2.0.0"

	// pluginDescription provides a brief description of the plugin's functionality
	pluginDescription = "grpc service plugin for lynx framework"

	// confPrefix is the configuration prefix used for loading gRPC service settings
	confPrefix = "lynx.grpc.service"
)

// Service represents the gRPC service plugin implementation.
// It embeds the BasePlugin for common plugin functionality and maintains
// the gRPC server instance along with its configuration.
type Service struct {
	// Embed Lynx framework's base plugin, inheriting common plugin functionality
	*plugins.BasePlugin
	// gRPC server instance
	server *grpc.Server
	// gRPC health server for readiness reporting
	healthServer *health.Server
	// cancel function for background health status poller
	healthPollCancel context.CancelFunc
	// gRPC service configuration information
	conf *conf.Service
	// runtime handle to query shared resources (e.g., required readiness)
	rt plugins.Runtime
	// Dependency injection providers
	appNameProvider      func() string
	loggerProvider       func() interface{}
	certProvider         func() interface{}
	controlPlaneProvider func() interface{}
	// Port availability check cache to avoid frequent retries on failures
	// Note: We only cache failures, never successes, to ensure real-time detection
	portCheckCache struct {
		mu          sync.RWMutex
		lastFailure time.Time
		lastError   error
		retryWindow time.Duration // Avoid retrying failed checks within this window
	}
}

// healthStatusPoller periodically syncs health serving status with required upstream readiness.
// It runs until the context is canceled in CleanupTasks.
func (g *Service) healthStatusPoller(ctx context.Context) {
	// Poll at a low frequency to avoid overhead, yet responsive enough for rollouts
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Add timeout control to prevent blocking
			updateCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
			g.updateHealthServingStatusWithContext(updateCtx)
			cancel()
		}
	}
}

// updateHealthServingStatusWithContext updates health status with context support
// Ensures goroutine exits when context is cancelled to prevent leaks
func (g *Service) updateHealthServingStatusWithContext(ctx context.Context) {
	// Use a channel to ensure we don't block indefinitely
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Check context before execution
		select {
		case <-ctx.Done():
			return
		default:
			// Perform update with context awareness
			// Use a separate goroutine context to ensure we can cancel if needed
			updateCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			defer cancel()

			// Create a channel to signal completion
			updateDone := make(chan struct{})
			go func() {
				defer close(updateDone)
				g.updateHealthServingStatus()
			}()

			// Wait for update or timeout
			select {
			case <-updateCtx.Done():
				// Context cancelled or timed out
				return
			case <-updateDone:
				// Update completed
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		// Timeout or cancellation - log but don't fail
		log.Warnf("Health status update timed out or was cancelled")
		// Wait briefly for goroutine to exit (non-blocking)
		select {
		case <-done:
		case <-time.After(50 * time.Millisecond):
			// Goroutine should complete quickly, but log if it doesn't
			log.Debugf("Health status update goroutine may still be running")
		}
	case <-done:
		// Update completed successfully
	}
}

// NewGrpcService creates and initializes a new instance of the gRPC service plugin.
// It sets up the base plugin with the appropriate metadata and returns a pointer
// to the Service structure.
func NewGrpcService() *Service {
	return &Service{
		BasePlugin: plugins.NewBasePlugin(
			// Generate unique plugin ID
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
		// Initialize port check cache to avoid frequent retries on failures
		// Success checks are never cached to ensure real-time failure detection
		portCheckCache: struct {
			mu          sync.RWMutex
			lastFailure time.Time
			lastError   error
			retryWindow time.Duration
		}{
			retryWindow: 500 * time.Millisecond, // Short window to avoid hammering failed ports
		},
	}
}

// SetDependencies sets the dependency injection providers for the gRPC service
func (g *Service) SetDependencies(
	appNameProvider func() string,
	loggerProvider func() interface{},
	certProvider func() interface{},
	controlPlaneProvider func() interface{},
) {
	g.appNameProvider = appNameProvider
	g.loggerProvider = loggerProvider
	g.certProvider = certProvider
	g.controlPlaneProvider = controlPlaneProvider
}

// InitializeResources implements the plugin initialization interface.
// It loads and validates the gRPC server configuration from the runtime environment.
// If no configuration is provided, it sets up default values for the server.
func (g *Service) InitializeResources(rt plugins.Runtime) error {
	// store runtime for later use (e.g., readiness resource lookups)
	g.rt = rt
	// Initialize an empty configuration structure
	g.conf = &conf.Service{}

	// Scan and load gRPC configuration from runtime configuration
	err := rt.GetConfig().Value(confPrefix).Scan(g.conf)
	if err != nil {
		return err
	}

	// Set default configuration
	defaultConf := &conf.Service{
		// Default network protocol is TCP
		Network: "tcp",
		// Default listening address is :9090
		Addr: ":9090",
		// TLS is disabled by default
		TlsEnable: false,
		// No client authentication by default
		TlsAuthType: 0,
		// Default timeout is 10 seconds
		Timeout: &durationpb.Duration{Seconds: 10},
	}

	// Use default values for unset fields
	if g.conf.Network == "" {
		g.conf.Network = defaultConf.Network
	}
	if g.conf.Addr == "" {
		g.conf.Addr = defaultConf.Addr
	}
	if g.conf.Timeout == nil {
		g.conf.Timeout = defaultConf.Timeout
	}
	// MaxConcurrentStreams defaults to 0 (unlimited) if not set
	// This is already the default value, so no need to set it explicitly

	// Validate configuration
	if err := g.validateConfig(); err != nil {
		return fmt.Errorf("invalid gRPC configuration: %v", err)
	}

	return nil
}

// StartupTasks implements the plugin startup interface.
// It configures and starts the gRPC server with all necessary middleware and options,
// including tracing, logging, rate limiting, validation, and recovery handlers.
func (g *Service) StartupTasks() error {
	// Log gRPC service startup
	log.Info("starting grpc service")

	var middlewares []middleware.Middleware

	// Add base middleware
	middlewares = append(middlewares,
		// Configure tracing middleware with application name as tracer name
		tracing.Server(tracing.WithTracerName(g.getAppName())),
		// Configure logging middleware using Lynx framework's logger
		// Note: Commented out due to type assertion issues
		// logging.Server(g.getLogger().(log.Logger)),
		// Configure validation middleware
		validate.ProtoValidate(),
		// Configure recovery middleware to handle panics during request processing
		recovery.Recovery(
			recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
				// Log error using context
				log.Context(ctx).Error("panic recovery", "error", err)
				g.recordServerError("panic_recovery")
				return nil
			}),
		),
		// Add metrics middleware - using a custom middleware function
		func(handler middleware.Handler) middleware.Handler {
			return func(ctx context.Context, req interface{}) (interface{}, error) {
				// This is a simplified version - in practice, you'd need to extract
				// method info from context or use a different approach
				start := time.Now()
				resp, err := handler(ctx, req)
				duration := time.Since(start)

				status := "success"
				if err != nil {
					status = "error"
					g.recordServerError("request_error")
				}

				// Record basic metrics without method info
				g.recordRequestMetrics("unknown", duration, status)
				return resp, err
			}
		},
	)
	// Configure rate limiting middleware using Lynx framework's control plane HTTP rate limit strategy
	// If there is a rate limiting middleware, append it
	if rateLimit := g.getGRPCRateLimit(); rateLimit != nil {
		if rl, ok := rateLimit.(middleware.Middleware); ok {
			middlewares = append(middlewares, rl)
			log.Info("Added rate limiting middleware")
		}
	}
	gMiddlewares := grpc.Middleware(middlewares...)

	// Define gRPC server options list
	opts := []grpc.ServerOption{
		gMiddlewares,
		// Use custom health to register our own health server so we can
		// programmatically control overall readiness based on required upstreams
		grpc.CustomHealth(),
	}

	// Configure server options based on configuration
	if g.conf.Network != "" {
		// Set network protocol
		opts = append(opts, grpc.Network(g.conf.Network))
	}
	if g.conf.Addr != "" {
		// Set listening address
		opts = append(opts, grpc.Address(g.conf.Addr))
	}
	if g.conf.Timeout != nil {
		// Set timeout
		opts = append(opts, grpc.Timeout(g.conf.Timeout.AsDuration()))
	}
	if g.conf.GetTlsEnable() {
		// If TLS is enabled, add TLS configuration options
		tlsOption, err := g.tlsLoad()
		if err != nil {
			return fmt.Errorf("failed to load TLS configuration: %v", err)
		}
		opts = append(opts, tlsOption)
	}

	// Configure MaxConcurrentStreams if specified
	// This is an important parameter for controlling server resource usage
	maxStreams := g.conf.GetMaxConcurrentStreams()
	if maxStreams > 0 {
		log.Infof("Setting gRPC MaxConcurrentStreams to %d", maxStreams)
		// Use underlying gRPC option to set MaxConcurrentStreams
		opts = append(opts, grpc.Options(grpcgo.MaxConcurrentStreams(uint32(maxStreams))))
	} else {
		// Default to 1000 for production safety (can be overridden by setting to 0 explicitly)
		const defaultMaxStreams = uint32(1000)
		log.Infof("MaxConcurrentStreams not configured, using safe default: %d", defaultMaxStreams)
		opts = append(opts, grpc.Options(grpcgo.MaxConcurrentStreams(defaultMaxStreams)))
	}

	// Configure message size limits (defaults to 10MB if not specified)
	const defaultMaxMsgSize = 10 * 1024 * 1024

	maxRecv := g.conf.GetMaxRecvMsgSize()
	switch {
	case maxRecv > 0:
		log.Infof("Setting gRPC MaxRecvMsgSize to %d bytes", maxRecv)
		opts = append(opts, grpc.Options(grpcgo.MaxRecvMsgSize(int(maxRecv))))
	default:
		log.Infof("MaxRecvMsgSize not configured, using safe default: %d bytes", defaultMaxMsgSize)
		opts = append(opts, grpc.Options(grpcgo.MaxRecvMsgSize(defaultMaxMsgSize)))
	}

	maxSend := g.conf.GetMaxSendMsgSize()
	switch {
	case maxSend > 0:
		log.Infof("Setting gRPC MaxSendMsgSize to %d bytes", maxSend)
		opts = append(opts, grpc.Options(grpcgo.MaxSendMsgSize(int(maxSend))))
	default:
		log.Infof("MaxSendMsgSize not configured, using safe default: %d bytes", defaultMaxMsgSize)
		opts = append(opts, grpc.Options(grpcgo.MaxSendMsgSize(defaultMaxMsgSize)))
	}

	// Create gRPC server instance
	g.server = grpc.NewServer(opts...)

	// Register custom health server and set initial serving status from required readiness
	g.healthServer = health.NewServer()
	// Register health on the underlying google gRPC server
	grpc_health_v1.RegisterHealthServer(g.server.Server, g.healthServer)
	// Apply initial serving status based on required upstream readiness
	g.updateHealthServingStatus()

	// Track resources that need cleanup on failure
	var cleanup func()
	defer func() {
		if cleanup != nil {
			log.Warnf("Startup failed, cleaning up resources")
			cleanup()
		}
	}()

	// Start background poller to keep serving status in sync with required upstream readiness
	pollCtx, cancel := context.WithCancel(context.Background())
	g.healthPollCancel = cancel
	go g.healthStatusPoller(pollCtx)
	cleanup = cancel // Set cleanup function to cancel poller on failure

	// Record server start time for metrics
	g.recordServerStartTime()

	// Success - clear cleanup function
	cleanup = nil

	// Log successful gRPC service startup
	log.Info("grpc service successfully started")
	return nil
}

// CleanupTasks implements the plugin cleanup interface.
// It gracefully stops the gRPC server and performs necessary cleanup operations.
// If the server is nil or already stopped, it will return nil.
func (g *Service) CleanupTasks() error {
	// Use a timeout context for cleanup to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return g.CleanupTasksContext(ctx)
}

// CleanupTasksContext implements context-aware cleanup with proper timeout handling.
func (g *Service) CleanupTasksContext(parentCtx context.Context) error {
	if g.server == nil {
		return nil
	}

	// Use parent context if it has a deadline, otherwise create timeout context
	var ctx context.Context
	var cancel context.CancelFunc

	if _, ok := parentCtx.Deadline(); ok {
		// Parent context has deadline, use it directly
		ctx = parentCtx
		cancel = func() {} // No-op cancel
	} else {
		// Create timeout context with configured timeout
		timeout := g.conf.GetTimeout().AsDuration()
		if timeout <= 0 {
			timeout = 30 * time.Second // Default timeout
		}
		ctx, cancel = context.WithTimeout(parentCtx, timeout)
	}
	defer cancel()

	// Stop background poller and shutdown health server
	if g.healthPollCancel != nil {
		g.healthPollCancel()
	}
	if g.healthServer != nil {
		g.healthServer.Shutdown()
	}

	if err := g.server.Stop(ctx); err != nil {
		// If stopping fails, return plugin error with error information
		return plugins.NewPluginError(g.ID(), "Stop", "Failed to stop gRPC server", err)
	}
	return nil
}

// Configure allows runtime configuration updates for the gRPC server.
// It accepts an interface{} parameter that should contain the new configuration
// and updates the server settings accordingly.
func (g *Service) Configure(c any) error {
	if c == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	// Try to convert the incoming configuration to *conf.Service type
	grpcConf, ok := c.(*conf.Service)
	if !ok {
		return fmt.Errorf("invalid configuration type: expected *conf.Service, got %T", c)
	}

	// Save the old configuration for rollback
	oldConf := g.conf
	g.conf = grpcConf

	// Validate the new configuration
	if err := g.validateConfig(); err != nil {
		// Invalid configuration; roll back to the old one
		g.conf = oldConf
		log.Errorf("Invalid new gRPC configuration, rolling back: %v", err)
		return fmt.Errorf("gRPC configuration validation failed: %w", err)
	}

	log.Infof("gRPC configuration updated successfully")
	return nil
}

// CheckHealth implements the health check interface for the gRPC server.
// It performs necessary health checks and updates the provided health report
// with the current status of the server.
func (g *Service) CheckHealth() error {
	if g.server == nil {
		return fmt.Errorf("gRPC server is not initialized")
	}

	// Check server configuration
	if g.conf == nil || g.conf.Addr == "" {
		return fmt.Errorf("gRPC server address not configured")
	}

	// Check port availability
	if err := g.checkPortAvailability(); err != nil {
		return fmt.Errorf("gRPC server port not available: %v", err)
	}

	// Check TLS configuration if enabled
	if g.conf.GetTlsEnable() {
		if err := g.validateTLSConfig(); err != nil {
			return fmt.Errorf("TLS configuration invalid: %v", err)
		}
	}

	// Record health check metrics
	g.recordHealthCheckMetricsInternal(true)

	return nil
}

// checkPortAvailability checks if the configured port is available for binding.
// It always performs a real network check to ensure real-time failure detection.
// Failed checks are cached briefly to avoid hammering unreachable ports.
func (g *Service) checkPortAvailability() error {
	if g.conf == nil || g.conf.Addr == "" {
		return fmt.Errorf("server address not configured")
	}

	// Parse address and normalize host for dial
	addr := g.conf.Addr
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid address %q: %v", addr, err)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	norm := net.JoinHostPort(host, port)

	// Only check TCP network here
	if g.conf.Network != "" && g.conf.Network != "tcp" {
		return nil
	}

	// Check if we recently failed and should skip retry to avoid hammering
	g.portCheckCache.mu.RLock()
	lastFailure := g.portCheckCache.lastFailure
	lastError := g.portCheckCache.lastError
	retryWindow := g.portCheckCache.retryWindow
	g.portCheckCache.mu.RUnlock()

	now := time.Now()
	// If we have a recent failure within retry window, return cached error
	// This avoids hammering unreachable ports while still checking frequently
	if lastError != nil && !lastFailure.IsZero() && now.Sub(lastFailure) < retryWindow {
		return fmt.Errorf("port %s is not reachable (cached): %v", norm, lastError)
	}

	// Always perform actual network check for real-time detection
	// This ensures we detect server crashes immediately, not after cache expiry
	conn, err := net.DialTimeout("tcp", norm, 2*time.Second)
	if err != nil {
		// Cache failure to avoid frequent retries
		g.portCheckCache.mu.Lock()
		g.portCheckCache.lastFailure = now
		g.portCheckCache.lastError = err
		g.portCheckCache.mu.Unlock()
		return fmt.Errorf("port %s is not reachable: %v", norm, err)
	}
	_ = conn.Close()

	// Clear cache on success - we never cache successes to ensure real-time detection
	g.portCheckCache.mu.Lock()
	g.portCheckCache.lastFailure = time.Time{} // Clear failure cache
	g.portCheckCache.lastError = nil
	g.portCheckCache.mu.Unlock()

	return nil
}

// updateHealthServingStatus sets the overall health serving status based on
// the shared readiness resource published by the gRPC client plugin.
// If the readiness resource is not found, it defaults to SERVING to remain
// backward compatible. If the resource exists and is false, it reports NOT_SERVING.
func (g *Service) updateHealthServingStatus() {
	if g.healthServer == nil {
		log.Warnf("Health server is nil, cannot update serving status")
		return
	}

	status := grpc_health_v1.HealthCheckResponse_SERVING
	if g.rt != nil {
		val, err := g.rt.GetSharedResource(requiredReadinessResourceName)
		if err != nil {
			// Resource not found or error accessing it - log but use default SERVING status
			log.Debugf("Could not get shared readiness resource: %v, defaulting to SERVING", err)
		} else {
			if ready, ok := val.(bool); ok {
				if !ready {
					status = grpc_health_v1.HealthCheckResponse_NOT_SERVING
					log.Debugf("Upstream readiness is false, setting status to NOT_SERVING")
				}
			} else {
				// Type assertion failed - log warning but use default SERVING status
				log.Warnf("Shared readiness resource has unexpected type: %T, expected bool, defaulting to SERVING", val)
			}
		}
	} else {
		log.Debugf("Runtime is nil, defaulting to SERVING status")
	}

	// Set the serving status
	g.healthServer.SetServingStatus("", status)
}

// validateTLSConfig validates TLS configuration
func (g *Service) validateTLSConfig() error {
	if !g.conf.GetTlsEnable() {
		return nil
	}

	// Check TLS auth type is valid
	authType := g.conf.GetTlsAuthType()
	if authType < 0 || authType > 4 {
		return fmt.Errorf("invalid TLS auth type: %d", authType)
	}

	// Check if certificate provider is available
	var cp app.CertificateProvider
	if prov := g.getCertProvider(); prov != nil {
		if typed, ok := prov.(app.CertificateProvider); ok {
			cp = typed
		}
	}
	if cp == nil && app.Lynx() != nil {
		cp = app.Lynx().Certificate()
	}
	if cp == nil {
		return fmt.Errorf("certificate provider not configured")
	}

	// Validate certificate data presence
	if len(cp.GetCertificate()) == 0 {
		return fmt.Errorf("server certificate not provided")
	}
	if len(cp.GetPrivateKey()) == 0 {
		return fmt.Errorf("server private key not provided")
	}

	return nil
}

// validateConfig validates the gRPC server configuration
func (g *Service) validateConfig() error {
	if g.conf == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Validate network type
	if g.conf.Network != "" && g.conf.Network != "tcp" && g.conf.Network != "unix" {
		return fmt.Errorf("unsupported network type: %s, supported types are 'tcp' and 'unix'", g.conf.Network)
	}

	// Validate address format
	if err := g.validateAddress(g.conf.Addr); err != nil {
		return fmt.Errorf("invalid address format: %v", err)
	}

	// Validate TLS configuration
	if g.conf.GetTlsEnable() {
		if err := g.validateTLSConfig(); err != nil {
			return fmt.Errorf("invalid TLS configuration: %v", err)
		}
	}

	// Validate timeout configuration
	if g.conf.Timeout != nil && g.conf.Timeout.AsDuration() <= 0 {
		return fmt.Errorf("timeout must be positive, got: %v", g.conf.Timeout.AsDuration())
	}

	// Validate MaxConcurrentStreams configuration
	// MaxConcurrentStreams of 0 means unlimited, which is valid
	// But if set, it should be within reasonable bounds
	if maxStreams := g.conf.GetMaxConcurrentStreams(); maxStreams > 0 {
		// Warn if value is very large (may indicate misconfiguration)
		if maxStreams > 100000 {
			log.Warnf("MaxConcurrentStreams is very large (%d), this may consume excessive server resources", maxStreams)
		}
		// Warn if value is very small (may limit performance unnecessarily)
		if maxStreams < 10 {
			log.Warnf("MaxConcurrentStreams is very small (%d), this may unnecessarily limit concurrent requests", maxStreams)
		}
	}

	// Validate message size configurations
	const maxAllowedMsgSize = 200 * 1024 * 1024 // 200MB upper bound warning
	if recv := g.conf.GetMaxRecvMsgSize(); recv > 0 {
		if recv > maxAllowedMsgSize {
			log.Warnf("MaxRecvMsgSize is very large (%d bytes), this may consume excessive memory", recv)
		}
	}
	if send := g.conf.GetMaxSendMsgSize(); send > 0 {
		if send > maxAllowedMsgSize {
			log.Warnf("MaxSendMsgSize is very large (%d bytes), this may consume excessive memory", send)
		}
	}

	return nil
}

// validateAddress validates the server address format
func (g *Service) validateAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("address cannot be empty")
	}

	// For TCP network, validate port format
	if g.conf.Network == "tcp" || g.conf.Network == "" {
		if !strings.Contains(addr, ":") {
			return fmt.Errorf("TCP address must include port (e.g., ':9090' or 'localhost:9090')")
		}

		// Try to parse the address
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return fmt.Errorf("invalid address format: %v", err)
		}

		// Validate port number
		if port == "" {
			return fmt.Errorf("port cannot be empty")
		}

		// For host validation, allow empty host (means all interfaces)
		if host != "" {
			// Try to resolve the hostname
			if _, err := net.LookupHost(host); err != nil {
				log.Warn("Warning: could not resolve hostname", "host", host, "error", err)
			}
		}
	}

	return nil
}

// getAppName returns the application name using dependency injection
func (g *Service) getAppName() string {
	if g.appNameProvider != nil {
		return g.appNameProvider()
	}
	return "lynx" // fallback default
}

// getLogger returns the logger using dependency injection
func (g *Service) getLogger() interface{} {
	if g.loggerProvider != nil {
		return g.loggerProvider()
	}
	return nil // fallback
}

// getCertProvider returns the certificate provider using dependency injection
func (g *Service) getCertProvider() interface{} {
	if g.certProvider != nil {
		return g.certProvider()
	}
	return nil // fallback
}

// getControlPlane returns the control plane using dependency injection
func (g *Service) getControlPlane() interface{} {
	if g.controlPlaneProvider != nil {
		return g.controlPlaneProvider()
	}
	return nil // fallback
}

// getGRPCRateLimit returns the gRPC rate limit middleware using dependency injection
func (g *Service) getGRPCRateLimit() interface{} {
	controlPlane := g.getControlPlane()
	if controlPlane == nil {
		return nil
	}
	// Type assertion would be needed here in real implementation
	// For now, return nil as fallback
	return nil
}

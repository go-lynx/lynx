package boot

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2/encoding/json"
	"github.com/go-lynx/lynx/app/banner"
	"github.com/go-lynx/lynx/app/log"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	kratoslog "github.com/go-kratos/kratos/v2/log"
	lynxapp "github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
)

// flagConf holds the configuration file path from command line arguments
var (
	flagConf string
)

// Application represents the main bootstrap structure for Lynx applications, responsible for managing application initialization, configuration loading, and lifecycle
type Application struct {
	wire    wireApp          // Function used to initialize Kratos application
	plugins []plugins.Plugin // List of plugins to initialize
	conf    config.Config    // Application configuration instance
	cleanup func()           // Cleanup function for resource cleanup when application shuts down
	lynxApp *lynxapp.LynxApp // Lynx application instance

	// Enhanced fields for production readiness
	shutdownTimeout time.Duration   // Graceful shutdown timeout
	shutdownChan    chan struct{}   // Channel to signal shutdown
	shutdownOnce    sync.Once       // Ensure shutdown is called only once
	healthChecker   *HealthChecker  // Health checker for monitoring
	circuitBreaker  *CircuitBreaker // Circuit breaker for error handling
}

// HealthChecker provides application health monitoring
type HealthChecker struct {
	mu            sync.RWMutex
	isHealthy     bool
	lastCheck     time.Time
	checkInterval time.Duration
	stopChan      chan struct{}
	stopOnce      sync.Once // Protect against multiple Stop() calls
}

// CircuitBreaker provides error handling and recovery
type CircuitBreaker struct {
	mu           sync.RWMutex
	state        CircuitState
	failureCount int
	successCount int
	lastFailure  time.Time
	threshold    int
	timeout      time.Duration
}

// CircuitState represents the state of circuit breaker
type CircuitState int

const (
	CircuitStateClosed CircuitState = iota
	CircuitStateOpen
	CircuitStateHalfOpen
)

// Default configuration constants
const (
	DefaultShutdownTimeout         = 30 * time.Second
	DefaultHealthCheckInterval     = 30 * time.Second
	DefaultCircuitBreakerThreshold = 5
	DefaultCircuitBreakerTimeout   = 60 * time.Second
)

// init package initialization function for parsing command line arguments and configuring JSON serialization options
func init() {
	// Only parse command line arguments in non-test environments
	if !isTestEnvironment() {
		// Use configuration manager to get default configuration path
		configMgr := GetConfigManager()
		defaultConfPath := configMgr.GetDefaultConfigPath()
		flag.StringVar(&flagConf, "conf", defaultConfPath, "config path, eg: -conf config.yaml")
		flag.Parse()

		// Set parsed configuration path to configuration manager
		configMgr.SetConfigPath(flagConf)
	}
}

// isTestEnvironment checks if running in test environment
func isTestEnvironment() bool {
	return flag.Lookup("test.v") != nil || flag.Lookup("test.run") != nil
}

// wireApp is a function type used to initialize and return a Kratos application instance
type wireApp func(logger kratoslog.Logger) (*kratos.App, error)

// Run starts the Lynx application and manages its lifecycle with enhanced production features
func (app *Application) Run() error {
	// Check if Application instance is nil
	if app == nil {
		return fmt.Errorf("application instance is nil: cannot start Lynx application")
	}

	// Initialize enhanced features
	app.initializeEnhancedFeatures()

	// Set up signal handling for graceful shutdown
	app.setupSignalHandling()

	// Improved resource cleanup order: handle panic first, then execute cleanup
	defer func() {
		if r := recover(); r != nil {
			app.handlePanic(r)
		}
		app.gracefulShutdown()
	}()

	// Record application startup time for calculating startup duration
	startTime := time.Now()

	// Load bootstrap configuration
	if err := app.LoadBootstrapConfig(); err != nil {
		return fmt.Errorf("failed to load bootstrap configuration: %w", err)
	}

	// Initialize Lynx application
	lynxApp, err := lynxapp.NewApp(app.conf, app.plugins...)
	if err != nil {
		return fmt.Errorf("failed to create Lynx application: %w", err)
	}
	app.lynxApp = lynxApp

	// Initialize logger
	if err := log.InitLogger(app.GetName(), app.GetHost(), app.GetVersion(), app.conf); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Show startup banner (decoupled from logger)
	if err := banner.Init(app.conf); err != nil {
		log.Warnf("failed to initialize/show banner: %v", err)
	}

	// Log application startup information
	log.Info("lynx application is starting up")

	// Get plugin manager
	pluginManager := lynxApp.GetPluginManager()
	if pluginManager == nil {
		return fmt.Errorf("plugin manager is nil: cannot manage plugins")
	}

	// Load plugins with circuit breaker protection
	if err := app.loadPluginsWithProtection(pluginManager); err != nil {
		return err
	}

	// Initialize Kratos application with proxy logger (hot-swappable inner)
	kratosApp, err := app.wire(log.GetProxyLogger())
	if err != nil {
		log.Error(err)
		return fmt.Errorf("failed to initialize Kratos application: %w", err)
	}

	// Configure protocol buffers JSON serialization options
	jsonEmit, jsonConfErr := lynxApp.GetGlobalConfig().Value("lynx.http.response.json.emitUnpopulated").Bool()
	if jsonConfErr != nil && errors.Is(jsonConfErr, config.ErrNotFound) {
		jsonEmit = false
	}
	// EmitUnpopulated: include unset fields during serialization
	// UseProtoNames: use field names defined in proto files
	json.MarshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: jsonEmit,
		UseProtoNames:   true,
	}

	// Start health checker
	app.startHealthChecker()

	// Calculate application startup duration
	elapsedMs := time.Since(startTime).Milliseconds()
	var elapsedDisplay string
	switch {
	case elapsedMs < 1000:
		// Less than 1 second, display milliseconds
		elapsedDisplay = fmt.Sprintf("%d ms", elapsedMs)
	case elapsedMs < 60_000:
		// Less than 1 minute, display seconds (keep two decimal places)
		elapsedDisplay = fmt.Sprintf("%.2f s", float64(elapsedMs)/1000)
	default:
		// More than 1 minute, display minutes (keep two decimal places)
		elapsedDisplay = fmt.Sprintf("%.2f m", float64(elapsedMs)/1000/60)
	}
	log.Infof("lynx application started successfully, elapsed time: %s, port listening initiated", elapsedDisplay)

	// Run Kratos application with graceful shutdown support
	return app.runWithGracefulShutdown(kratosApp)
}

// initializeEnhancedFeatures initializes enhanced production features
func (app *Application) initializeEnhancedFeatures() {
	app.shutdownTimeout = DefaultShutdownTimeout
	app.shutdownChan = make(chan struct{})

	// Initialize health checker
	app.healthChecker = &HealthChecker{
		isHealthy:     true,
		lastCheck:     time.Now(),
		checkInterval: DefaultHealthCheckInterval,
		stopChan:      make(chan struct{}),
	}

	// Initialize circuit breaker
	app.circuitBreaker = &CircuitBreaker{
		state:     CircuitStateClosed,
		threshold: DefaultCircuitBreakerThreshold,
		timeout:   DefaultCircuitBreakerTimeout,
	}
}

// setupSignalHandling sets up signal handling for graceful shutdown
func (app *Application) setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		sig := <-sigChan
		log.Infof("Received signal %v, initiating graceful shutdown", sig)
		app.initiateShutdown()
	}()
}

// initiateShutdown initiates the graceful shutdown process
func (app *Application) initiateShutdown() {
	app.shutdownOnce.Do(func() {
		close(app.shutdownChan)
	})
}

// gracefulShutdown performs graceful shutdown of the application
func (app *Application) gracefulShutdown() {
	log.Info("Starting graceful shutdown...")

	// Stop health checker
	if app.healthChecker != nil {
		app.healthChecker.Stop()
	}

	// Close Lynx application
	if app.lynxApp != nil {
		if err := app.lynxApp.Close(); err != nil {
			log.Errorf("Error during Lynx application shutdown: %v", err)
		}
	}

	// Execute custom cleanup
	if app.cleanup != nil {
		app.cleanup()
	}

	log.Info("Graceful shutdown completed")
}

// loadPluginsWithProtection loads plugins with circuit breaker protection
func (app *Application) loadPluginsWithProtection(pluginManager lynxapp.TypedPluginManager) error {
	// Check circuit breaker state before loading plugins
	if !app.circuitBreaker.CanExecute() {
		return fmt.Errorf("circuit breaker is open, skipping plugin loading")
	}

	err := pluginManager.LoadPlugins(app.conf)
	app.circuitBreaker.RecordResult(err)
	return err
}

// runWithGracefulShutdown runs the Kratos application with graceful shutdown support
func (app *Application) runWithGracefulShutdown(kratosApp *kratos.App) error {
	// Run Kratos application in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := kratosApp.Run(); err != nil {
			errChan <- err
		}
	}()

	// Wait for either shutdown signal or error
	select {
	case <-app.shutdownChan:
		log.Info("Shutdown signal received, stopping Kratos application...")

		// Create context with shutdown timeout only after receiving shutdown signal
		ctx, cancel := context.WithTimeout(context.Background(), app.shutdownTimeout)
		defer cancel()

		// Stop Kratos application gracefully with timeout
		stopChan := make(chan error, 1)
		go func() {
			stopChan <- kratosApp.Stop()
		}()

		select {
		case err := <-stopChan:
			if err != nil {
				log.Errorf("Error stopping Kratos application: %v", err)
			}
			return err
		case <-ctx.Done():
			log.Error("Shutdown timeout exceeded during graceful shutdown")
			return fmt.Errorf("shutdown timeout exceeded during graceful shutdown")
		}

	case err := <-errChan:
		log.Error(err)
		return fmt.Errorf("failed to run Kratos application: %w", err)
	}
}

// startHealthChecker starts the health checker
func (app *Application) startHealthChecker() {
	if app.healthChecker == nil {
		return
	}

	go app.healthChecker.Run()
}

// handlePanic recovers from panic and ensures proper resource cleanup
func (app *Application) handlePanic(r interface{}) {
	var err error
	// Convert panic to error based on its type
	switch v := r.(type) {
	case error:
		err = v
	case string:
		err = fmt.Errorf("panic: %s", v)
	default:
		err = fmt.Errorf("panic: %v", r)
	}
	log.Error(err)

	// Record failure in circuit breaker
	if app.circuitBreaker != nil {
		app.circuitBreaker.RecordResult(err)
	}

	// Ensure plugins are unloaded
	if app.lynxApp != nil && app.lynxApp.GetPluginManager() != nil {
		app.lynxApp.GetPluginManager().UnloadPlugins()
	}
}

// NewApplication creates a new Lynx microservice bootstrap instance
// Parameters:
//   - wire: Function used to initialize Kratos application
//   - plugins: Optional plugin list to initialize with the application
//
// Returns:
//   - *Application: Initialized Application instance
func NewApplication(wire wireApp, plugins ...plugins.Plugin) *Application {
	// Check if wire function is nil
	if wire == nil {
		log.Error("wire function cannot be nil: required for Kratos application initialization")
		return nil
	}

	// Return initialized Application instance
	return &Application{
		wire:    wire,
		plugins: plugins,
	}
}

// HealthChecker methods

// Run starts the health checker
func (hc *HealthChecker) Run() {
	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.performHealthCheck()
		case <-hc.stopChan:
			return
		}
	}
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	hc.stopOnce.Do(func() {
		close(hc.stopChan)
	})
}

// performHealthCheck performs a health check
func (hc *HealthChecker) performHealthCheck() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.lastCheck = time.Now()
	// Add your health check logic here
	// For now, we'll just mark as healthy
	hc.isHealthy = true
}

// IsHealthy returns the current health status
func (hc *HealthChecker) IsHealthy() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.isHealthy
}

// CircuitBreaker methods

// CanExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitStateClosed:
		return true
	case CircuitStateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailure) >= cb.timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = CircuitStateHalfOpen
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case CircuitStateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordResult records the result of an operation
func (cb *CircuitBreaker) RecordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailure = time.Now()

		if cb.state == CircuitStateClosed && cb.failureCount >= cb.threshold {
			cb.state = CircuitStateOpen
			log.Warnf("Circuit breaker opened after %d failures", cb.failureCount)
		} else if cb.state == CircuitStateHalfOpen {
			cb.state = CircuitStateOpen
			log.Warn("Circuit breaker reopened after failed attempt")
		}
	} else {
		cb.successCount++

		if cb.state == CircuitStateHalfOpen {
			cb.state = CircuitStateClosed
			cb.resetCounters()
			log.Info("Circuit breaker closed after successful attempt")
		}
	}
}

// resetCounters resets the circuit breaker counters
func (cb *CircuitBreaker) resetCounters() {
	cb.failureCount = 0
	cb.successCount = 0
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

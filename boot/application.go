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
	"github.com/go-lynx/lynx/internal/banner"
	"github.com/go-lynx/lynx/log"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	lynxapp "github.com/go-lynx/lynx"
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
	stopOnce      sync.Once    // Protect against multiple Stop() calls
	app           *Application // Reference to application for health checks
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
type wireApp func() (*kratos.App, error)

// Run starts the Lynx application and manages its lifecycle with enhanced production features
func (app *Application) Run() error {
	// Check if Application instance is nil
	if app == nil {
		return fmt.Errorf("application instance is nil: cannot start Lynx application")
	}

	// Initialize enhanced features
	app.initializeEnhancedFeatures()

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

	// Initialize logger (must be done before signal handling to allow logging)
	if err := log.InitLogger(app.GetName(), app.GetHost(), app.GetVersion(), app.conf); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Set up signal handling for graceful shutdown (after logger is initialized)
	app.setupSignalHandling()

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
	// Note: Control plane plugins (Apollo/Polaris) may call LoadPlugins() in their StartupTasks()
	// to load plugins from remote configuration. This initial LoadPlugins() loads plugins from
	// local bootstrap configuration, which typically includes the control plane plugin itself.
	// The preparePlugin() method has built-in deduplication logic to prevent loading the same
	// plugin twice, so it's safe to call LoadPlugins() multiple times.
	if err := app.loadPluginsWithProtection(pluginManager); err != nil {
		return err
	}

	// Initialize Kratos application with proxy logger (hot-swappable inner)
	kratosApp, err := app.wire()
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

	// Start health checker after plugins are loaded
	// This ensures health checks can properly verify plugin states
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
		app:           app, // Store reference for health checks
	}

	// Initialize circuit breaker
	app.circuitBreaker = &CircuitBreaker{
		state:     CircuitStateClosed,
		threshold: DefaultCircuitBreakerThreshold,
		timeout:   DefaultCircuitBreakerTimeout,
	}
}

// setupSignalHandling sets up signal handling for graceful shutdown
// Note: This should be called after logger is initialized to ensure proper logging
func (app *Application) setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		sig := <-sigChan
		// Logger should be initialized by now
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
// Shutdown order: health checker -> plugins -> application -> cleanup -> loggers
func (app *Application) gracefulShutdown() {
	// Protect against panic during shutdown
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic during graceful shutdown: %v", r)
		}
	}()

	log.Info("Starting graceful shutdown...")

	// Step 1: Stop health checker first to prevent new health checks during shutdown
	if app.healthChecker != nil {
		// Protect against panic when stopping health checker
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("Panic when stopping health checker: %v", r)
				}
			}()
			app.healthChecker.Stop()
		}()
	}

	// Step 2: Close Lynx application (this will unload plugins in reverse dependency order)
	if app.lynxApp != nil {
		// Protect against panic when closing application
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("Panic when closing Lynx application: %v", r)
				}
			}()
			if err := app.lynxApp.Close(); err != nil {
				log.Errorf("Error during Lynx application shutdown: %v", err)
			}
		}()
	}

	// Step 3: Execute custom cleanup functions
	if app.cleanup != nil {
		// Protect against panic in custom cleanup
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("Panic in custom cleanup: %v", r)
				}
			}()
			app.cleanup()
		}()
	}

	// Step 4: Cleanup loggers and close all writers (should be last)
	// Protect against panic when cleaning up loggers
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Use fmt.Printf as fallback since logger may be closed
				fmt.Printf("Panic when cleaning up loggers: %v\n", r)
			}
		}()
		log.CleanupLoggers()
	}()

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
		defer func() {
			// Recover from panic in Kratos application
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic in Kratos application: %v", r)
			}
		}()
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
			defer func() {
				// Recover from panic during stop
				if r := recover(); r != nil {
					stopChan <- fmt.Errorf("panic during Kratos application stop: %v", r)
				}
			}()
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
		// Initiate shutdown on error to ensure cleanup
		app.initiateShutdown()
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
	// Protect against panic in health check
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic in health check: %v", r)
			// Mark as unhealthy on panic
			hc.mu.Lock()
			hc.isHealthy = false
			hc.mu.Unlock()
		}
	}()

	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.lastCheck = time.Now()

	// Enhanced health check: check application and plugin states
	healthy := true

	// Check if application is initialized
	if hc.app != nil && hc.app.lynxApp != nil {
		// Check plugin manager
		pluginManager := hc.app.lynxApp.GetPluginManager()
		if pluginManager != nil {
			// Check for unload failures
			failures := pluginManager.GetUnloadFailures()
			if len(failures) > 0 {
				// Recent failures (within last 5 minutes) indicate unhealthy state
				recentFailures := 0
				fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
				for _, failure := range failures {
					if failure.FailureTime.After(fiveMinutesAgo) {
						recentFailures++
					}
				}
				if recentFailures > 0 {
					log.Warnf("Health check: %d recent plugin unload failures detected", recentFailures)
					healthy = false
				}
			}

			// Check resource stats for potential leaks
			resourceStats := pluginManager.GetResourceStats()
			if resourceStats != nil {
				// Check if resource count is reasonable (threshold: 1000 resources)
				// Use type assertion with error handling for robustness
				if totalResourcesVal, exists := resourceStats["total_resources"]; exists {
					var totalResources int
					switch v := totalResourcesVal.(type) {
					case int:
						totalResources = v
					case int32:
						totalResources = int(v)
					case int64:
						totalResources = int(v)
					default:
						// Try to convert if possible
						if intVal, ok := totalResourcesVal.(int); ok {
							totalResources = intVal
						} else {
							log.Debugf("Health check: unexpected type for total_resources: %T", totalResourcesVal)
							break
						}
					}
					if totalResources > 1000 {
						log.Warnf("Health check: high resource count detected: %d (potential leak)", totalResources)
						// Don't mark as unhealthy, just warn - could be legitimate high usage
					}
				}

				// Check resource size for potential memory leaks
				if totalSizeVal, exists := resourceStats["total_size_bytes"]; exists {
					var totalSize int64
					switch v := totalSizeVal.(type) {
					case int64:
						totalSize = v
					case int:
						totalSize = int64(v)
					case int32:
						totalSize = int64(v)
					default:
						// Try to convert if possible
						if int64Val, ok := totalSizeVal.(int64); ok {
							totalSize = int64Val
						} else {
							log.Debugf("Health check: unexpected type for total_size_bytes: %T", totalSizeVal)
							break
						}
					}
					if totalSize > 0 {
						// Threshold: 100MB
						const maxSizeBytes = 100 * 1024 * 1024
						if totalSize > maxSizeBytes {
							log.Warnf("Health check: large resource size detected: %d bytes (%.2f MB) - potential memory leak",
								totalSize, float64(totalSize)/(1024*1024))
							// Don't mark as unhealthy, just warn
						}
					}
				}
			}
		}
	}

	hc.isHealthy = healthy
}

// IsHealthy returns the current health status
func (hc *HealthChecker) IsHealthy() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.isHealthy
}

// CircuitBreaker methods

// CanExecute checks if the circuit breaker allows execution.
// Open->HalfOpen transition is done under exclusive lock to avoid multiple goroutines allowing execution.
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitStateClosed:
		return true
	case CircuitStateOpen:
		if time.Since(cb.lastFailure) >= cb.timeout {
			cb.state = CircuitStateHalfOpen
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
		} else if cb.state == CircuitStateClosed {
			// Reset counters periodically in closed state to prevent unbounded growth
			// Reset after a reasonable number of successes to maintain recent history
			const resetThreshold = 1000
			if cb.successCount >= resetThreshold {
				cb.resetCounters()
			}
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

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
	flagConf                   string
	registerBootstrapFlagsOnce sync.Once
)

// Application is an optional bootstrap shell around Lynx core.
// It owns process-level concerns such as signal handling, startup sequencing,
// and Kratos integration. It should not be treated as part of the plugin
// orchestration core itself.
type Application struct {
	wire              wireApp          // Function used to initialize Kratos application
	plugins           []plugins.Plugin // List of plugins to initialize
	conf              config.Config    // Application configuration instance
	cleanup           func()           // Cleanup function for resource cleanup when application shuts down
	lynxApp           *lynxapp.LynxApp // Lynx application instance
	configPath        string           // Optional instance-scoped bootstrap config path
	publishDefaultApp bool             // Whether boot publishes the created app as process-wide default

	// Enhanced fields for production readiness
	shutdownTimeout time.Duration           // Graceful shutdown timeout
	shutdownChan    chan struct{}           // Channel to signal shutdown
	shutdownOnce    sync.Once               // Ensure shutdown is called only once
	healthChecker   *HealthChecker          // Health checker for monitoring
	circuitBreaker  *lynxapp.CircuitBreaker // Circuit breaker for error handling
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

// Default configuration constants
const (
	DefaultShutdownTimeout         = 30 * time.Second
	DefaultHealthCheckInterval     = 30 * time.Second
	DefaultCircuitBreakerThreshold = 5
	DefaultCircuitBreakerTimeout   = 60 * time.Second
)

// RegisterBootstrapFlags registers boot compatibility flags on the provided flag set.
// Host processes that parse flags before calling Run should invoke this explicitly.
// The registration is idempotent for the process-wide default FlagSet.
func RegisterBootstrapFlags(fs *flag.FlagSet) {
	if fs == nil {
		fs = flag.CommandLine
	}

	if existing := fs.Lookup("conf"); existing != nil {
		if existing.Value != nil {
			flagConf = existing.Value.String()
		}
		return
	}

	configMgr := GetConfigManager()
	defaultConfPath := configMgr.GetDefaultConfigPath()
	fs.StringVar(&flagConf, "conf", defaultConfPath, "config path, eg: -conf config.yaml")
}

func ensureBootstrapFlagsRegistered() {
	if isTestEnvironment() {
		return
	}
	registerBootstrapFlagsOnce.Do(func() {
		RegisterBootstrapFlags(flag.CommandLine)
	})
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
	lynxApp, err := lynxapp.NewStandaloneApp(app.conf, app.plugins...)
	if err != nil {
		return fmt.Errorf("failed to create Lynx application: %w", err)
	}
	if err := app.initializeRuntimeShell(lynxApp); err != nil {
		return err
	}

	// Load plugins with circuit breaker protection
	// Note: Control plane plugins (Apollo/Polaris) may call LoadPlugins() in their StartupTasks()
	// to load plugins from remote configuration. This initial LoadPlugins() loads plugins from
	// local bootstrap configuration, which typically includes the control plane plugin itself.
	// The preparePlugin() method has built-in deduplication logic to prevent loading the same
	// plugin twice, so it's safe to call LoadPlugins() multiple times.
	if err := app.loadPluginsWithProtection(); err != nil {
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

	log.Infof("lynx application started successfully, elapsed time: %s, port listening initiated", formatStartupElapsed(time.Since(startTime)))

	// Run Kratos application with graceful shutdown support
	return app.runWithGracefulShutdown(kratosApp)
}

func (app *Application) initializeRuntimeShell(lynxApp *lynxapp.LynxApp) error {
	if app == nil {
		return fmt.Errorf("application instance is nil: cannot initialize runtime shell")
	}
	if lynxApp == nil {
		return fmt.Errorf("lynx application is nil: cannot initialize runtime shell")
	}

	app.publishAppIfConfigured(lynxApp)
	app.lynxApp = lynxApp

	if err := log.InitLogger(app.GetName(), app.GetHost(), app.GetVersion(), app.conf); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	app.setupSignalHandling()

	if err := banner.Init(app.conf); err != nil {
		log.Warnf("failed to initialize/show banner: %v", err)
	}

	log.Info("lynx application is starting up")

	if lynxApp.GetPluginManager() == nil {
		return fmt.Errorf("plugin manager is nil: cannot manage plugins")
	}

	return nil
}

func (app *Application) publishAppIfConfigured(lynxApp *lynxapp.LynxApp) {
	if app == nil || lynxApp == nil || !app.publishDefaultApp {
		return
	}
	lynxapp.SetDefaultApp(lynxApp)
}

func formatStartupElapsed(elapsed time.Duration) string {
	elapsedMs := elapsed.Milliseconds()
	switch {
	case elapsedMs < 1000:
		return fmt.Sprintf("%d ms", elapsedMs)
	case elapsedMs < 60_000:
		return fmt.Sprintf("%.2f s", float64(elapsedMs)/1000)
	default:
		return fmt.Sprintf("%.2f m", float64(elapsedMs)/1000/60)
	}
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
	app.circuitBreaker = lynxapp.NewCircuitBreaker(DefaultCircuitBreakerThreshold, DefaultCircuitBreakerTimeout)
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

func (app *Application) protectShutdownStep(stepName string, fn func()) {
	if fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic in %s: %v", stepName, r)
		}
	}()
	fn()
}

func protectLoggerCleanup(fn func()) {
	if fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic when cleaning up loggers: %v\n", r)
		}
	}()
	fn()
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
		app.protectShutdownStep("stopping health checker", func() {
			app.healthChecker.Stop()
		})
	}

	// Step 2: Close Lynx application (this will unload plugins in reverse dependency order)
	if app.lynxApp != nil {
		app.protectShutdownStep("closing Lynx application", func() {
			if err := app.lynxApp.Close(); err != nil {
				log.Errorf("Error during Lynx application shutdown: %v", err)
			}
		})
	}

	// Step 3: Execute custom cleanup functions
	if app.cleanup != nil {
		app.protectShutdownStep("custom cleanup", func() {
			app.cleanup()
		})
	}

	log.Info("Graceful shutdown completed")

	// Step 4: Cleanup loggers and close all writers (should be last)
	protectLoggerCleanup(func() {
		log.CleanupLoggers()
	})
}

// loadPluginsWithProtection loads plugins with circuit breaker protection
func (app *Application) loadPluginsWithProtection() error {
	// Check circuit breaker state before loading plugins
	if !app.circuitBreaker.CanExecute() {
		return fmt.Errorf("circuit breaker is open, skipping plugin loading")
	}

	if app.lynxApp == nil {
		return fmt.Errorf("lynx application is nil: cannot load plugins")
	}

	err := app.lynxApp.LoadPlugins()
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

		return app.stopKratosAppWithTimeout(kratosApp)

	case err := <-errChan:
		log.Error(err)
		// Initiate shutdown on error to ensure cleanup
		app.initiateShutdown()
		return fmt.Errorf("failed to run Kratos application: %w", err)
	}
}

func (app *Application) stopKratosAppWithTimeout(kratosApp *kratos.App) error {
	ctx, cancel := context.WithTimeout(context.Background(), app.shutdownTimeout)
	defer cancel()

	stopChan := make(chan error, 1)
	go func() {
		defer func() {
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
}

// startHealthChecker starts the health checker
func (app *Application) startHealthChecker() {
	if app.healthChecker == nil {
		return
	}

	go app.healthChecker.Run()
}

// handlePanic recovers from panic and ensures proper resource cleanup
func (app *Application) handlePanic(r any) {
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
		wire:              wire,
		plugins:           plugins,
		publishDefaultApp: true,
	}
}

// SetConfigPath binds a bootstrap config path to this Application instance.
// Instance-scoped paths take precedence over process-wide config manager state.
func (app *Application) SetConfigPath(path string) *Application {
	if app == nil {
		return nil
	}
	app.configPath = path
	return app
}

// SetPublishDefaultApp controls whether Run publishes the created app as the
// process-wide default Lynx application instance.
func (app *Application) SetPublishDefaultApp(enabled bool) *Application {
	if app == nil {
		return nil
	}
	app.publishDefaultApp = enabled
	return app
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

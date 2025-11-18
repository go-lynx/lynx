// Package app provides core application functionality for the Lynx framework
package app

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/app/conf"
	"github.com/go-lynx/lynx/app/events"
	"github.com/go-lynx/lynx/app/log"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/plugins"
	"google.golang.org/grpc"
)

var (
	// lynxApp is the singleton instance of the Lynx application
	lynxApp *LynxApp
	// initState tracks initialization state using atomic operations for better control
	// 0 = not initialized, 1 = initializing, 2 = initialized, 3 = failed
	initState int32
	// RW mutex protecting reads/writes of lynxApp to avoid race conditions
	lynxMu sync.RWMutex
	// initErr stores initialization error for retry mechanism
	initErr error
	// initMu protects initErr and allows reset on failure
	initMu sync.Mutex
)

const (
	initStateNotInitialized = iota
	initStateInitializing
	initStateInitialized
	initStateFailed
)

// Fixed plugin ID used internally for configuration-related events.
// Avoids using an empty string which would break PluginID-based filtering.
const configEventPluginID = "lynx.config"

// LynxApp represents the main application instance.
// It serves as the central coordinator for all application components,
// managing configuration, logging, plugins, and the control plane.
type LynxApp struct {
	// host represents the application's host address.
	// Used for network communication and service registration.
	host string

	// name is the unique identifier of the application.
	// Used for service discovery and logging.
	name string

	// version represents the application's version number.
	// Used for compatibility checks and deployment management.
	version string

	// certificateProvider manages the application's TLS/SSL certificates.
	// Used for secure communication and TLS configuration.
	cert CertificateProvider

	// Bootstrap configuration
	bootConfig *conf.Bootstrap

	// globalConf holds the application's global configuration.
	// Contains settings that apply across all components.
	globalConf config.Config

	// controlPlane manages the application's control interface.
	// Handles dynamic configuration updates and system monitoring.
	controlPlane ControlPlane

	// pluginManager handles plugin lifecycle and dependencies.
	// Responsible for loading, unloading, and coordinating plugins.
	pluginManager TypedPluginManager

	// typedPluginManager handles typed plugin lifecycle and dependencies.
	// Provides type-safe plugin management with generic support.
	typedPluginManager TypedPluginManager

	// grpcSubs stores upstream gRPC connections subscribed via configuration; key is the service name
	grpcSubs map[string]*grpc.ClientConn

	// Configuration version (monotonically increasing) used for event ordering and idempotent handling
	configVersion uint64
}

// Lynx returns the global LynxApp instance.
// It ensures thread-safe access to the singleton instance.
func Lynx() *LynxApp {
	lynxMu.RLock()
	defer lynxMu.RUnlock()
	return lynxApp
}

// GetHost retrieves the hostname of the current application instance.
// Returns an empty string if the application is not initialized.
func GetHost() string {
	a := Lynx()
	if a == nil {
		return ""
	}
	return a.host
}

// GetName retrieves the application name.
// Returns an empty string if the application is not initialized.
func GetName() string {
	a := Lynx()
	if a == nil {
		return ""
	}
	return a.name
}

// GetVersion retrieves the application version.
// Returns an empty string if the application is not initialized.
func GetVersion() string {
	a := Lynx()
	if a == nil {
		return ""
	}
	return a.version
}

// NewApp creates a new Lynx application instance with the provided configuration and plugins.
// It initializes the application with system hostname and bootstrap configuration.
//
// Parameters:
//   - cfg: Configuration instance
//   - plugins: Optional list of plugins to initialize with
//
// Returns:
//   - *LynxApp: Initialized application instance
//   - error: Any error that occurred during initialization
func NewApp(cfg config.Config, plugins ...plugins.Plugin) (*LynxApp, error) {
	// Validate configuration is not nil; return error if nil
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	// Check if already initialized successfully
	lynxMu.RLock()
	existing := lynxApp
	lynxMu.RUnlock()
	if existing != nil {
		return existing, nil
	}

	// Check initialization state atomically
	state := atomic.LoadInt32(&initState)
	if state == initStateInitialized {
		// Already initialized successfully, return existing instance
		lynxMu.RLock()
		existing := lynxApp
		lynxMu.RUnlock()
		if existing != nil {
			return existing, nil
		}
	}

	// Try to acquire initialization lock
	acquired := atomic.CompareAndSwapInt32(&initState, initStateNotInitialized, initStateInitializing) ||
		atomic.CompareAndSwapInt32(&initState, initStateFailed, initStateInitializing)

	if !acquired {
		// Another goroutine is initializing or already initialized
		// Wait for it to complete with timeout
		waitTimeout := 100 * 10 * time.Millisecond // 100 iterations * 10ms = 1 second
		deadline := time.Now().Add(waitTimeout)

		for time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
			state := atomic.LoadInt32(&initState)

			if state == initStateInitialized {
				// Initialization completed, check if app exists
				lynxMu.RLock()
				existing := lynxApp
				lynxMu.RUnlock()
				if existing != nil {
					return existing, nil
				}
				// App was initialized but is now nil (likely closed)
				// Return error instead of continuing to initialize
				return nil, fmt.Errorf("application was closed after initialization")
			}

			if state == initStateFailed {
				// Previous initialization failed, try to acquire lock for retry
				if atomic.CompareAndSwapInt32(&initState, initStateFailed, initStateInitializing) {
					acquired = true
					break
				}
			}

			// If state is still initializing, continue waiting
			// If state changed to not initialized (shouldn't happen), try to acquire
			if state == initStateNotInitialized {
				if atomic.CompareAndSwapInt32(&initState, initStateNotInitialized, initStateInitializing) {
					acquired = true
					break
				}
			}
		}

		// After waiting loop, check final state before proceeding
		// CRITICAL: Do not proceed to initializeApp() if we don't hold the lock
		if !acquired {
			finalState := atomic.LoadInt32(&initState)

			if finalState == initStateInitialized {
				// Another goroutine completed initialization
				lynxMu.RLock()
				existing := lynxApp
				lynxMu.RUnlock()
				if existing != nil {
					return existing, nil
				}
				// App was closed after initialization, return error
				return nil, fmt.Errorf("application was closed after initialization")
			}

			if finalState == initStateInitializing {
				// Another goroutine is still initializing
				// Wait a bit more to see if it completes
				time.Sleep(100 * time.Millisecond)
				finalState = atomic.LoadInt32(&initState)
				if finalState == initStateInitialized {
					lynxMu.RLock()
					existing := lynxApp
					lynxMu.RUnlock()
					if existing != nil {
						return existing, nil
					}
				}
				// Still initializing or failed - return error to prevent duplicate initialization
				return nil, fmt.Errorf("initialization timeout: another goroutine is still initializing")
			}

			// Final attempt to acquire lock (state might be failed or not initialized)
			if atomic.CompareAndSwapInt32(&initState, initStateNotInitialized, initStateInitializing) ||
				atomic.CompareAndSwapInt32(&initState, initStateFailed, initStateInitializing) {
				acquired = true
			} else {
				// Still can't acquire lock - another goroutine must have acquired it
				// Return error to prevent duplicate initialization
				return nil, fmt.Errorf("failed to acquire initialization lock: another goroutine is initializing")
			}
		}
	}

	// CRITICAL: Verify we hold the initialization lock before proceeding
	// Double-check to prevent race condition where lock was lost
	currentState := atomic.LoadInt32(&initState)
	if currentState != initStateInitializing {
		// We don't hold the lock - another goroutine must have it
		if currentState == initStateInitialized {
			lynxMu.RLock()
			existing := lynxApp
			lynxMu.RUnlock()
			if existing != nil {
				return existing, nil
			}
		}
		return nil, fmt.Errorf("initialization lock verification failed: state is %d, expected %d",
			currentState, initStateInitializing)
	}

	var app *LynxApp
	var err error

	// Perform initialization (we hold the lock at this point)
	app, err = initializeApp(cfg, plugins...)

	initMu.Lock()
	initErr = err
	initMu.Unlock()

	if err != nil {
		atomic.StoreInt32(&initState, initStateFailed)
	} else {
		atomic.StoreInt32(&initState, initStateInitialized)
	}

	// If initialization failed, return error
	if err != nil {
		// Store error for potential retry mechanism
		initMu.Lock()
		storedErr := initErr
		initMu.Unlock()
		if storedErr != nil {
			return nil, fmt.Errorf("failed to initialize application: %w (stored error: %v)", err, storedErr)
		}
		return nil, fmt.Errorf("failed to initialize application: %w", err)
	}

	// In concurrent scenarios: if app is nil but the singleton was initialized by another goroutine, return the singleton
	if app == nil {
		lynxMu.RLock()
		existing := lynxApp
		lynxMu.RUnlock()
		if existing != nil {
			return existing, nil
		}
	}

	// Return the new instance; if unexpectedly nil, return an explicit error
	if app == nil {
		return nil, fmt.Errorf("application initialization resulted in nil instance")
	}
	return app, nil
}

// initializeApp handles the actual initialization of the LynxApp instance.
func initializeApp(cfg config.Config, plugins ...plugins.Plugin) (*LynxApp, error) {
	// Parse bootstrap configuration
	var bConf conf.Bootstrap
	// Scan configuration into bConf; return error if scanning fails
	if err := cfg.Scan(&bConf); err != nil {
		return nil, fmt.Errorf("failed to parse bootstrap configuration: %w", err)
	}

	// Validate bootstrap configuration
	if bConf.Lynx == nil || bConf.Lynx.Application == nil {
		return nil, fmt.Errorf("invalid bootstrap configuration: missing required fields")
	}

	// Get host from configuration, fallback to system hostname if not configured
	host := bConf.Lynx.Application.Host
	if host == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to get hostname: %w", err)
		}
		host = hostname
	}

	// Initialize unified event system
	if err := events.InitWithDefaultConfig(); err != nil {
		return nil, fmt.Errorf("failed to initialize event system: %w", err)
	}

	// Start event system health check
	events.StartHealthCheck(30 * time.Second) // Check every 30 seconds

	// Create new application instance
	typedMgr := NewTypedPluginManager(plugins...)
	app := &LynxApp{
		host:               host,
		name:               bConf.Lynx.Application.Name,
		version:            bConf.Lynx.Application.Version,
		bootConfig:         &bConf,
		globalConf:         cfg,
		pluginManager:      typedMgr,
		typedPluginManager: typedMgr,
		controlPlane:       &DefaultControlPlane{},
		grpcSubs:           make(map[string]*grpc.ClientConn),
	}

	// Validate required fields
	if app.name == "" {
		return nil, fmt.Errorf("application name cannot be empty")
	}

	// Set global singleton instance (publish with lock)
	lynxMu.Lock()
	lynxApp = app
	lynxMu.Unlock()

	// Emit system start event
	app.emitSystemEvent(events.EventSystemStart, map[string]any{
		"app_name":    app.name,
		"app_version": app.version,
		"host":        app.host,
	})

	return app, nil
}

// GetPluginManager returns the plugin manager instance.
// Returns nil if the application is not initialized.
func (a *LynxApp) GetPluginManager() TypedPluginManager {
	if a == nil {
		return nil
	}
	return a.pluginManager
}

// GetTypedPluginManager returns the typed plugin manager instance.
// Returns nil if the application is not initialized.
func (a *LynxApp) GetTypedPluginManager() TypedPluginManager {
	if a == nil {
		return nil
	}
	return a.typedPluginManager
}

// GetGlobalConfig returns the global configuration instance.
// Returns nil if the application is not initialized.
func (a *LynxApp) GetGlobalConfig() config.Config {
	if a == nil {
		return nil
	}
	return a.globalConf
}

// GetTypedPlugin globally retrieves a type-safe plugin instance
func GetTypedPlugin[T plugins.Plugin](name string) (T, error) {
	var zero T
	a := Lynx()
	if a == nil {
		return zero, fmt.Errorf("lynx application not initialized")
	}

	manager := a.GetTypedPluginManager()
	if manager == nil {
		return zero, fmt.Errorf("typed plugin manager not initialized")
	}

	// Retrieve via the unified PluginManager and perform a type assertion
	return GetTypedPluginFromManager[T](manager, name)
}

// SetGlobalConfig updates the global configuration instance.
// It properly closes the existing configuration before updating.
func (a *LynxApp) SetGlobalConfig(cfg config.Config) error {
	// Check if application instance is nil
	if a == nil {
		return fmt.Errorf("application instance is nil")
	}

	// Validate the new configuration is not nil
	if cfg == nil {
		return fmt.Errorf("new configuration cannot be nil")
	}

	// Close existing configuration if present
	if a.globalConf != nil {
		if err := a.globalConf.Close(); err != nil {
			// Log the failure to close the existing configuration
			log.Errorf("Failed to close existing configuration: %v", err)
			return err
		}
	}

	// Update global configuration
	a.globalConf = cfg

	// Inject new config into the plugin manager and runtime, then broadcast config events
	if pm := a.GetPluginManager(); pm != nil {
		pm.SetConfig(cfg)
		if rt := pm.GetRuntime(); rt != nil {
			// Inject config
			rt.SetConfig(cfg)
			// Increment configuration version with overflow detection
			ver := atomic.AddUint64(&a.configVersion, 1)
			// Check for overflow (unlikely but possible in extreme scenarios)
			if ver == 0 {
				// Overflow detected - reset to 1 and log warning
				atomic.StoreUint64(&a.configVersion, 1)
				log.Warnf("Configuration version overflow detected, resetting to 1. This should not happen in normal operation.")
				ver = 1
			}
			// Broadcast: configuration is changing (using the fixed system plugin ID)
			rt.EmitPluginEvent(configEventPluginID, string(plugins.EventConfigurationChanged), map[string]any{
				"app":            a.name,
				"version":        a.version,
				"host":           a.host,
				"source":         "SetGlobalConfig",
				"config_version": ver,
			})
			// Broadcast: configuration has been applied
			rt.EmitPluginEvent(configEventPluginID, string(plugins.EventConfigurationApplied), map[string]any{
				"app":            a.name,
				"version":        a.version,
				"host":           a.host,
				"source":         "SetGlobalConfig",
				"config_version": ver,
			})
		}
	}

	return nil
}

// emitSystemEvent emits a system event to the unified event system
func (a *LynxApp) emitSystemEvent(eventType events.EventType, metadata map[string]any) {
	if a.pluginManager == nil {
		return
	}

	rt := a.pluginManager.GetRuntime()
	if rt == nil {
		return
	}

	// Create system event
	pluginEvent := plugins.PluginEvent{
		Type:      plugins.EventType(fmt.Sprintf("system.%d", eventType)),
		Priority:  plugins.PriorityHigh,
		Source:    "lynx-app",
		Category:  "system",
		PluginID:  "system",
		Status:    plugins.StatusActive,
		Timestamp: time.Now().Unix(),
		Metadata:  metadata,
	}

	// Emit through runtime
	rt.EmitEvent(pluginEvent)
}

// Close gracefully shuts down the Lynx application
func (a *LynxApp) Close() error {
	if a == nil {
		return nil
	}

	// Emit system shutdown event
	a.emitSystemEvent(events.EventSystemShutdown, map[string]any{
		"app_name":    a.name,
		"app_version": a.version,
		"host":        a.host,
		"reason":      "application_close",
	})

	// Close plugin manager
	if a.pluginManager != nil {
		a.pluginManager.UnloadPlugins()
	}

	// Close gRPC connections to prevent resource leaks
	if a.grpcSubs != nil {
		for serviceName, conn := range a.grpcSubs {
			if conn != nil {
				if err := conn.Close(); err != nil {
					log.Errorf("Failed to close gRPC connection for service %s: %v", serviceName, err)
				} else {
					log.Debugf("Successfully closed gRPC connection for service %s", serviceName)
				}
			}
		}
		a.grpcSubs = nil
	}

	// Stop health check before closing event bus
	events.StopHealthCheck()

	// Close global event bus
	if err := events.CloseGlobalEventBus(); err != nil {
		log.Errorf("Failed to close global event bus: %v", err)
	}

	// Clear global singleton instance
	lynxMu.Lock()
	lynxApp = nil
	lynxMu.Unlock()

	// Reset initialization state to allow re-initialization
	atomic.StoreInt32(&initState, initStateNotInitialized)
	initMu.Lock()
	initErr = nil
	initMu.Unlock()

	return nil
}

// Shutdown is an alias for Close for backward compatibility
func (a *LynxApp) Shutdown() error {
	return a.Close()
}

// Package lynx provides the core application framework for building microservices.
//
// This file (app.go) contains the LynxApp structure and main API entry points.
package lynx

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/events"
	"github.com/go-lynx/lynx/log"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/plugins"
	"google.golang.org/grpc"
)

var (
	// lynxApp is the singleton instance of the Lynx application
	lynxApp *LynxApp
	// RW mutex protecting reads/writes of lynxApp to avoid race conditions
	lynxMu sync.RWMutex
	// initMu protects initialization state.
	initMu sync.Mutex
	// initErr stores the last initialization error.
	initErr error
	// initCompleted indicates whether at least one initialization attempt finished.
	initCompleted bool
	// initInProgress indicates whether an initialization attempt is currently running.
	initInProgress bool
	// initDone channel signals current initialization attempt completion.
	initDone chan struct{}
)

// SetDefaultApp publishes app as the process-wide default Lynx application instance.
// It intentionally does not manage initialization lifecycle; callers should only use it
// after a fully initialized app is available.
func SetDefaultApp(app *LynxApp) {
	lynxMu.Lock()
	defer lynxMu.Unlock()
	lynxApp = app
}

// ClearDefaultApp clears the process-wide default Lynx application instance.
func ClearDefaultApp() {
	SetDefaultApp(nil)
}

// clearDefaultAppIf clears the global default only when it still points to app.
func clearDefaultAppIf(app *LynxApp) bool {
	lynxMu.Lock()
	defer lynxMu.Unlock()
	if lynxApp != app {
		return false
	}
	lynxApp = nil
	return true
}

// resetInitState resets initialization state (for testing/restart scenarios)
// Should only be called during application shutdown
func resetInitState() {
	initMu.Lock()
	defer initMu.Unlock()
	initErr = nil
	initCompleted = false
	initInProgress = false
	initDone = nil
}

// getInitTimeout returns the initialization timeout from config, default 30s.
// Can be configured via "lynx.app.init_timeout" config key.
func getInitTimeout(cfg config.Config) time.Duration {
	defaultTimeout := 30 * time.Second
	if cfg == nil {
		return defaultTimeout
	}

	var confStr string
	if err := cfg.Value("lynx.app.init_timeout").Scan(&confStr); err == nil {
		if parsed, err2 := time.ParseDuration(confStr); err2 == nil {
			// Validate timeout range: 10s to 300s
			if parsed < 10*time.Second {
				log.Warnf("init_timeout too short (%v), using minimum 10s", parsed)
				return 10 * time.Second
			}
			if parsed > 300*time.Second {
				log.Warnf("init_timeout too long (%v), using maximum 300s", parsed)
				return 300 * time.Second
			}
			return parsed
		}
	}
	return defaultTimeout
}

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

	// controlPlane manages the application's control interface (full implementation).
	// Handles dynamic configuration updates and system monitoring.
	controlPlane ControlPlane

	// Optional capability overrides for partial control plane implementations.
	// When set, these take precedence over controlPlane for the respective capability.
	// Plugins can register only the capabilities they implement via SetRateLimiter, etc.
	rateLimiter     RateLimiter
	serviceRegistry ServiceRegistry
	routeManager    RouteManager
	configManager   ConfigManager
	systemCore      SystemCore

	// controlPlaneMu protects control plane and capability fields
	controlPlaneMu sync.RWMutex

	// pluginManager handles plugin lifecycle and dependencies.
	// Provides type-safe plugin management with generic support.
	pluginManager TypedPluginManager

	// grpcSubs stores upstream gRPC connections subscribed via configuration; key is the service name
	// Protected by grpcSubsMu for concurrent access
	grpcSubsMu sync.RWMutex
	grpcSubs   map[string]*grpc.ClientConn

	// Configuration version (monotonically increasing) used for event ordering and idempotent handling
	// Uses atomic operations for thread-safe access
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

// NewStandaloneApp creates a fully initialized Lynx application instance without
// publishing it as the process-wide default singleton.
//
// This is the preferred constructor for tests, isolated runtimes, and future
// multi-instance scenarios. Call SetDefaultApp explicitly only when a global
// default instance is truly needed.
func NewStandaloneApp(cfg config.Config, plugins ...plugins.Plugin) (*LynxApp, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}
	return initializeApp(cfg, plugins...)
}

// NewApp creates or returns the process-wide default Lynx application instance.
// It preserves the historical singleton behavior for compatibility, while the
// actual instance construction is delegated to NewStandaloneApp/initializeApp.
//
// NewApp uses an explicit initialization state machine instead of sync.Once so
// failed initialization can be retried and concurrent callers can wait on the
// in-flight attempt with a bounded timeout.
func NewApp(cfg config.Config, plugins ...plugins.Plugin) (*LynxApp, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	initTimeout := getInitTimeout(cfg)

	for {
		// Fast path: already published default instance.
		if existing := Lynx(); existing != nil {
			log.Warnf("Lynx application already initialized, returning existing instance. New configuration and plugins are ignored.")
			return existing, nil
		}

		initMu.Lock()
		if initInProgress {
			doneChan := initDone
			initMu.Unlock()
			if doneChan == nil {
				return nil, fmt.Errorf("initialization in progress but completion channel is nil")
			}
			select {
			case <-doneChan:
				continue
			case <-time.After(initTimeout):
				return nil, fmt.Errorf("initialization timeout: initialization did not complete within %v", initTimeout)
			}
		}

		// If a previous attempt failed and no new initialization is in progress,
		// allow a fresh retry with the latest cfg/plugins.
		initInProgress = true
		initCompleted = false
		initErr = nil
		doneChan := make(chan struct{})
		initDone = doneChan
		initMu.Unlock()

		app, err := NewStandaloneApp(cfg, plugins...)

		initMu.Lock()
		if err != nil {
			initErr = err
		} else {
			SetDefaultApp(app)
		}
		initCompleted = true
		initInProgress = false
		close(doneChan)
		initMu.Unlock()

		if err != nil {
			return nil, fmt.Errorf("failed to initialize application: %w", err)
		}
		if app == nil {
			return nil, fmt.Errorf("application initialization resulted in nil instance")
		}
		return app, nil
	}
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
		host:          host,
		name:          bConf.Lynx.Application.Name,
		version:       bConf.Lynx.Application.Version,
		bootConfig:    &bConf,
		globalConf:    cfg,
		pluginManager: typedMgr,
		controlPlane:  &DefaultControlPlane{},
		grpcSubs:      make(map[string]*grpc.ClientConn),
	}

	// Validate required fields
	if app.name == "" {
		return nil, fmt.Errorf("application name cannot be empty")
	}

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
// This is an alias for GetPluginManager() for backward compatibility.
func (a *LynxApp) GetTypedPluginManager() TypedPluginManager {
	return a.GetPluginManager()
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

	// Retrieve via the unified TypedPluginManager and perform a type assertion
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
	// Use atomic operations to ensure configuration update is atomic
	if pm := a.GetPluginManager(); pm != nil {
		pm.SetConfig(cfg)
		if rt := pm.GetRuntime(); rt != nil {
			// Inject config atomically
			rt.SetConfig(cfg)

			// Increment configuration version with atomic overflow detection
			// Use CAS loop to ensure atomicity and prevent race conditions
			var ver uint64
			for {
				oldVer := atomic.LoadUint64(&a.configVersion)
				newVer := oldVer + 1
				// Skip 0 to avoid confusion (0 means uninitialized)
				if newVer == 0 {
					newVer = 1
				}
				// Atomically update if version hasn't changed
				if atomic.CompareAndSwapUint64(&a.configVersion, oldVer, newVer) {
					ver = newVer
					// Log warning only if we actually wrapped around
					if oldVer == ^uint64(0) {
						log.Warnf("Configuration version overflow detected, resetting to 1. This should not happen in normal operation.")
					}
					break
				}
				// Version changed, retry (with small delay to avoid busy loop)
				// Optimized: Use runtime.Gosched() instead of fixed sleep
				// This yields CPU to other goroutines more efficiently
				runtime.Gosched()
			}

			// Broadcast configuration change events atomically
			// Emit events in order: changed first, then applied
			// Use event sequence numbers to ensure ordering instead of relying on delay
			// Use best-effort error handling to prevent config update failure
			changedEvent := plugins.PluginEvent{
				Type:      plugins.EventConfigurationChanged,
				Priority:  plugins.PriorityHigh,
				Source:    "SetGlobalConfig",
				Category:  "configuration",
				PluginID:  configEventPluginID,
				Status:    plugins.StatusActive,
				Timestamp: time.Now().Unix(),
				Metadata: map[string]any{
					"app":            a.name,
					"version":        a.version,
					"host":           a.host,
					"source":         "SetGlobalConfig",
					"config_version": ver,
					"event_sequence": 1, // Sequence number to ensure ordering
				},
			}
			rt.EmitEvent(changedEvent)

			// Removed time.Sleep - rely on event sequence numbers and config_version for ordering
			// Events are processed in order by the event bus, and sequence numbers provide additional guarantee

			appliedEvent := plugins.PluginEvent{
				Type:      plugins.EventConfigurationApplied,
				Priority:  plugins.PriorityHigh,
				Source:    "SetGlobalConfig",
				Category:  "configuration",
				PluginID:  configEventPluginID,
				Status:    plugins.StatusActive,
				Timestamp: time.Now().Unix(),
				Metadata: map[string]any{
					"app":            a.name,
					"version":        a.version,
					"host":           a.host,
					"source":         "SetGlobalConfig",
					"config_version": ver,
					"event_sequence": 2, // Sequence number to ensure ordering
				},
			}
			rt.EmitEvent(appliedEvent)
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

	// Close plugin manager (unload plugins) then shutdown runtime
	if a.pluginManager != nil {
		a.pluginManager.UnloadPlugins()
		if rt := a.pluginManager.GetRuntime(); rt != nil {
			rt.Shutdown()
		}
	}

	// Close gRPC connections to prevent resource leaks
	a.grpcSubsMu.Lock()
	grpcSubsCopy := make(map[string]*grpc.ClientConn)
	for k, v := range a.grpcSubs {
		grpcSubsCopy[k] = v
	}
	a.grpcSubs = nil
	a.grpcSubsMu.Unlock()

	// Close connections outside the lock to avoid holding lock during I/O
	for serviceName, conn := range grpcSubsCopy {
		if conn != nil {
			if err := conn.Close(); err != nil {
				log.Errorf("Failed to close gRPC connection for service %s: %v", serviceName, err)
			} else {
				log.Debugf("Successfully closed gRPC connection for service %s", serviceName)
			}
		}
	}

	// Stop health check before closing event bus
	events.StopHealthCheck()

	// Close global event bus
	if err := events.CloseGlobalEventBus(); err != nil {
		log.Errorf("Failed to close global event bus: %v", err)
	}

	// Close global configuration to stop watcher goroutines and release resources.
	if a.globalConf != nil {
		if err := a.globalConf.Close(); err != nil {
			log.Errorf("Failed to close global configuration: %v", err)
		}
		a.globalConf = nil
	}

	// Clear global singleton instance if this app is the published default.
	clearDefaultAppIf(a)

	// Fix Bug 2: Cleanup memory stats cache goroutine to prevent goroutine leak
	// This ensures the background goroutine for memory stats updates is properly shut down
	cleanupMemoryStatsCache()

	// Reset initialization state
	// Note: sync.Once cannot be reset, so re-initialization after Close requires restart
	// This is acceptable as applications typically don't re-initialize after shutdown
	resetInitState()

	return nil
}

// Shutdown is an alias for Close for backward compatibility
func (a *LynxApp) Shutdown() error {
	return a.Close()
}

// GetResourceStats returns resource statistics from the plugin manager
func (a *LynxApp) GetResourceStats() map[string]any {
	if a == nil {
		return nil
	}
	if pm := a.GetPluginManager(); pm != nil {
		return pm.GetResourceStats()
	}
	return nil
}

// GetUnloadFailures returns plugin unload failures for monitoring
func (a *LynxApp) GetUnloadFailures() []UnloadFailureRecord {
	if a == nil {
		return nil
	}
	if pm := a.GetPluginManager(); pm != nil {
		return pm.GetUnloadFailures()
	}
	return nil
}

// ClearUnloadFailures clears recorded unload failures
func (a *LynxApp) ClearUnloadFailures() {
	if a == nil {
		return
	}
	if pm := a.GetPluginManager(); pm != nil {
		pm.ClearUnloadFailures()
	}
}

// Package app provides core application functionality for the Lynx framework
package app

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/go-lynx/lynx/app/conf"
	"github.com/go-lynx/lynx/app/log"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/plugins"
	"google.golang.org/grpc"
)

var (
	// lynxApp is the singleton instance of the Lynx application
	lynxApp *LynxApp
	// initOnce ensures the Lynx application is initialized only once.
	// Uses sync.Once to guarantee atomic initialization in concurrent scenarios.
	initOnce sync.Once
	// RW mutex protecting reads/writes of lynxApp to avoid race conditions
	lynxMu sync.RWMutex
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

	// If already initialized, return the singleton to avoid returning (nil, nil)
	if existing := Lynx(); existing != nil {
		return existing, nil
	}

	var app *LynxApp
	var err error

	// Use sync.Once to ensure the application is initialized only once
	initOnce.Do(func() {
		app, err = initializeApp(cfg, plugins...)
	})

	// Return error if initialization failed
	if err != nil {
		return nil, fmt.Errorf("failed to initialize application: %w", err)
	}

	// In concurrent scenarios: if app is nil but the singleton was initialized by another goroutine, return the singleton
	if app == nil {
		if existing := Lynx(); existing != nil {
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
	// Get system hostname
	hostname, err := os.Hostname()
	// Return error if hostname retrieval fails
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

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

	// Create new application instance
	typedMgr := NewTypedPluginManager(plugins...)
	app := &LynxApp{
		host:               hostname,
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
			// Increment configuration version
			ver := atomic.AddUint64(&a.configVersion, 1)
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

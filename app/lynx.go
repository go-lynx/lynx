// Package app provides core application functionality for the Lynx framework
package app

import (
	"fmt"
	"os"
	"sync"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/conf"
	"github.com/go-lynx/lynx/plugins"
)

var (
	// lynxApp is the singleton instance of the Lynx application
	lynxApp  *LynxApp
	initOnce sync.Once
)

// LynxApp represents the main application instance
type LynxApp struct {
	host          string
	name          string
	version       string
	cert          Cert
	logger        log.Logger
	logHelper     log.Helper
	globalConf    config.Config
	controlPlane  ControlPlane
	pluginManager LynxPluginManager
}

// Lynx returns the global LynxApp instance.
// It ensures thread-safe access to the singleton instance.
func Lynx() *LynxApp {
	return lynxApp
}

// GetHost retrieves the hostname of the current application instance.
// Returns an empty string if the application is not initialized.
func GetHost() string {
	if lynxApp == nil {
		return ""
	}
	return lynxApp.host
}

// GetName retrieves the application name.
// Returns an empty string if the application is not initialized.
func GetName() string {
	if lynxApp == nil {
		return ""
	}
	return lynxApp.name
}

// GetVersion retrieves the application version.
// Returns an empty string if the application is not initialized.
func GetVersion() string {
	if lynxApp == nil {
		return ""
	}
	return lynxApp.version
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
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	var app *LynxApp
	var err error

	initOnce.Do(func() {
		app, err = initializeApp(cfg, plugins...)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to initialize application: %w", err)
	}

	return app, nil
}

// initializeApp handles the actual initialization of the LynxApp instance.
func initializeApp(cfg config.Config, plugins ...plugins.Plugin) (*LynxApp, error) {
	// Get system hostname
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	// Parse bootstrap configuration
	var bootConfig conf.Bootstrap
	if err := cfg.Scan(&bootConfig); err != nil {
		return nil, fmt.Errorf("failed to parse bootstrap configuration: %w", err)
	}

	// Validate bootstrap configuration
	if bootConfig.Lynx == nil || bootConfig.Lynx.Application == nil {
		return nil, fmt.Errorf("invalid bootstrap configuration: missing required fields")
	}

	// Create new application instance
	app := &LynxApp{
		host:          hostname,
		name:          bootConfig.Lynx.Application.Name,
		version:       bootConfig.Lynx.Application.Version,
		globalConf:    cfg,
		pluginManager: NewLynxPluginManager(plugins...),
		controlPlane:  &LocalControlPlane{},
	}

	// Validate required fields
	if app.name == "" {
		return nil, fmt.Errorf("application name cannot be empty")
	}

	// Set global singleton instance
	lynxApp = app

	return app, nil
}

// GetPluginManager returns the plugin manager instance.
// Returns nil if the application is not initialized.
func (a *LynxApp) GetPluginManager() LynxPluginManager {
	if a == nil {
		return nil
	}
	return a.pluginManager
}

// GetGlobalConfig returns the global configuration instance.
// Returns nil if the application is not initialized.
func (a *LynxApp) GetGlobalConfig() config.Config {
	if a == nil {
		return nil
	}
	return a.globalConf
}

// SetGlobalConfig updates the global configuration instance.
// It properly closes the existing configuration before updating.
func (a *LynxApp) SetGlobalConfig(cfg config.Config) error {
	if a == nil {
		return fmt.Errorf("application instance is nil")
	}

	if cfg == nil {
		return fmt.Errorf("new configuration cannot be nil")
	}

	// Close existing configuration if present
	if a.globalConf != nil {
		if err := a.globalConf.Close(); err != nil {
			a.logHelper.Errorf("Failed to close existing configuration: %v", err)
			return err
		}
	}

	a.globalConf = cfg
	return nil
}

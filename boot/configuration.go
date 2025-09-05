package boot

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
)

// LoadBootstrapConfig loads bootstrap configuration from local files or directories.
// It reads configuration from the path specified by the configuration manager and initializes the application's configuration state.
//
// Returns:
//   - error: Any error that occurs during configuration loading
func (app *Application) LoadBootstrapConfig() error {
	// Check if Application instance is nil
	if app == nil {
		return fmt.Errorf("application instance is nil: cannot load bootstrap configuration")
	}

	// Get configuration path
	configMgr := GetConfigManager()
	configPath := configMgr.GetConfigPath()

	// Check if configuration path is empty
	if configPath == "" {
		return fmt.Errorf("configuration path is empty: please specify config path via -conf flag or LYNX_CONFIG_PATH environment variable")
	}

	// Log attempt to load configuration
	log.Infof("loading local bootstrap configuration from: %s", configPath)

	// Create configuration source from local file
	source := file.NewSource(configPath)
	// Check if configuration source was created successfully
	if source == nil {
		return fmt.Errorf("failed to create configuration source from: %s", configPath)
	}

	// Create new configuration instance
	cfg := config.New(
		// Specify configuration source
		config.WithSource(source),
	)
	// Check if configuration instance was created successfully
	if cfg == nil {
		return fmt.Errorf("failed to create configuration instance")
	}

	// Load configuration
	if err := cfg.Load(); err != nil {
		return fmt.Errorf("failed to load configuration from %s: %w", configPath, err)
	}

	// Validate configuration
	if err := app.validateConfig(cfg); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Store configuration before setting up cleanup operations
	// This ensures we don't lose the configuration reference if cleanup setup fails
	app.conf = cfg

	// Set up configuration cleanup operations - return error if failed
	if err := app.setupConfigCleanup(cfg); err != nil {
		return fmt.Errorf("failed to setup configuration cleanup: %w", err)
	}

	return nil
}

// validateConfig validates the integrity and correctness of configuration
func (app *Application) validateConfig(cfg config.Config) error {
	// Check if configuration instance is nil
	if cfg == nil {
		return fmt.Errorf("configuration instance is nil")
	}

	// Check if required configuration items exist
	// Additional validation logic can be added here based on actual requirements
	requiredKeys := []string{
		"lynx.application.name",
		"lynx.application.version",
	}

	for _, key := range requiredKeys {
		if _, err := cfg.Value(key).String(); err != nil {
			return fmt.Errorf("required configuration key '%s' is missing or invalid: %w", key, err)
		}
	}

	return nil
}

// setupConfigCleanup ensures proper cleanup when configuration resources are no longer needed.
func (app *Application) setupConfigCleanup(cfg config.Config) error {
	// Check if configuration instance is nil
	if cfg == nil {
		return fmt.Errorf("configuration instance is nil: cannot setup cleanup for nil config")
	}

	// Set up deferred cleanup function
	app.cleanup = func() {
		// Close configuration resources
		if err := cfg.Close(); err != nil {
			// Log configuration close failure
			log.Errorf("failed to close configuration: %v", err)
		}
	}

	return nil
}

// GetName gets application name
func (app *Application) GetName() string {
	if app.conf != nil {
		if name, err := app.conf.Value("lynx.application.name").String(); err == nil {
			return name
		}
	}
	return "lynx"
}

// GetHost gets application host
func (app *Application) GetHost() string {
	if app.conf != nil {
		if host, err := app.conf.Value("lynx.application.host").String(); err == nil {
			return host
		}
	}
	return "localhost"
}

// GetVersion gets application version
func (app *Application) GetVersion() string {
	if app.conf != nil {
		if version, err := app.conf.Value("lynx.application.version").String(); err == nil {
			return version
		}
	}
	return "unknown"
}

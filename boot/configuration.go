package boot

import (
	"flag"
	"fmt"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
)

func (app *Application) resolveBootstrapConfigPath() string {
	if app == nil {
		return ""
	}

	configMgr := GetConfigManager()
	if app.configPath != "" {
		return app.configPath
	}
	if configPath := configMgr.GetConfigPath(); configPath != "" {
		return configPath
	}
	if configPath := currentBootstrapFlagValue(); configPath != "" {
		return configPath
	}
	return configMgr.GetDefaultConfigPath()
}

func currentBootstrapFlagValue() string {
	if existing := flag.CommandLine.Lookup("conf"); existing != nil && existing.Value != nil {
		if value := existing.Value.String(); value != "" {
			return value
		}
	}
	return flagConf
}

// LoadBootstrapConfig loads, validates, and stores the local bootstrap config.
// The path is resolved (in order) from the instance path, the config manager,
// the -conf flag, then the manager's default. It also registers the cleanup
// that closes the config on shutdown.
func (app *Application) LoadBootstrapConfig() error {
	if app == nil {
		return fmt.Errorf("application instance is nil: cannot load bootstrap configuration")
	}

	ensureBootstrapFlagsRegistered()

	configPath := app.resolveBootstrapConfigPath()
	if configPath != "" && app.configPath == "" {
		GetConfigManager().SetConfigPath(configPath)
	}

	if configPath == "" {
		return fmt.Errorf("configuration path is empty: please specify config path via -conf flag or LYNX_CONFIG_PATH environment variable")
	}

	log.Infof("loading local bootstrap configuration from: %s", configPath)

	source := file.NewSource(configPath)
	if source == nil {
		return fmt.Errorf("failed to create configuration source from: %s", configPath)
	}

	cfg := config.New(
		config.WithSource(source),
	)
	if cfg == nil {
		return fmt.Errorf("failed to create configuration instance")
	}

	if err := cfg.Load(); err != nil {
		return fmt.Errorf("failed to load configuration from %s: %w", configPath, err)
	}

	if err := app.validateConfig(cfg); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Store the config before wiring cleanup so the reference isn't lost if
	// cleanup setup fails.
	app.conf = cfg

	if err := app.setupConfigCleanup(cfg); err != nil {
		return fmt.Errorf("failed to setup configuration cleanup: %w", err)
	}

	return nil
}

// validateConfig fails fast if the config is nil or missing keys the framework
// requires to identify the application.
func (app *Application) validateConfig(cfg config.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration instance is nil")
	}

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
	if cfg == nil {
		return fmt.Errorf("configuration instance is nil: cannot setup cleanup for nil config")
	}

	app.cleanup = func() {
		if err := cfg.Close(); err != nil {
			log.Errorf("failed to close configuration: %v", err)
		}
	}

	return nil
}

// GetName returns lynx.application.name, or "lynx" if unset.
func (app *Application) GetName() string {
	if app.conf != nil {
		if name, err := app.conf.Value("lynx.application.name").String(); err == nil {
			return name
		}
	}
	return "lynx"
}

// GetHost returns lynx.application.host, or "localhost" if unset.
func (app *Application) GetHost() string {
	if app.conf != nil {
		if host, err := app.conf.Value("lynx.application.host").String(); err == nil {
			return host
		}
	}
	return "localhost"
}

// GetVersion returns lynx.application.version, or "unknown" if unset.
func (app *Application) GetVersion() string {
	if app.conf != nil {
		if version, err := app.conf.Value("lynx.application.version").String(); err == nil {
			return version
		}
	}
	return "unknown"
}

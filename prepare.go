// Package lynx provides the core application framework for building microservices.
//
// This file (prepare.go) contains plugin preparation and bootstrapping:
//   - PreparePlug: Bootstrap plugin loading from configuration
//   - Plugin factory integration for automatic plugin creation
//   - Configuration-driven plugin registration
package lynx

import (
	"fmt"

	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/plugins"

	"github.com/go-kratos/kratos/v2/config"
)

// PreparePlug bootstraps plugin loading via remote or local configuration files.
// It initializes and registers plugins based on configuration. If a single plugin fails,
// it logs the error and skips that plugin, attempting to return other successful items.
// Returns a list of successfully prepared plugins (possibly empty) and an error only for global failures.
func (m *DefaultPluginManager[T]) PreparePlug(config config.Config) ([]plugins.Plugin, error) {
	// Validate configuration is not nil; log and return error if nil
	if config == nil {
		log.Error("Configuration is nil")
		return nil, fmt.Errorf("configuration is nil")
	}

	// Retrieve registry containing all registered plugin config prefixes
	table := m.factory.GetPluginRegistry()
	// If registry is empty, log a warning and return error
	if len(table) == 0 {
		log.Warn("No plugins registered in factory")
		return nil, fmt.Errorf("no plugins registered in factory")
	}

	// Initialize slice to store plugins to be loaded with preallocated capacity
	prepared := make([]plugins.Plugin, 0, len(table))

	// Iterate over configuration prefixes
	for confPrefix, names := range table {
		// Validate prefix not empty; warn and skip if empty
		if confPrefix == "" {
			log.Warnf("Empty configuration prefix found, skipping")
			continue
		}

		// Try to get configuration value for current prefix
		cfg := config.Value(confPrefix)
		// If value is nil, log debug and continue
		if cfg == nil {
			log.Debugf("No configuration found for prefix: %s", confPrefix)
			continue
		}

		// Load configuration; if result is nil, log debug and continue
		if loaded := cfg.Load(); loaded == nil {
			log.Debugf("Configuration cfg is nil for prefix: %s", confPrefix)
			continue
		}

		// Ensure there are plugin names associated with the prefix; otherwise skip
		if len(names) == 0 {
			log.Debugf("No plugins associated with prefix: %s", confPrefix)
			continue
		}

		// Process each plugin name and collect success/failure counts (observability)
		var successCount, failCount int
		for _, name := range names {
			// Validate plugin name not empty
			if name == "" {
				log.Warn("Empty plugin name found, skipping")
				continue
			}

			// Prepare plugin if creatable
			if err := m.preparePlugin(name); err != nil {
				// Log specific preparation failure reason
				log.Warnf("prepare plugin %s failed: %v", name, err)
				failCount++
				continue
			}
			successCount++

			// Retrieve the plugin instance and append to the result slice
			if value, ok := m.pluginInstances.Load(name); ok {
				if plugin, ok := value.(plugins.Plugin); ok {
					prepared = append(prepared, plugin)
				}
			}
		}

		// Prefix-level summary logging to help diagnose configuration issues
		if successCount > 0 || failCount > 0 {
			if failCount > 0 {
				log.Warnf("confPrefix %s prepared summary: success=%d, failed=%d, total=%d", confPrefix, successCount, failCount, len(names))
			} else {
				log.Infof("confPrefix %s prepared summary: success=%d, failed=%d, total=%d", confPrefix, successCount, failCount, len(names))
			}
		} else {
			log.Debugf("confPrefix %s has no matched plugin names in registry or no valid config", confPrefix)
		}
	}

	// Log overall preparation result
	if len(prepared) != 0 {
		log.Infof("successfully prepared %d plugins", len(prepared))
	} else {
		log.Warn("no plugins prepared from config and registry")
	}

	return prepared, nil
}

// preparePlugin performs preparation for a single plugin.
// It checks for existing instances, creates the plugin, and adds it to the manager.
// Returns an error if any step fails.
func (m *DefaultPluginManager[T]) preparePlugin(name string) error {
	// Return error if plugin is already loaded
	if _, exists := m.pluginInstances.Load(name); exists {
		return fmt.Errorf("plugin %s is already loaded", name)
	}

	// Validate the plugin exists in the factory
	if !m.factory.HasPlugin(name) {
		return fmt.Errorf("plugin %s does not exist in factory", name)
	}

	// Create the plugin instance
	p, err := m.factory.CreatePlugin(name)
	if err != nil {
		return fmt.Errorf("failed to create plugin %s: %v", name, err)
	}

	// Validate the created plugin instance is not nil
	if p == nil {
		return fmt.Errorf("created plugin %s is nil", name)
	}

	// Track the plugin within the manager
	m.mu.Lock()
	m.pluginList = append(m.pluginList, p)
	m.mu.Unlock()
	m.pluginInstances.Store(p.Name(), p)

	return nil
}

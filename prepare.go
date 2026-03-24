// Package lynx provides the core application framework for building microservices.
//
// This file (prepare.go) contains plugin preparation and bootstrapping:
//   - PreparePlug: Bootstrap plugin loading from configuration
//   - Plugin factory integration for automatic plugin creation
//   - Configuration-driven plugin registration
package lynx

import (
	"fmt"
	"strings"

	"github.com/go-lynx/lynx/log"
	"github.com/go-lynx/lynx/plugins"

	"github.com/go-kratos/kratos/v2/config"
)

// PreparePlug bootstraps plugin staging via remote or local configuration files.
// It prepares plugin instances based on configuration and returns only plugins
// that are not yet managed by the lifecycle manager.
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
	allowPartialFailure := allowPartialPrepareFailure(config)
	report := PrepareReport{PartialAllowed: allowPartialFailure}
	var prepareFailures []string

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
			report.Skipped = append(report.Skipped, confPrefix)
			continue
		}

		// Load configuration; if result is nil, log debug and continue
		if loaded := cfg.Load(); loaded == nil {
			log.Debugf("Configuration cfg is nil for prefix: %s", confPrefix)
			report.Skipped = append(report.Skipped, confPrefix)
			continue
		}

		// Ensure there are plugin names associated with the prefix; otherwise skip
		if len(names) == 0 {
			log.Debugf("No plugins associated with prefix: %s", confPrefix)
			report.Skipped = append(report.Skipped, confPrefix)
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
			plugin, err := m.preparePlugin(name)
			if err != nil {
				// Log specific preparation failure reason
				log.Warnf("prepare plugin %s failed: %v", name, err)
				failCount++
				report.Failures = append(report.Failures, PrepareFailure{PluginName: name, Reason: err.Error()})
				prepareFailures = append(prepareFailures, fmt.Sprintf("%s: %v", name, err))
				continue
			}
			if plugin == nil {
				report.Skipped = append(report.Skipped, name)
				continue
			}
			successCount++
			report.Prepared = append(report.Prepared, name)
			prepared = append(prepared, plugin)
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
	m.setLastPrepareReport(report)

	if len(prepareFailures) > 0 && !allowPartialFailure {
		return prepared, fmt.Errorf("plugin preparation failed for %d plugin(s): %s", len(prepareFailures), strings.Join(prepareFailures, "; "))
	}

	return prepared, nil
}

func allowPartialPrepareFailure(config config.Config) bool {
	if config == nil {
		return false
	}
	var allow bool
	if err := config.Value("lynx.plugins.allow_partial_prepare_failure").Scan(&allow); err == nil {
		return allow
	}
	return false
}

// preparePlugin performs preparation for a single plugin.
// It returns nil,nil when the plugin is already lifecycle-managed and therefore
// should not be prepared again for the current load attempt.
func (m *DefaultPluginManager[T]) preparePlugin(name string) (plugins.Plugin, error) {
	if value, exists := m.managedInstances.Load(name); exists {
		if _, ok := value.(plugins.Plugin); ok {
			return nil, nil
		}
	}

	// Reuse an already staged prepared plugin when available.
	if value, exists := m.pluginInstances.Load(name); exists {
		if plugin, ok := value.(plugins.Plugin); ok && plugin != nil {
			return plugin, nil
		}
		return nil, fmt.Errorf("plugin %s has invalid prepared instance", name)
	}

	// Validate the plugin exists in the factory
	if !m.factory.HasPlugin(name) {
		return nil, fmt.Errorf("plugin %s does not exist in factory", name)
	}

	// Create the plugin instance
	p, err := m.factory.CreatePlugin(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin %s: %v", name, err)
	}

	// Validate the created plugin instance is not nil
	if p == nil {
		return nil, fmt.Errorf("created plugin %s is nil", name)
	}

	if err := m.registerPreparedPlugin(p); err != nil {
		return nil, err
	}
	return p, nil
}

package app

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/config"
)

// PreparePlug bootstraps plugin loading through remote or local configuration files.
// It handles the initialization and registration of plugins based on configuration.
// Returns a list of successfully prepared plugin names.
func (m *DefaultLynxPluginManager) PreparePlug(config config.Config) []string {
	if config == nil {
		Lynx().logHelper.Error("Configuration is nil")
		return nil
	}

	// Get the registration table containing all registered plugin configuration prefixes
	table := m.factory.GetPluginRegistry()
	if len(table) == 0 {
		Lynx().logHelper.Warn("No plugins registered in factory")
		return nil
	}

	// Initialize slice to store names of plugins to be loaded
	plugNames := make([]string, 0, len(table))

	// Iterate through configuration prefixes
	for confPrefix, names := range table {
		if confPrefix == "" {
			Lynx().logHelper.Warnf("Empty configuration prefix found, skipping")
			continue
		}

		// Attempt to get configuration value for current prefix
		cfg := config.Value(confPrefix)
		if cfg == nil {
			Lynx().logHelper.Debugf("No configuration found for prefix: %s", confPrefix)
			continue
		}

		if loaded := cfg.Load(); loaded == nil {
			Lynx().logHelper.Debugf("Configuration cfg is nil for prefix: %s", confPrefix)
			continue
		}

		// Skip if no plugin names associated with prefix
		if len(names) == 0 {
			Lynx().logHelper.Debugf("No plugins associated with prefix: %s", confPrefix)
			continue
		}

		// Process each plugin name
		for _, name := range names {
			if name == "" {
				Lynx().logHelper.Warn("Empty plugin name found, skipping")
				continue
			}

			// Check if plugin already exists and can be created
			if err := m.preparePlugin(name); err != nil {
				Lynx().logHelper.Errorf("Failed to prepare plugin %s: %v", name, err)
				continue
			}

			plugNames = append(plugNames, name)
		}
	}

	if len(plugNames) == 0 {
		Lynx().logHelper.Warn("No plugins were prepared")
	} else {
		Lynx().logHelper.Infof("Successfully prepared %d plugins", len(plugNames))
	}

	return plugNames
}

// preparePlugin handles the preparation of a single plugin.
// It checks existence, creates the plugin, and adds it to the manager.
// Returns error if any step fails.
func (m *DefaultLynxPluginManager) preparePlugin(name string) error {
	// Check if plugin is already loaded
	if _, exists := m.pluginMap[name]; exists {
		return fmt.Errorf("plugin %s is already loaded", name)
	}

	// Verify plugin exists in factory
	if !m.factory.HasPlugin(name) {
		return fmt.Errorf("plugin %s does not exist in factory", name)
	}

	// Create plugin instance
	p, err := m.factory.CreatePlugin(name)
	if err != nil {
		return fmt.Errorf("failed to create plugin %s: %v", name, err)
	}

	if p == nil {
		return fmt.Errorf("created plugin %s is nil", name)
	}

	// Add plugin to manager's tracking structures
	m.pluginList = append(m.pluginList, p)
	m.pluginMap[p.Name()] = p

	return nil
}

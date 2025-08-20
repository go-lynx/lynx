package factory

import (
	"fmt"
	"sync"
	"github.com/go-lynx/lynx/plugins"
)

// Global registry instance
var (
	globalPluginRegistry *PluginRegistry
	once                 sync.Once
)

// GlobalPluginRegistry returns the singleton instance of the plugin registry.
func GlobalPluginRegistry() Registry {
	once.Do(func() {
		globalPluginRegistry = newDefaultPluginRegistry()
	})
	return globalPluginRegistry
}

// PluginRegistry implements the Registry interface.
type PluginRegistry struct {
	// configToPlugins maps configuration prefixes to their associated plugin names.
	// Example: "http" -> ["http_server", "http_client"]
	configToPlugins map[string][]string

	// pluginCreators stores the creation functions for each plugin.
	// Maps plugin names to their respective creation functions.
	pluginCreators map[string]func() plugins.Plugin
}

// newDefaultPluginRegistry initializes a new instance of PluginRegistry.
func newDefaultPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		configToPlugins: make(map[string][]string),
		pluginCreators:  make(map[string]func() plugins.Plugin),
	}
}

// RegisterPlugin registers a new plugin with its configuration prefix and creation function.
// Panics if a plugin with the same name is already registered.
func (r *PluginRegistry) RegisterPlugin(name string, configPrefix string, creator func() plugins.Plugin) {
	if _, exists := r.pluginCreators[name]; exists {
		panic(fmt.Errorf("plugin already registered: %s", name))
	}

	r.pluginCreators[name] = creator

	pluginList := r.configToPlugins[configPrefix]
	if pluginList == nil {
		r.configToPlugins[configPrefix] = []string{name}
	} else {
		r.configToPlugins[configPrefix] = append(pluginList, name)
	}
}

// UnregisterPlugin removes a plugin from both the creator map and configuration mapping.
func (r *PluginRegistry) UnregisterPlugin(name string) {
	// Remove from creator map
	delete(r.pluginCreators, name)

	// Remove from configuration mapping
	for prefix, pluginList := range r.configToPlugins {
		for i, plugin := range pluginList {
			if plugin == name {
				// Remove the plugin from the slice
				r.configToPlugins[prefix] = append(pluginList[:i], pluginList[i+1:]...)

				// If no pluginList left for this prefix, remove the prefix entry
				if len(r.configToPlugins[prefix]) == 0 {
					delete(r.configToPlugins, prefix)
				}
				break
			}
		}
	}
}

// GetPluginRegistry returns the current mapping of configuration prefixes to plugin names.
func (r *PluginRegistry) GetPluginRegistry() map[string][]string {
	return r.configToPlugins
}

// CreatePlugin creates a new instance of a plugin by its name.
// Returns an error if the plugin is not registered.
func (r *PluginRegistry) CreatePlugin(name string) (plugins.Plugin, error) {
	creator, exists := r.pluginCreators[name]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}
	return creator(), nil
}

// HasPlugin checks if a plugin is registered in the registry.
func (r *PluginRegistry) HasPlugin(name string) bool {
	_, exists := r.pluginCreators[name]
	return exists
}

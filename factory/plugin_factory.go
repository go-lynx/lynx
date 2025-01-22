// Package factory provides functionality for creating and managing plugins in the Lynx framework.
package factory

import (
	"fmt"

	"github.com/go-lynx/lynx/plugins"
)

// Global factory instance
var (
	globalFactory *LynxPluginFactory
)

// PluginFactory defines the complete interface for plugin management,
// combining both creation and registry capabilities.
type PluginFactory interface {
	PluginCreator
	PluginRegistry
}

// PluginCreator defines the interface for creating plugin instances.
type PluginCreator interface {
	// CreatePlugin instantiates a new plugin instance by its name.
	// Returns an error if the plugin cannot be created.
	CreatePlugin(name string) (plugins.Plugin, error)
}

// PluginRegistry defines the interface for managing plugin registrations.
type PluginRegistry interface {
	// RegisterPlugin adds a new plugin to the registry with its configuration prefix
	// and creation function.
	RegisterPlugin(name string, configPrefix string, creator func() plugins.Plugin)

	// UnregisterPlugin removes a plugin from the registry.
	UnregisterPlugin(name string)

	// GetPluginRegistry returns the mapping of configuration prefixes to plugin names.
	GetPluginRegistry() map[string][]string

	// HasPlugin checks if a plugin is registered with the given name.
	HasPlugin(name string) bool
}

// GlobalPluginFactory returns the singleton instance of the plugin factory.
func GlobalPluginFactory() PluginFactory {
	if globalFactory == nil {
		globalFactory = newDefaultPluginFactory()
	}
	return globalFactory
}

// LynxPluginFactory implements the PluginFactory interface.
type LynxPluginFactory struct {
	// configToPlugins maps configuration prefixes to their associated plugin names.
	// Example: "http" -> ["http_server", "http_client"]
	configToPlugins map[string][]string

	// pluginCreators stores the creation functions for each plugin.
	// Maps plugin names to their respective creation functions.
	pluginCreators map[string]func() plugins.Plugin
}

// newDefaultPluginFactory initializes a new instance of LynxPluginFactory.
func newDefaultPluginFactory() *LynxPluginFactory {
	return &LynxPluginFactory{
		configToPlugins: make(map[string][]string),
		pluginCreators:  make(map[string]func() plugins.Plugin),
	}
}

// RegisterPlugin registers a new plugin with its configuration prefix and creation function.
// Panics if a plugin with the same name is already registered.
func (f *LynxPluginFactory) RegisterPlugin(name string, configPrefix string, creator func() plugins.Plugin) {
	if _, exists := f.pluginCreators[name]; exists {
		panic(fmt.Errorf("plugin already registered: %s", name))
	}

	f.pluginCreators[name] = creator

	pluginList := f.configToPlugins[configPrefix]
	if pluginList == nil {
		f.configToPlugins[configPrefix] = []string{name}
	} else {
		f.configToPlugins[configPrefix] = append(pluginList, name)
	}
}

// UnregisterPlugin removes a plugin from both the creator map and configuration mapping.
func (f *LynxPluginFactory) UnregisterPlugin(name string) {
	// Remove from creator map
	delete(f.pluginCreators, name)

	// Remove from configuration mapping
	for prefix, pluginList := range f.configToPlugins {
		for i, plugin := range pluginList {
			if plugin == name {
				// Remove the plugin from the slice
				f.configToPlugins[prefix] = append(pluginList[:i], pluginList[i+1:]...)

				// If no pluginList left for this prefix, remove the prefix entry
				if len(f.configToPlugins[prefix]) == 0 {
					delete(f.configToPlugins, prefix)
				}
				break
			}
		}
	}
}

// GetPluginRegistry returns the current mapping of configuration prefixes to plugin names.
func (f *LynxPluginFactory) GetPluginRegistry() map[string][]string {
	return f.configToPlugins
}

// CreatePlugin creates a new instance of a plugin by its name.
// Returns an error if the plugin is not registered.
func (f *LynxPluginFactory) CreatePlugin(name string) (plugins.Plugin, error) {
	creator, exists := f.pluginCreators[name]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}
	return creator(), nil
}

// HasPlugin checks if a plugin is registered in the factory.
func (f *LynxPluginFactory) HasPlugin(name string) bool {
	_, exists := f.pluginCreators[name]
	return exists
}

// Package factory provides functionality for creating and managing plugins in the Lynx framework.
package factory

import (
	"github.com/go-lynx/lynx/plugins"
)

// Registry defines the interface for managing plugin registrations.
type Registry interface {
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

// Creator defines the interface for creating plugin instances.
type Creator interface {
	// CreatePlugin instantiates a new plugin instance by its name.
	// Returns an error if the plugin cannot be created.
	CreatePlugin(name string) (plugins.Plugin, error)
}

// Factory defines the complete interface for plugin management,
// combining both creation and registry capabilities.
type Factory interface {
	Creator
	Registry
}

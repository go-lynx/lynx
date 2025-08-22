package pulsar

import (
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init function registers the Apache Pulsar plugin to the global plugin factory.
// This function is automatically called when the package is imported.
// It creates a new PulsarClient instance and registers it to the plugin factory with the configured plugin name and configuration prefix.
func init() {
	// Call the RegisterPlugin method of the global plugin factory for plugin registration
	// Pass in the plugin name, configuration prefix, and a function that returns a plugins.Plugin interface instance
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		// Create and return a new PulsarClient instance
		return NewPulsarClient()
	})
}

// GetPulsarClient gets the Apache Pulsar client instance from the plugin manager.
// This function provides access to the underlying Pulsar client for other parts of the application
// that may need to use message queue functionality.
//
// Returns:
//   - *PulsarClient: Configured Apache Pulsar client instance
//
// Note: This function will panic if the plugin is not properly initialized or if the plugin manager cannot find the Pulsar plugin.
func GetPulsarClient() *PulsarClient {
	// Get the plugin with the specified name from the application's plugin manager,
	// convert it to *PulsarClient type, and return it
	return app.Lynx().GetPluginManager().GetPlugin(pluginName).(*PulsarClient)
}

// GetPulsarClientByName gets a specific Pulsar client by name if multiple instances are configured.
// This function provides access to specific client instances when multiple Pulsar configurations are used.
//
// Parameters:
//   - name: The name of the Pulsar client instance to retrieve
//
// Returns:
//   - *PulsarClient: The specified Pulsar client instance, or nil if not found
func GetPulsarClientByName(name string) *PulsarClient {
	client := GetPulsarClient()
	if client != nil && client.GetPulsarConfig() != nil {
		// For now, return the main client since we support single instance
		// In the future, this could be extended to support multiple named instances
		return client
	}
	return nil
}

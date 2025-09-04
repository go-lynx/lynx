package seata

import (
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init is Go's initialization function that is automatically executed when the package is loaded.
// This function's purpose is to register the Seata client plugin to the global plugin factory.
func init() {
	// Call the RegisterPlugin method of the global plugin factory instance for plugin registration.
	// The first parameter pluginName is the unique name of the plugin, used to identify the plugin in the system.
	// The second parameter confPrefix is the configuration prefix, used to read the plugin's related configuration from the configuration file.
	// The third parameter is an anonymous function that returns an instance implementing the plugins.Plugin interface.
	// Create a new Seata client plugin instance by calling the NewSeataClient function and return it.
	factory.GlobalTypedFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		return NewTxSeataClient()
	})
}

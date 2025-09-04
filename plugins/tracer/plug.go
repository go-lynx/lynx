package tracer

import (
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init is a special function in Go that is automatically executed when the package is initialized, and only once.
// This function's purpose is to register the tracing plugin to the global plugin factory.
func init() {
	// Register the tracing plugin to the global plugin factory.
	// The first parameter pluginName is the name of the plugin, used to uniquely identify the plugin.
	// The second parameter confPrefix is the configuration prefix, used to locate the plugin's configuration items in the configuration file.
	// The third parameter is an anonymous function that returns an instance implementing the plugins.Plugin interface.
	factory.GlobalTypedFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		// Call the NewPlugTracer function to create a new tracing plugin instance and return it.
		return NewPlugTracer()
	})
}

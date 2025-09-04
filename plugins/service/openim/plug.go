package openim

import (
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

const (
	pluginName = "openim"
	confPrefix = "openim"
)

// init function registers the OpenIM service plugin to the global plugin factory.
// This function is automatically called when the package is imported.
// It creates a new ServiceOpenIM instance and registers it to the package openim

func init() {
    // Call the RegisterPlugin method of the global plugin factory for plugin registration
    // Pass in the plugin name, configuration prefix, and a function that returns a plugins.Plugin interface instance
    factory.GlobalTypedFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
        // Create and return a new ServiceOpenIM instance
        return NewServiceOpenIM()
    })
}

// GetOpenIMService gets the OpenIM service instance from the plugin manager.
// This function provides access to the underlying OpenIM service for other parts of the application
// that may need to use IM functionality.
//
// Returns:
//   - *ServiceOpenIM: Configured OpenIM service instance
//
// Note: This function will panic if the plugin is not properly initialized or if the plugin manager cannot find the OpenIM plugin.
func GetOpenIMService() *ServiceOpenIM {
	// Get the plugin with the specified name from the application's plugin manager,
	// convert it to *ServiceOpenIM type, and return it
	return app.Lynx().GetPluginManager().GetPlugin(pluginName).(*ServiceOpenIM)
}

package polaris

import (
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init is executed automatically when the package is imported.
// It registers the Polaris plugin into the global plugin factory so the plugin
// manager can discover and load it.
func init() {
	// Register the Polaris control-plane plugin to the global plugin factory.
	// The first parameter is the plugin name; the second parameter confPrefix is used to read
	// plugin-related settings from configuration files.
	// The last parameter is a constructor function returning an instance that
	// implements plugins.Plugin.
	factory.GlobalTypedFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		// Create and return a new Polaris control-plane plugin instance
		return NewPolarisControlPlane()
	})
}

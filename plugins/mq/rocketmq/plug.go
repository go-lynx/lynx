package rocketmq

import (
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init is executed automatically when the package is imported.
// It registers the RocketMQ plugin into the global plugin factory so the plugin
// manager can discover and load it.
func init() {
	// Obtain the global plugin factory and register the plugin by calling
	// RegisterPlugin. The parameter `pluginName` is the unique plugin identifier.
	// The parameter `confPrefix` is the configuration prefix used to load
	// plugin-related settings from configuration files.
	// The last parameter is a constructor function returning an instance that
	// implements plugins.Plugin.
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		// Create and return a new RocketMQ client plugin instance
		return NewRocketMQClient()
	})
}

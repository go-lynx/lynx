package snowflake

import (
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init function is a special function in Go that is automatically executed when the package is loaded.
// This function registers the Snowflake ID generator plugin to the global plugin factory.
func init() {
	// Register the Snowflake plugin to the global plugin factory.
	// The first parameter PluginName is the unique plugin name used for identification.
	// The second parameter ConfPrefix is the configuration prefix, used to read plugin-related configuration from the config.
	// The third parameter is an anonymous function that returns an instance of plugins.Plugin interface type,
	// by calling the NewSnowflakePlugin function to create a new Snowflake plugin instance.
	factory.GlobalTypedFactory().RegisterPlugin(PluginName, ConfPrefix, func() plugins.Plugin {
		return NewSnowflakePlugin()
	})
}

// GetSnowflakeGenerator function is used to get the Snowflake generator instance.
// It gets the plugin manager through the global Lynx application instance, then gets the corresponding plugin instance by plugin name,
// finally converts the plugin instance to *PlugSnowflake type and returns it.
func GetSnowflakeGenerator() *PlugSnowflake {
	plugin := app.Lynx().GetPluginManager().GetPlugin(PluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*PlugSnowflake)
}

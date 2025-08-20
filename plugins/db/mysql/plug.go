package mysql

import (
	"entgo.io/ent/dialect/sql"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
)

// init is Go's initialization function, automatically executed when the package is loaded.
// This function's purpose is to register the MySQL client plugin to the global plugin factory.
func init() {
	// Get global plugin factory instance and call its RegisterPlugin method for plugin registration.
	// First parameter pluginName is the plugin name, used to uniquely identify the plugin.
	// Second parameter confPrefix is the configuration prefix, used to read plugin-related configuration from config.
	// Third parameter is an anonymous function that returns an instance of plugins.Plugin interface type,
	// here calling NewMysqlClient function to create a new MySQL client plugin instance.
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		return NewMysqlClient()
	})
}

// GetDriver function is used to get the database driver instance of MySQL client.
// Returns *sql.Driver type, which is the database driver pointer.
func GetDriver() *sql.Driver {
	// Get plugin manager from global Lynx application instance,
	// then get corresponding plugin instance by plugin name through plugin manager,
	// finally convert the obtained plugin instance to *DBMysqlClient type,
	// and return its dri field, which is the database driver instance.
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*DBMysqlClient).dri
}

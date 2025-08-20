package pgsql

import (
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/db/pgsql/conf"
)

// init is Go's initialization function that executes automatically when the package is loaded.
// The purpose of this function is to register the PgSQL client plugin into the global plugin factory.
func init() {
	// Get the global plugin factory instance and call its RegisterPlugin method to register the plugin.
	// The first parameter pluginName is the plugin name, used to uniquely identify the plugin.
	// The second parameter confPrefix is the configuration prefix, used to read plugin-related configuration from the config.
	// The third parameter is an anonymous function that returns an instance of the plugins.Plugin interface type,
	// here calling the NewPgsqlClient function to create a new PgSQL client plugin instance.
	factory.GlobalPluginFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		return NewPgsqlClient()
	})
}

// GetDriver function is used to get the database driver instance of the PgSQL client.
// The return value is of type *sql.Driver, which is a database driver pointer.
func GetDriver() *sql.Driver {
	// Get the plugin manager from the global Lynx application instance,
	// then get the corresponding plugin instance through the plugin manager by plugin name,
	// finally convert the obtained plugin instance to *DBPgsqlClient type,
	// and return its dri field, which is the database driver instance.
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*DBPgsqlClient).GetDriver()
}

// GetStats gets connection pool statistics
func GetStats() *ConnectionPoolStats {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*DBPgsqlClient).GetStats()
}

// GetConfig gets current configuration
func GetConfig() *conf.Pgsql {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*DBPgsqlClient).GetConfig()
}

// IsConnected checks if connected
func IsConnected() bool {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return false
	}
	return plugin.(*DBPgsqlClient).IsConnected()
}

// CheckHealth performs health check
func CheckHealth() error {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return fmt.Errorf("pgsql plugin not found")
	}
	return plugin.(*DBPgsqlClient).CheckHealth()
}

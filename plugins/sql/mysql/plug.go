package mysql

import (
	"fmt"

	"entgo.io/ent/dialect/sql"
	conf "github.com/go-lynx/lynx/api/plugins/sql/mysql"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/base"
)

// init is Go's initialization function, automatically executed when the package is loaded.
// This function's purpose is to register the MySQL package mysql

func init() {
    // Register the MySQL client plugin to the global plugin factory.
    // First parameter pluginName is the unique name of the plugin, used to identify the plugin in the system.
    // Second parameter confPrefix is the configuration prefix, used to read plugin-related configuration from config.
    // Third parameter is an anonymous function that returns an instance of plugins.Plugin interface type,
    // here calling NewMysqlClient function to create a new MySQL client plugin instance.
    factory.GlobalTypedFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
        return NewMysqlClient()
    })
}

// GetDriver function is used to get the database driver instance of MySQL client.
// Returns *sql.Driver type, which is the database driver pointer.
func GetDriver() *sql.Driver {
	// Get plugin manager from global Lynx application instance,
	// then get corresponding plugin instance by plugin name through plugin manager,
	// finally convert the obtained plugin instance to *DBMysqlClient type,
	// and return its driver through the GetDriver method.
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*DBMysqlClient).GetDriver()
}

// GetStats gets connection pool statistics
func GetStats() *base.ConnectionPoolStats {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	return plugin.(*DBMysqlClient).GetStats()
}

// GetConfig gets current configuration
func GetConfig() *conf.Mysql {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	mysqlClient := plugin.(*DBMysqlClient)
	if config, ok := mysqlClient.GetConfig().(*conf.Mysql); ok {
		return config
	}
	return nil
}

// IsConnected checks if connected
func IsConnected() bool {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return false
	}
	return plugin.(*DBMysqlClient).IsConnected()
}

// CheckHealth performs health check
func CheckHealth() error {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return fmt.Errorf("mysql plugin not found")
	}
	return plugin.(*DBMysqlClient).CheckHealth()
}

package pgsql

import (
	"context"
	"database/sql"
	"fmt"
	"entgo.io/ent/dialect"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/base"
	"github.com/go-lynx/lynx/plugins/sql/pgsql/conf"
)

// init is Go's initialization function that executes automatically when the package is loaded.
// The purpose of this function is to register the PgSQL package pgsql

func init() {
    // Register the PgSQL client plugin to the global plugin factory.
    // The first parameter pluginName is the unique name of the plugin used for identification.
    // The second parameter confPrefix is the configuration prefix, used to read plugin-related configuration from the config.
    // The third parameter is an anonymous function that returns an instance of the plugins.Plugin interface type,
    // here calling the NewPgsqlClient function to create a new PgSQL client plugin instance.
    factory.GlobalTypedFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
        return NewPgsqlClient()
    })
}

// GetDriver function is used to get the database driver instance of the PgSQL client.
// The return value is of type *sql.Driver, which is a database driver pointer.
func (p *DBPgsqlClient) GetDriver() dialect.Driver {
	return p.SQLPlugin.GetDriver()
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

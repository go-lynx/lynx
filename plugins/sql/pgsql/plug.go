package pgsql

import (
	"database/sql"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/interfaces"
)

// Plugin name constant
const pluginName = "pgsql.client"

// init function registers the PostgreSQL client plugin to the global plugin factory
func init() {
	factory.GlobalTypedFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		return NewPgsqlClient()
	})
}

// GetDB gets the database connection from the PostgreSQL plugin
func GetDB() (*sql.DB, error) {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil, nil
	}
	if sqlPlugin, ok := plugin.(interfaces.SQLPlugin); ok {
		return sqlPlugin.GetDB()
	}
	return nil, nil
}

// GetDialect gets the database dialect
func GetDialect() string {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return ""
	}
	if sqlPlugin, ok := plugin.(interfaces.SQLPlugin); ok {
		return sqlPlugin.GetDialect()
	}
	return ""
}

// IsConnected checks if the database is connected
func IsConnected() bool {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return false
	}
	if sqlPlugin, ok := plugin.(interfaces.SQLPlugin); ok {
		return sqlPlugin.IsConnected()
	}
	return false
}

// CheckHealth performs health check
func CheckHealth() error {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil
	}
	if sqlPlugin, ok := plugin.(interfaces.SQLPlugin); ok {
		return sqlPlugin.CheckHealth()
	}
	return nil
}

// GetDriver gets the ent SQL driver from the PostgreSQL plugin
func GetDriver() *entsql.Driver {
	db, err := GetDB()
	if err != nil || db == nil {
		return nil
	}
	return entsql.OpenDB(GetDialect(), db)
}
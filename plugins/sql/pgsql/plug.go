package pgsql

import (
	"database/sql"

	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins/sql/interfaces"
)

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
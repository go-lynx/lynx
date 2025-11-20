package mssql

import (
	"database/sql"
	"fmt"

	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/interfaces"
)

// init function registers the Microsoft SQL Server plugin to the global plugin factory.
// This function is automatically called when the package is imported.
// It creates a new DBMssqlClient instance and registers it to the plugin factory with the configured plugin name and configuration prefix.
func init() {
	// Call the RegisterPlugin method of the global plugin factory for plugin registration
	// Pass in the plugin name, configuration prefix, and a function that returns a plugins.Plugin interface instance
	factory.GlobalTypedFactory().RegisterPlugin(pluginName, confPrefix, func() plugins.Plugin {
		// Create and return a new DBMssqlClient instance
		return NewMssqlClient()
	})
}

// GetMssqlClient gets the Microsoft SQL Server client instance from the plugin manager.
// This function provides access to the underlying Microsoft SQL Server client for other parts of the application
// that may need to use database functionality.
//
// Returns:
//   - *DBMssqlClient: Configured Microsoft SQL Server client instance
//   - error: Any error that occurred while getting the client
//
// Note: This function will return an error if the plugin is not properly initialized or if the plugin manager cannot find the Microsoft SQL Server plugin.
func GetMssqlClient() (*DBMssqlClient, error) {
	// Get the plugin with the specified name from the application's plugin manager,
	// convert it to *DBMssqlClient type, and return it
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if client, ok := plugin.(*DBMssqlClient); ok {
		return client, nil
	}
	return nil, fmt.Errorf("failed to get MSSQL client: plugin not found or type assertion failed")
}

// GetMssqlDB gets the underlying database connection from the Microsoft SQL Server plugin.
// This function provides direct access to the database/sql.DB instance for advanced usage.
//
// Returns:
//   - *sql.DB: The underlying database connection
//   - error: Any error that occurred while getting the database connection
//
// Note: This function will return an error if the plugin is not properly initialized.
// Deprecated: Use GetDB() instead for consistency with other database plugins.
func GetMssqlDB() (*sql.DB, error) {
	return GetDB()
}

// GetDB gets the database connection from the MSSQL plugin.
// This function provides a unified API consistent with MySQL and PostgreSQL plugins.
func GetDB() (*sql.DB, error) {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return nil, fmt.Errorf("plugin %s not found", pluginName)
	}
	if sqlPlugin, ok := plugin.(interfaces.SQLPlugin); ok {
		return sqlPlugin.GetDB()
	}
	return nil, fmt.Errorf("plugin %s is not a SQLPlugin", pluginName)
}

// GetDialect gets the database dialect.
// This function provides a unified API consistent with MySQL and PostgreSQL plugins.
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

// IsConnected checks if the database is connected.
// This function provides a unified API consistent with MySQL and PostgreSQL plugins.
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

// CheckHealth performs health check.
// This function provides a unified API consistent with MySQL and PostgreSQL plugins.
func CheckHealth() error {
	plugin := app.Lynx().GetPluginManager().GetPlugin(pluginName)
	if plugin == nil {
		return fmt.Errorf("plugin %s not found", pluginName)
	}
	if sqlPlugin, ok := plugin.(interfaces.SQLPlugin); ok {
		return sqlPlugin.CheckHealth()
	}
	return fmt.Errorf("plugin %s is not a SQLPlugin", pluginName)
}

package mssql

import (
	"database/sql"

	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
	"github.com/go-lynx/lynx/plugins"
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
//
// Note: This function will panic if the plugin is not properly initialized or if the plugin manager cannot find the Microsoft SQL Server plugin.
func GetMssqlClient() *DBMssqlClient {
	// Get the plugin with the specified name from the application's plugin manager,
	// convert it to *DBMssqlClient type, and return it
	return app.Lynx().GetPluginManager().GetPlugin(pluginName).(*DBMssqlClient)
}

// GetMssqlDB gets the underlying database connection from the Microsoft SQL Server plugin.
// This function provides direct access to the database/sql.DB instance for advanced usage.
//
// Returns:
//   - *sql.DB: The underlying database connection
//   - error: Any error that occurred while getting the database connection
//
// Note: This function will panic if the plugin is not properly initialized.
func GetMssqlDB() (*sql.DB, error) {
	return GetMssqlClient().BaseSQLPlugin.GetDB()
}

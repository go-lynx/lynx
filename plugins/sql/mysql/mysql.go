package mysql

import (
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/base"
	"github.com/go-lynx/lynx/plugins/sql/interfaces"

	// MySQL driver
	_ "github.com/go-sql-driver/mysql"
)

// Plugin metadata
const (
	pluginName        = "mysql.client"
	pluginVersion     = "v2.0.0"
	pluginDescription = "mysql client plugin for lynx framework"
	confPrefix        = "lynx.mysql"
)

// DBMysqlClient represents MySQL client plugin instance
type DBMysqlClient struct {
	*base.SQLPlugin
	config *interfaces.Config
}

// NewMysqlClient creates a new MySQL client plugin instance
func NewMysqlClient() *DBMysqlClient {
	config := &interfaces.Config{
		Driver: "mysql",
		// Default connection pool settings
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 3600, // 1 hour
		ConnMaxIdleTime: 300,  // 5 minutes
		// Default health check settings
		HealthCheckInterval: 30, // 30 seconds
		HealthCheckQuery:    "SELECT 1",
	}

	c := &DBMysqlClient{
		config: config,
	}

	c.SQLPlugin = base.NewBaseSQLPlugin(
		plugins.GeneratePluginID("", pluginName, pluginVersion),
		pluginName,
		pluginDescription,
		pluginVersion,
		confPrefix,
		101,
		config,
	)

	return c
}

// StartupTasks initializes database connection
func (m *DBMysqlClient) StartupTasks() error {
	log.Infof("initializing mysql database connection")

	if err := m.SQLPlugin.StartupTasks(); err != nil {
		return err
	}

	log.Infof("mysql database successfully initialized with connection pool: max_open=%d, max_idle=%d",
		m.config.MaxOpenConns, m.config.MaxIdleConns)
	return nil
}

// CleanupTasks gracefully closes database connection
func (m *DBMysqlClient) CleanupTasks() error {
	log.Infof("closing mysql database connection")
	return m.SQLPlugin.CleanupTasks()
}

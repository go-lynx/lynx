package pgsql

import (
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/base"
	"github.com/go-lynx/lynx/plugins/sql/interfaces"
	
	// PostgreSQL driver
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Plugin metadata
const (
	pluginName        = "pgsql.client"
	pluginVersion     = "v2.0.0"
	pluginDescription = "pgsql client plugin for lynx framework"
	confPrefix        = "lynx.pgsql"
)

// DBPgsqlClient represents PostgreSQL client plugin instance
type DBPgsqlClient struct {
	*base.BaseSQLPlugin
	config *interfaces.Config
}

// NewPgsqlClient creates a new PostgreSQL client plugin instance
func NewPgsqlClient() *DBPgsqlClient {
	config := &interfaces.Config{
		Driver: "postgres",
		// Default connection pool settings
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 3600, // 1 hour
		ConnMaxIdleTime: 300,  // 5 minutes
		// Default health check settings
		HealthCheckInterval: 30,         // 30 seconds
		HealthCheckQuery:    "SELECT 1",
	}

	c := &DBPgsqlClient{
		config: config,
	}

	c.BaseSQLPlugin = base.NewBaseSQLPlugin(
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
func (p *DBPgsqlClient) StartupTasks() error {
	log.Infof("initializing pgsql database connection")
	
	if err := p.BaseSQLPlugin.StartupTasks(); err != nil {
		return err
	}
	
	log.Infof("pgsql database successfully initialized with connection pool: max_open=%d, max_idle=%d",
		p.config.MaxOpenConns, p.config.MaxIdleConns)
	return nil
}

// CleanupTasks gracefully closes database connection
func (p *DBPgsqlClient) CleanupTasks() error {
	log.Infof("closing pgsql database connection")
	return p.BaseSQLPlugin.CleanupTasks()
}
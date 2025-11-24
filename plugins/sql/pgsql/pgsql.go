package pgsql

import (
	"fmt"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/base"
	"github.com/go-lynx/lynx/plugins/sql/interfaces"
	"github.com/go-lynx/lynx/plugins/sql/pgsql/conf"

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
	*base.SQLPlugin
	config   *interfaces.Config
	pbConfig *conf.Pgsql // protobuf configuration
}

// NewPgsqlClient creates a new PostgreSQL client plugin instance
func NewPgsqlClient() *DBPgsqlClient {
	config := &interfaces.Config{
		Driver: "pgx",
		// Default connection pool settings
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 3600, // 1 hour
		ConnMaxIdleTime: 300,  // 5 minutes
		// Default health check settings
		HealthCheckInterval: 30, // 30 seconds
		HealthCheckQuery:    "SELECT 1",
	}

	c := &DBPgsqlClient{
		config:   config,
		pbConfig: &conf.Pgsql{},
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

// InitializeResources loads protobuf configuration and initializes resources
func (p *DBPgsqlClient) InitializeResources(rt plugins.Runtime) error {
	// Load protobuf configuration to a temporary variable first
	// This ensures we don't partially update p.pbConfig if loading fails
	pbConfig := &conf.Pgsql{}
	if err := rt.GetConfig().Value(confPrefix).Scan(pbConfig); err != nil {
		return fmt.Errorf("failed to load PostgreSQL configuration: %w", err)
	}

	// Only update p.pbConfig after successful loading
	p.pbConfig = pbConfig

	// Update interfaces.Config from protobuf config
	// This ensures atomic update - either all fields are updated or none
	p.config.Driver = pbConfig.Driver
	p.config.DSN = pbConfig.Source
	if pbConfig.MinConn > 0 {
		p.config.MaxIdleConns = int(pbConfig.MinConn)
	}
	if pbConfig.MaxConn > 0 {
		p.config.MaxOpenConns = int(pbConfig.MaxConn)
	}

	// Call parent initialization
	return p.SQLPlugin.InitializeResources(rt)
}

// StartupTasks initializes database connection
func (p *DBPgsqlClient) StartupTasks() error {
	log.Infof("initializing pgsql database connection")

	if err := p.SQLPlugin.StartupTasks(); err != nil {
		return err
	}

	log.Infof("pgsql database successfully initialized with connection pool: max_open=%d, max_idle=%d",
		p.config.MaxOpenConns, p.config.MaxIdleConns)
	return nil
}

// CleanupTasks gracefully closes database connection
func (p *DBPgsqlClient) CleanupTasks() error {
	log.Infof("closing pgsql database connection")
	return p.SQLPlugin.CleanupTasks()
}

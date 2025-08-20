package mysql

import (
	"context"
	_ "database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"

	esql "entgo.io/ent/dialect/sql"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/db/mysql/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Plugin metadata
const (
	// Plugin name
	pluginName = "mysql.client"
	// Plugin version
	pluginVersion = "v2.0.0"
	// Plugin description
	pluginDescription = "mysql client plugin for lynx framework"
	// Configuration prefix
	confPrefix = "lynx.mysql"
)

// DBMysqlClient represents MySQL client plugin instance
type DBMysqlClient struct {
	// Inherit base plugin
	*plugins.BasePlugin
	// Database driver
	dri *esql.Driver
	// MySQL configuration
	conf *conf.Mysql
}

// NewMysqlClient creates a new MySQL client plugin instance
// Returns a pointer to DBMysqlClient struct
func NewMysqlClient() *DBMysqlClient {
	return &DBMysqlClient{
		BasePlugin: plugins.NewBasePlugin(
			// Generate plugin ID
			plugins.GeneratePluginID("", pluginName, pluginVersion),
			// Plugin name
			pluginName,
			// Plugin description
			pluginDescription,
			// Plugin version
			pluginVersion,
			// Configuration prefix
			confPrefix,
			// Weight
			101,
		),
	}
}

// InitializeResources scans and loads MySQL configuration from runtime configuration
// Parameter rt is the runtime environment
// Returns error information, returns corresponding error if configuration loading fails
func (m *DBMysqlClient) InitializeResources(rt plugins.Runtime) error {
	// Initialize an empty configuration structure
	m.conf = &conf.Mysql{}

	// Scan and load MySQL configuration from runtime configuration
	err := rt.GetConfig().Value(confPrefix).Scan(m.conf)
	if err != nil {
		return err
	}

	// Set default configuration
	defaultConf := &conf.Mysql{
		Driver:      "mysql",
		Source:      "root:123456@tcp(127.0.0.1:3306)/db_name?charset=utf8mb4&parseTime=True&loc=Local",
		MinConn:     10,
		MaxConn:     20,
		MaxIdleTime: &durationpb.Duration{Seconds: 10},
		MaxLifeTime: &durationpb.Duration{Seconds: 300},
	}

	// Use default values for unset fields
	if m.conf.Driver == "" {
		m.conf.Driver = defaultConf.Driver
	}
	if m.conf.Source == "" {
		m.conf.Source = defaultConf.Source
	}
	if m.conf.MinConn == 0 {
		m.conf.MinConn = defaultConf.MinConn
	}
	if m.conf.MaxConn == 0 {
		m.conf.MaxConn = defaultConf.MaxConn
	}
	if m.conf.MaxIdleTime == nil {
		m.conf.MaxIdleTime = defaultConf.MaxIdleTime
	}
	if m.conf.MaxLifeTime == nil {
		m.conf.MaxLifeTime = defaultConf.MaxLifeTime
	}

	return nil
}

// StartupTasks initializes database connection and performs health check
// Returns error information, returns corresponding error if connection or health check fails
func (m *DBMysqlClient) StartupTasks() error {
	// Log database initialization
	log.Infof("Initializing database")

	// Open database connection
	drv, err := esql.Open(
		m.conf.Driver,
		m.conf.Source,
	)

	if err != nil {
		// Log database connection failure
		log.Errorf("failed opening connection to dataBase: %v", err)
		// Panic when error occurs
		panic(err)
	}

	// Set maximum idle connections in connection pool
	drv.DB().SetMaxIdleConns(int(m.conf.MinConn))
	// Set maximum open connections in connection pool
	drv.DB().SetMaxOpenConns(int(m.conf.MaxConn))
	// Set maximum idle time for connections
	drv.DB().SetConnMaxIdleTime(m.conf.MaxIdleTime.AsDuration())
	// Set maximum lifetime for connections
	drv.DB().SetConnMaxLifetime(m.conf.MaxLifeTime.AsDuration())

	// Assign database driver to instance
	m.dri = drv
	// Log successful database initialization
	log.Infof("database successfully initialized")
	// Original code had incorrect return value here, correctly return nil
	return nil
}

// CleanupTasks closes database connection
// Returns error information, returns corresponding error if closing connection fails
func (m *DBMysqlClient) CleanupTasks() error {
	if m.dri == nil {
		return nil
	}
	// Close database connection
	if err := m.dri.Close(); err != nil {
		// Log database connection close failure
		log.Error(err)
		return err
	}
	// Log database resource close
	log.Info("message", "Closing the DataBase resources")
	return nil
}

// Configure updates MySQL configuration.
// This function receives a parameter of any type, attempts to convert it to *conf.Mysql type, and updates configuration if conversion succeeds.
func (m *DBMysqlClient) Configure(c any) error {
	// Attempt to convert passed configuration to *conf.Mysql type
	if mysqlConf, ok := c.(*conf.Mysql); ok {
		// Conversion successful, update configuration
		m.conf = mysqlConf
		return nil
	}
	// Conversion failed, return invalid configuration error
	return plugins.ErrInvalidConfiguration
}

// CheckHealth performs health check on database connection.
// This function executes a ping context to check database connectivity.
func (m *DBMysqlClient) CheckHealth() error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Execute database connection health check
	err := m.dri.DB().PingContext(ctx)
	if err != nil {
		// Return error information
		return err
	}
	return nil
}

package mysql

import (
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/base"
	"github.com/go-lynx/lynx/plugins/sql/mysql/conf"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/types/known/durationpb"
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
	config            *conf.Mysql // Changed to config for consistency
	closeChan         chan struct{}
	closed            bool
	prometheusMetrics *PrometheusMetrics
}

// NewMysqlClient creates a new MySQL client plugin instance
func NewMysqlClient() *DBMysqlClient {
	mysqlConf := &conf.Mysql{
		Driver:      "mysql",
		Source:      "root:123456@tcp(127.0.0.1:3306)/db_name?charset=utf8mb4&parseTime=True&loc=Local",
		MinConn:     10,
		MaxConn:     20,
		MaxIdleTime: &durationpb.Duration{Seconds: 10},
		MaxLifeTime: &durationpb.Duration{Seconds: 300},
	}

	c := &DBMysqlClient{
		config:    mysqlConf,
		closeChan: make(chan struct{}),
		closed:    false,
	}

	c.SQLPlugin = base.NewSQLPlugin(
		plugins.GeneratePluginID("", pluginName, pluginVersion),
		pluginName,
		pluginDescription,
		pluginVersion,
		confPrefix,
		101,
		c.config,
	)
	return c
}

// Configure updates MySQL configuration.
func (m *DBMysqlClient) Configure(c any) error {
	if mysqlConf, ok := c.(*conf.Mysql); ok {
		m.config = mysqlConf
		return nil
	}
	return plugins.ErrInvalidConfiguration
}

// StartupTasks initializes database connection and performs health check
func (m *DBMysqlClient) StartupTasks() error {
	log.Infof("initializing mysql database connection")
	m.initPrometheusMetrics()
	if err := m.SQLPlugin.StartupTasks(); err != nil {
		return err
	}
	m.updateStats()
	log.Infof("mysql database successfully initialized with connection pool: max_open=%d, max_idle=%d",
		m.config.MaxConn, m.config.MinConn)
	return nil
}

// initPrometheusMetrics initializes Prometheus monitoring metrics
func (m *DBMysqlClient) initPrometheusMetrics() {
	promConfig := createPrometheusConfig(m.config)
	m.prometheusMetrics = NewPrometheusMetrics(promConfig)
}

// CheckHealth performs comprehensive health check on database connection
func (m *DBMysqlClient) CheckHealth() error {
	if err := m.SQLPlugin.CheckHealth(); err != nil {
		return err
	}
	m.updateStats()
	if m.prometheusMetrics != nil {
		m.prometheusMetrics.RecordHealthCheck(true, m.config)
	}
	return nil
}

// updateStats updates connection pool statistics
func (m *DBMysqlClient) updateStats() {
	stats := m.SQLPlugin.GetStats()
	if m.prometheusMetrics != nil && stats != nil {
		m.prometheusMetrics.UpdateMetrics(stats, m.config)
	}
}

// MetricsGatherer returns the Prometheus Gatherer for this plugin
func (m *DBMysqlClient) MetricsGatherer() prometheus.Gatherer {
	if m == nil || m.prometheusMetrics == nil {
		return nil
	}
	return m.prometheusMetrics.GetGatherer()
}

// GetConfig returns current configuration
func (m *DBMysqlClient) GetConfig() *conf.Mysql {
	return m.config
}

// GetStats returns connection pool statistics
func (m *DBMysqlClient) GetStats() *base.ConnectionPoolStats {
	return m.SQLPlugin.GetStats()
}

// IsConnected checks if database is connected
func (m *DBMysqlClient) IsConnected() bool {
	return !m.closed && m.SQLPlugin.IsConnected()
}

// CleanupTasks gracefully closes database connection
func (m *DBMysqlClient) CleanupTasks() error {
	if m.closed {
		return nil
	}
	log.Infof("closing mysql database connection")
	m.closed = true
	close(m.closeChan)
	return m.SQLPlugin.CleanupTasks()
}

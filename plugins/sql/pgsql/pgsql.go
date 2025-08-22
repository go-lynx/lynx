package pgsql

import (
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/base"
	"github.com/go-lynx/lynx/plugins/sql/pgsql/conf"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Plugin metadata
const (
	pluginName        = "pgsql.client"
	pluginVersion     = "v2.0.0"
	pluginDescription = "pgsql client plugin for lynx framework"
	confPrefix        = "lynx.pgsql"
)

// DBPgsqlClient represents a PgSQL client plugin instance
type DBPgsqlClient struct {
	*base.SQLPlugin
	conf              *conf.Pgsql
	closeChan         chan struct{}
	closed            bool
	prometheusMetrics *PrometheusMetrics
}


// NewPgsqlClient creates a new PgSQL client plugin instance
func NewPgsqlClient() *DBPgsqlClient {
	pgsqlConf := &conf.Pgsql{
		Driver:      "postgres",
		Source:      "postgres://admin:123456@127.0.0.1:5432/demo?sslmode=disable",
		MinConn:     10,
		MaxConn:     20,
		MaxIdleTime: &durationpb.Duration{Seconds: 10},
		MaxLifeTime: &durationpb.Duration{Seconds: 300},
	}

	c := &DBPgsqlClient{
		conf:      pgsqlConf,
		closeChan: make(chan struct{}),
	}

	c.SQLPlugin = base.NewSQLPlugin(
		plugins.GeneratePluginID("", pluginName, pluginVersion),
		pluginName,
		pluginDescription,
		pluginVersion,
		confPrefix,
		101,
		c.conf,
	)
	return c
}

// StartupTasks initializes database connection and performs health check
func (p *DBPgsqlClient) StartupTasks() error {
	log.Infof("initializing pgsql database connection")
	p.initPrometheusMetrics()
	if err := p.SQLPlugin.StartupTasks(); err != nil {
		return err
	}
	p.updateStats()
	log.Infof("pgsql database successfully initialized with connection pool: max_open=%d, max_idle=%d",
		p.conf.MaxConn, p.conf.MinConn)
	return nil
}

// CleanupTasks gracefully closes database connection
func (p *DBPgsqlClient) CleanupTasks() error {
	if p.closed {
		return nil
	}
	log.Infof("closing pgsql database connection")
	p.closed = true
	close(p.closeChan)
	return p.SQLPlugin.CleanupTasks()
}

// Configure updates PgSQL configuration
func (p *DBPgsqlClient) Configure(c any) error {
	if pgsqlConf, ok := c.(*conf.Pgsql); ok {
		p.conf = pgsqlConf
		return nil
	}
	return plugins.ErrInvalidConfiguration
}

// CheckHealth performs comprehensive health check on database connection
func (p *DBPgsqlClient) CheckHealth() error {
	if err := p.SQLPlugin.CheckHealth(); err != nil {
		return err
	}
	p.updateStats()
	if p.prometheusMetrics != nil {
		p.prometheusMetrics.RecordHealthCheck(true, p.conf)
	}
	return nil
}

// initPrometheusMetrics initializes Prometheus monitoring metrics
func (p *DBPgsqlClient) initPrometheusMetrics() {
	promConfig := createPrometheusConfig(p.conf)
	p.prometheusMetrics = NewPrometheusMetrics(promConfig)
	globalPgsqlMetrics = p.prometheusMetrics
	globalPgsqlConf = p.conf
}

// MetricsGatherer returns the Prometheus Gatherer for this plugin
func (p *DBPgsqlClient) MetricsGatherer() prometheus.Gatherer {
	if p == nil || p.prometheusMetrics == nil {
		return nil
	}
	return p.prometheusMetrics.GetGatherer()
}

// updateStats updates connection pool statistics
func (p *DBPgsqlClient) updateStats() {
	stats := p.SQLPlugin.GetStats()
	if p.prometheusMetrics != nil && stats != nil {
		p.prometheusMetrics.UpdateMetrics(stats, p.conf)
	}
}

// GetStats gets connection pool statistics
func (p *DBPgsqlClient) GetStats() *base.ConnectionPoolStats {
	return p.SQLPlugin.GetStats()
}

// GetConfig gets current configuration
func (p *DBPgsqlClient) GetConfig() *conf.Pgsql {
	return p.conf
}

// IsConnected checks if connected
func (p *DBPgsqlClient) IsConnected() bool {
	return !p.closed && p.SQLPlugin.IsConnected()
}


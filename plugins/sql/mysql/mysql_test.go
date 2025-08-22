package mysql

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins/sql/base"
	"github.com/go-lynx/lynx/plugins/sql/mysql/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestMySQLPlugin(t *testing.T) {
	// Create test configuration
	cfg := &conf.Mysql{
		Driver:      "mysql",
		Source:      "root:password@tcp(localhost:3306)/test?charset=utf8mb4&parseTime=True&loc=Local",
		MinConn:     5,
		MaxConn:     20,
		MaxLifeTime: durationpb.New(30 * time.Minute),
		MaxIdleTime: durationpb.New(10 * time.Minute),
		EnableStats: true,
		Debug:       false,
		HealthCheck: &conf.HealthCheck{
			Enabled:  true,
			Interval: durationpb.New(30 * time.Second),
			Timeout:  durationpb.New(5 * time.Second),
			Query:    "SELECT 1",
		},
		Monitoring: &conf.Monitoring{
			PrometheusEnabled: true,
			Namespace:         "lynx",
			Subsystem:         "mysql",
		},
	}

	// Create MySQL plugin instance
	plugin := &DBMysqlClient{
		SQLPlugin: &base.SQLPlugin{},
	}

	// Test configuration method
	t.Run("Configure", func(t *testing.T) {
		// This only tests configuration setting, not actual database connection
		plugin.config = cfg

		// Test configuration fields
		config := plugin.GetConfig()
		if config == nil {
			t.Error("Expected config to be non-nil")
		}
		if config.Driver != "mysql" {
			t.Errorf("Expected driver to be mysql, got %s", config.Driver)
		}
		if config.MinConn != 10 {
			t.Errorf("Expected MinConn to be 10, got %d", config.MinConn)
		}
		if config.MaxConn != 20 {
			t.Errorf("Expected MaxConn to be 20, got %d", config.MaxConn)
		}
	})

	// Test getting configuration
	t.Run("GetConfig", func(t *testing.T) {
		config := plugin.GetConfig()
		if config == nil {
			t.Error("Expected config to be non-nil")
		}
	})

	// Test health check configuration
	t.Run("HealthCheckConfig", func(t *testing.T) {
		if !cfg.HealthCheck.Enabled {
			t.Error("Expected health check to be enabled")
		}

		if cfg.HealthCheck.Query != "SELECT 1" {
			t.Errorf("Expected health check query to be 'SELECT 1', got %s", cfg.HealthCheck.Query)
		}
	})

	// Test monitoring configuration
	t.Run("MonitoringConfig", func(t *testing.T) {
		if !cfg.Monitoring.PrometheusEnabled {
			t.Error("Expected Prometheus monitoring to be enabled")
		}

		if cfg.Monitoring.Namespace != "lynx" {
			t.Errorf("Expected namespace to be 'lynx', got %s", cfg.Monitoring.Namespace)
		}
	})
}

func TestPrometheusMetrics(t *testing.T) {
	// Create test configuration
	cfg := &conf.Mysql{
		Driver:      "mysql",
		Source:      "test",
		EnableStats: true,
		Monitoring: &conf.Monitoring{
			PrometheusEnabled: true,
			Namespace:         "test",
			Subsystem:         "mysql",
		},
	}

	// Create Prometheus metrics instance
	metrics := NewPrometheusMetrics(cfg)

	t.Run("MetricsCreation", func(t *testing.T) {
		if metrics == nil {
			t.Fatal("Expected metrics to be non-nil")
		}

		if metrics.registry == nil {
			t.Error("Expected registry to be non-nil")
		}
	})

	t.Run("UpdateStats", func(t *testing.T) {
		// Create test statistics data
		stats := &base.DBStats{
			MaxOpenConnections: 20,
			OpenConnections:    10,
			InUse:              5,
			Idle:               5,
			WaitCount:          2,
			WaitDuration:       time.Second * 3,
			MaxIdleClosed:      1,
			MaxIdleTimeClosed:  0,
			MaxLifetimeClosed:  0,
		}

		// Test UpdateMetrics
		metrics.UpdateMetrics(stats, cfg)
	})

	t.Run("UpdateHealthCheck", func(t *testing.T) {
		// Test RecordHealthCheck
		metrics.RecordHealthCheck(true, cfg)

		// Test RecordHealthCheck with failure
		metrics.RecordHealthCheck(false, cfg)
	})
}

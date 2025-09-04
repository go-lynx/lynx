package sql

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins/sql/mysql"
	"github.com/go-lynx/lynx/plugins/sql/mysql/conf"
	"github.com/go-lynx/lynx/plugins/sql/pgsql"
	pgsqlconf "github.com/go-lynx/lynx/plugins/sql/pgsql/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// TestMySQLIntegration test MySQL plugin integration
func TestMySQLIntegration(t *testing.T) {
	// Create MySQL configuration
	mysqlConfig := &conf.Mysql{
		Driver:      "mysql",
		Source:      "root:password@tcp(localhost:3306)/test",
		MinConn:     5,
		MaxConn:     20,
		MaxLifeTime: durationpb.New(30 * time.Minute),
		MaxIdleTime: durationpb.New(10 * time.Minute),
		MaxIdleConn: 10,
	}

	// Create MySQL plugin instance
	mysqlPlugin := mysql.NewMySQLPlugin(mysqlConfig)
	if mysqlPlugin == nil {
		t.Fatal("Failed to create MySQL plugin")
	}

	// Test getting configuration
	config := mysqlPlugin.GetConfig()
	if config == nil {
		t.Fatal("MySQL config should not be nil")
	}

	// Test connection status
	if mysqlPlugin.IsConnected() {
		t.Error("MySQL should not be connected before Init")
	}

	t.Log("MySQL plugin basic test passed")
}

// TestPostgreSQLIntegration test PostgreSQL plugin integration
func TestPostgreSQLIntegration(t *testing.T) {
	// Create PostgreSQL configuration
	pgsqlConfig := &pgsqlconf.Pgsql{
		Driver:      "postgres",
		Source:      "host=localhost user=postgres password=password dbname=test sslmode=disable",
		MinConn:     5,
		MaxConn:     20,
		MaxLifeTime: durationpb.New(30 * time.Minute),
		MaxIdleTime: durationpb.New(10 * time.Minute),
		MaxIdleConn: 10,
	}

	// Create PostgreSQL plugin instance
	pgsqlPlugin := pgsql.NewPgSQLPlugin(pgsqlConfig)
	if pgsqlPlugin == nil {
		t.Fatal("Failed to create PostgreSQL plugin")
	}

	// Test getting configuration
	config := pgsqlPlugin.GetConfig()
	if config == nil {
		t.Fatal("PostgreSQL config should not be nil")
	}

	// Test connection status
	if pgsqlPlugin.IsConnected() {
		t.Error("PostgreSQL should not be connected before Init")
	}

	t.Log("PostgreSQL plugin basic test passed")
}

// TestPluginConsistency test interface consistency between two plugins
func TestPluginConsistency(t *testing.T) {
	// Create configurations
	mysqlConfig := &conf.Mysql{
		Driver:      "mysql",
		Source:      "root:password@tcp(localhost:3306)/test",
		MinConn:     5,
		MaxConn:     20,
		MaxLifeTime: durationpb.New(30 * time.Minute),
		MaxIdleTime: durationpb.New(10 * time.Minute),
		MaxIdleConn: 10,
	}

	pgsqlConfig := &pgsqlconf.Pgsql{
		Driver:      "postgres",
		Source:      "host=localhost user=postgres password=password dbname=test sslmode=disable",
		MinConn:     5,
		MaxConn:     20,
		MaxLifeTime: durationpb.New(30 * time.Minute),
		MaxIdleTime: durationpb.New(10 * time.Minute),
		MaxIdleConn: 10,
	}

	// Create plugin instances
	mysqlPlugin := mysql.NewMySQLPlugin(mysqlConfig)
	pgsqlPlugin := pgsql.NewPgSQLPlugin(pgsqlConfig)

	// Verify both plugins implement the same interface methods
	if mysqlPlugin == nil || pgsqlPlugin == nil {
		t.Fatal("Failed to create plugins")
	}

	// Test GetConfig method
	if mysqlPlugin.GetConfig() == nil || pgsqlPlugin.GetConfig() == nil {
		t.Error("GetConfig should return non-nil config")
	}

	// Test IsConnected method
	_ = mysqlPlugin.IsConnected()
	_ = pgsqlPlugin.IsConnected()

	// Test GetStats method
	mysqlStats := mysqlPlugin.GetStats()
	pgsqlStats := pgsqlPlugin.GetStats()
	if mysqlStats == nil || pgsqlStats == nil {
		t.Error("GetStats should return non-nil stats")
	}

	t.Log("Plugin interface consistency test passed")
}

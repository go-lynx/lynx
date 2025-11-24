//go:build integration
// +build integration

package pgsql

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/pgsql/conf"
)

// TestPostgreSQLIntegration tests PostgreSQL plugin with real database connection
func TestPostgreSQLIntegration(t *testing.T) {
	// Check if PostgreSQL is available
	if !isPostgreSQLAvailable() {
		t.Skip("PostgreSQL is not available, skipping integration test")
	}

	// Create a minimal runtime for testing
	rt := createTestRuntime(t)

	// Create PostgreSQL client
	client := NewPgsqlClient()

	// Initialize resources
	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	// Start plugin
	err = client.StartupTasks()
	if err != nil {
		t.Fatalf("StartupTasks failed: %v", err)
	}
	defer client.CleanupTasks()

	// Test connection
	if !client.IsConnected() {
		t.Error("PostgreSQL client should be connected")
	}

	// Test GetDB
	db, err := client.GetDB()
	if err != nil {
		t.Fatalf("GetDB failed: %v", err)
	}
	if db == nil {
		t.Fatal("GetDB returned nil")
	}

	// Test query execution
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result int
	err = db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("Query execution failed: %v", err)
	}

	if result != 1 {
		t.Errorf("Expected result 1, got %d", result)
	}

	// Test health check
	err = client.CheckHealth()
	if err != nil {
		t.Errorf("CheckHealth failed: %v", err)
	}

	// Test GetDialect
	dialect := client.GetDialect()
	if dialect != "postgres" {
		t.Errorf("Expected dialect 'postgres', got '%s'", dialect)
	}

	// Test GetStats
	stats := client.GetStats()
	if stats == nil {
		t.Error("GetStats returned nil")
	}
}

// TestPostgreSQLPluginIntegration tests PostgreSQL plugin through plugin interface
func TestPostgreSQLPluginIntegration(t *testing.T) {
	if !isPostgreSQLAvailable() {
		t.Skip("PostgreSQL is not available, skipping integration test")
	}

	rt := createTestRuntime(t)
	client := NewPgsqlClient()

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	err = client.StartupTasks()
	if err != nil {
		t.Fatalf("StartupTasks failed: %v", err)
	}
	defer client.CleanupTasks()

	// Test through plugin interface
	db, err := GetDB()
	if err != nil {
		t.Fatalf("GetDB failed: %v", err)
	}

	// Create test table
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS test_table (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert data
	var id int
	err = db.QueryRowContext(ctx, "INSERT INTO test_table (name) VALUES ($1) RETURNING id", "test").Scan(&id)
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}

	// Query data
	var name string
	err = db.QueryRowContext(ctx, "SELECT name FROM test_table WHERE id = $1", id).Scan(&name)
	if err != nil {
		t.Fatalf("Failed to query data: %v", err)
	}

	if name != "test" {
		t.Errorf("Expected name 'test', got '%s'", name)
	}

	// Cleanup
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS test_table")
}

// TestPostgreSQLConnectionPool tests connection pool functionality
func TestPostgreSQLConnectionPool(t *testing.T) {
	if !isPostgreSQLAvailable() {
		t.Skip("PostgreSQL is not available, skipping integration test")
	}

	rt := createTestRuntime(t)
	client := NewPgsqlClient()

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	err = client.StartupTasks()
	if err != nil {
		t.Fatalf("StartupTasks failed: %v", err)
	}
	defer client.CleanupTasks()

	db, err := client.GetDB()
	if err != nil {
		t.Fatalf("GetDB failed: %v", err)
	}

	// Test concurrent queries
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var result int
			err := db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
			if err != nil {
				t.Errorf("Concurrent query failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all queries
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check stats
	stats := client.GetStats()
	if stats.MaxOpenConnections == 0 {
		t.Error("MaxOpenConnections should be greater than 0")
	}
}

// TestPostgreSQLGetDriver tests GetDriver for Ent integration
func TestPostgreSQLGetDriver(t *testing.T) {
	if !isPostgreSQLAvailable() {
		t.Skip("PostgreSQL is not available, skipping integration test")
	}

	rt := createTestRuntime(t)
	client := NewPgsqlClient()

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	err = client.StartupTasks()
	if err != nil {
		t.Fatalf("StartupTasks failed: %v", err)
	}
	defer client.CleanupTasks()

	// Test GetDriver
	driver, err := GetDriver()
	if err != nil {
		t.Fatalf("GetDriver failed: %v", err)
	}
	if driver == nil {
		t.Error("GetDriver returned nil")
	}
}

// Helper functions

func isPostgreSQLAvailable() bool {
	db, err := sql.Open("pgx", "postgres://lynx:lynx123456@localhost:5432/lynx_test?sslmode=disable")
	if err != nil {
		return false
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return false
	}
	return true
}

func createTestRuntime(t *testing.T) plugins.Runtime {
	// Create a mock config
	mockConfig := &mockConfig{
		values: map[string]interface{}{
			"lynx.pgsql": &conf.Pgsql{
				Driver:  "pgx",
				Source:  "postgres://lynx:lynx123456@localhost:5432/lynx_test?sslmode=disable",
				MinConn: 5,
				MaxConn: 20,
			},
		},
	}

	// Create runtime
	rt := app.NewTypedRuntimePlugin()
	rt.SetConfig(mockConfig)

	return rt
}

// mockConfig implements config.Config for testing
type mockConfig struct {
	values map[string]interface{}
}

func (m *mockConfig) Value(key string) config.Value {
	return &mockValue{key: key, values: m.values}
}

type mockValue struct {
	key    string
	values map[string]interface{}
}

func (m *mockValue) Scan(dest interface{}) error {
	if val, ok := m.values[m.key]; ok {
		if pbConfig, ok := dest.(*conf.Pgsql); ok {
			if cfg, ok := val.(*conf.Pgsql); ok {
				*pbConfig = *cfg
				return nil
			}
		}
	}
	return nil
}

func (m *mockValue) String() (string, error) {
	return "", nil
}

func (m *mockValue) Bool() (bool, error) {
	return false, nil
}

func (m *mockValue) Int() (int64, error) {
	return 0, nil
}

func (m *mockValue) Float() (float64, error) {
	return 0, nil
}

func (m *mockValue) Duration() (time.Duration, error) {
	return 0, nil
}

func (m *mockConfig) Load() error {
	return nil
}

func (m *mockConfig) Watch(key string, o config.Observer) error {
	return nil
}

func (m *mockConfig) Close() error {
	return nil
}


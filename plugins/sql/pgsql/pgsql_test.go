package pgsql

import (
	"context"
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/pgsql/conf"
)

// mockRuntime is a mock implementation of plugins.Runtime for testing
type mockRuntime struct {
	config map[string]interface{}
}

func (m *mockRuntime) GetConfig() plugins.Config {
	return &mockConfig{values: m.config}
}

type mockConfig struct {
	values map[string]interface{}
}

func (m *mockConfig) Value(key string) plugins.Value {
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
	return nil // Return nil to use default config
}

func TestNewPgsqlClient(t *testing.T) {
	client := NewPgsqlClient()
	if client == nil {
		t.Fatal("NewPgsqlClient returned nil")
	}

	if client.Name() != pluginName {
		t.Errorf("Expected name '%s', got '%s'", pluginName, client.Name())
	}

	if client.config.Driver != "pgx" {
		t.Errorf("Expected driver 'pgx', got '%s'", client.config.Driver)
	}
}

func TestDBPgsqlClient_InitializeResources(t *testing.T) {
	client := NewPgsqlClient()

	pbConfig := &conf.Pgsql{
		Driver:  "pgx",
		Source:  "postgres://user:password@localhost:5432/testdb?sslmode=disable",
		MinConn: 5,
		MaxConn: 20,
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: pbConfig,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	if client.pbConfig.Driver != "pgx" {
		t.Errorf("Expected driver 'pgx', got '%s'", client.pbConfig.Driver)
	}

	if client.config.Driver != "pgx" {
		t.Errorf("Expected config driver 'pgx', got '%s'", client.config.Driver)
	}

	if client.config.DSN != pbConfig.Source {
		t.Errorf("Expected DSN '%s', got '%s'", pbConfig.Source, client.config.DSN)
	}

	if client.config.MaxIdleConns != int(pbConfig.MinConn) {
		t.Errorf("Expected MaxIdleConns %d, got %d", pbConfig.MinConn, client.config.MaxIdleConns)
	}

	if client.config.MaxOpenConns != int(pbConfig.MaxConn) {
		t.Errorf("Expected MaxOpenConns %d, got %d", pbConfig.MaxConn, client.config.MaxOpenConns)
	}
}

func TestDBPgsqlClient_InitializeResources_WithDefaults(t *testing.T) {
	client := NewPgsqlClient()

	// Test with empty config (should use defaults)
	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: &conf.Pgsql{},
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}
}

func TestDBPgsqlClient_StartupTasks(t *testing.T) {
	// Skip test if PostgreSQL is not available
	// This test requires a real PostgreSQL connection or will fail
	t.Skip("Skipping test that requires PostgreSQL connection")

	client := NewPgsqlClient()

	pbConfig := &conf.Pgsql{
		Driver:  "pgx",
		Source:  "postgres://user:password@localhost:5432/testdb?sslmode=disable",
		MinConn: 5,
		MaxConn: 20,
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: pbConfig,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	err = client.StartupTasks()
	if err != nil {
		t.Fatalf("StartupTasks failed: %v", err)
	}

	if !client.IsConnected() {
		t.Error("Client should be connected after StartupTasks")
	}

	// Cleanup
	_ = client.CleanupTasks()
}

func TestDBPgsqlClient_CleanupTasks(t *testing.T) {
	client := NewPgsqlClient()

	pbConfig := &conf.Pgsql{
		Driver: "pgx",
		Source: "postgres://user:password@localhost:5432/testdb?sslmode=disable",
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: pbConfig,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	// Cleanup should not fail even if not started
	err = client.CleanupTasks()
	if err != nil {
		t.Errorf("CleanupTasks failed: %v", err)
	}
}

func TestDBPgsqlClient_GetDB(t *testing.T) {
	// Test GetDB before initialization
	_, err := GetDB()
	if err == nil {
		t.Error("GetDB should return error when plugin not initialized")
	}
}

func TestDBPgsqlClient_IsConnected(t *testing.T) {
	// Test IsConnected before initialization
	if IsConnected() {
		t.Error("IsConnected should return false when plugin not initialized")
	}
}

func TestDBPgsqlClient_CheckHealth(t *testing.T) {
	// Test CheckHealth before initialization
	err := CheckHealth()
	if err == nil {
		t.Error("CheckHealth should return error when plugin not initialized")
	}
}

func TestDBPgsqlClient_GetDialect(t *testing.T) {
	client := NewPgsqlClient()

	pbConfig := &conf.Pgsql{
		Driver: "pgx",
		Source: "postgres://user:password@localhost:5432/testdb?sslmode=disable",
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: pbConfig,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	dialect := GetDialect()
	if dialect != "postgres" {
		t.Errorf("Expected dialect 'postgres', got '%s'", dialect)
	}
}

func TestDBPgsqlClient_GetDriver(t *testing.T) {
	// Test GetDriver before initialization
	_, err := GetDriver()
	if err == nil {
		t.Error("GetDriver should return error when plugin not initialized")
	}
}

func TestDBPgsqlClient_DefaultConfig(t *testing.T) {
	client := NewPgsqlClient()

	if client.config.Driver != "pgx" {
		t.Errorf("Expected default driver 'pgx', got '%s'", client.config.Driver)
	}

	if client.config.MaxOpenConns != 25 {
		t.Errorf("Expected default MaxOpenConns 25, got %d", client.config.MaxOpenConns)
	}

	if client.config.MaxIdleConns != 5 {
		t.Errorf("Expected default MaxIdleConns 5, got %d", client.config.MaxIdleConns)
	}

	if client.config.ConnMaxLifetime != 3600 {
		t.Errorf("Expected default ConnMaxLifetime 3600, got %d", client.config.ConnMaxLifetime)
	}

	if client.config.HealthCheckInterval != 30 {
		t.Errorf("Expected default HealthCheckInterval 30, got %d", client.config.HealthCheckInterval)
	}
}

func TestDBPgsqlClient_ConfigurationMapping(t *testing.T) {
	tests := []struct {
		name     string
		pbConfig *conf.Pgsql
		check    func(*testing.T, *DBPgsqlClient)
	}{
		{
			name: "min and max conns",
			pbConfig: &conf.Pgsql{
				Driver:  "pgx",
				Source:  "postgres://localhost/testdb",
				MinConn: 10,
				MaxConn: 50,
			},
			check: func(t *testing.T, client *DBPgsqlClient) {
				if client.config.MaxIdleConns != 10 {
					t.Errorf("Expected MaxIdleConns 10, got %d", client.config.MaxIdleConns)
				}
				if client.config.MaxOpenConns != 50 {
					t.Errorf("Expected MaxOpenConns 50, got %d", client.config.MaxOpenConns)
				}
			},
		},
		{
			name: "zero min conn",
			pbConfig: &conf.Pgsql{
				Driver:  "pgx",
				Source:  "postgres://localhost/testdb",
				MinConn: 0,
				MaxConn: 20,
			},
			check: func(t *testing.T, client *DBPgsqlClient) {
				// Should use default when MinConn is 0
				if client.config.MaxIdleConns < 0 {
					t.Errorf("MaxIdleConns should not be negative")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewPgsqlClient()

			rt := &mockRuntime{
				config: map[string]interface{}{
					confPrefix: tt.pbConfig,
				},
			}

			err := client.InitializeResources(rt)
			if err != nil {
				t.Fatalf("InitializeResources failed: %v", err)
			}

			tt.check(t, client)
		})
	}
}

func TestDBPgsqlClient_ConcurrentAccess(t *testing.T) {
	client := NewPgsqlClient()

	pbConfig := &conf.Pgsql{
		Driver:  "pgx",
		Source:  "postgres://user:password@localhost:5432/testdb?sslmode=disable",
		MinConn: 5,
		MaxConn: 20,
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: pbConfig,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	// Test that configuration is thread-safe
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_ = client.config.MaxOpenConns
			_ = client.pbConfig.MaxConn
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestDBPgsqlClient_ContextSupport(t *testing.T) {
	client := NewPgsqlClient()

	pbConfig := &conf.Pgsql{
		Driver: "pgx",
		Source: "postgres://user:password@localhost:5432/testdb?sslmode=disable",
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: pbConfig,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	// Test with context
	ctx := context.Background()
	_, err = client.GetDBWithContext(ctx)
	if err == nil {
		t.Error("GetDBWithContext should return error when not connected")
	}

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.GetDBWithContext(ctx)
	if err == nil {
		t.Error("GetDBWithContext should return error with cancelled context")
	}
}

func TestDBPgsqlClient_TimeoutHandling(t *testing.T) {
	client := NewPgsqlClient()

	pbConfig := &conf.Pgsql{
		Driver: "pgx",
		Source: "postgres://user:password@localhost:5432/testdb?sslmode=disable",
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: pbConfig,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	// Test with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = client.GetDBWithContext(ctx)
	if err == nil {
		t.Error("GetDBWithContext should return error when not connected")
	}
}

func TestDBPgsqlClient_PluginMetadata(t *testing.T) {
	client := NewPgsqlClient()

	if client.ID() == "" {
		t.Error("Plugin ID should not be empty")
	}

	if client.Name() != pluginName {
		t.Errorf("Expected plugin name '%s', got '%s'", pluginName, client.Name())
	}

	if client.Version() != pluginVersion {
		t.Errorf("Expected plugin version '%s', got '%s'", pluginVersion, client.Version())
	}

	if client.Description() != pluginDescription {
		t.Errorf("Expected plugin description '%s', got '%s'", pluginDescription, client.Description())
	}
}

func TestDBPgsqlClient_AtomicConfigUpdate(t *testing.T) {
	client := NewPgsqlClient()

	// Test that config update is atomic
	pbConfig1 := &conf.Pgsql{
		Driver:  "pgx",
		Source:  "postgres://localhost/db1",
		MinConn: 5,
		MaxConn: 10,
	}

	rt1 := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: pbConfig1,
		},
	}

	err := client.InitializeResources(rt1)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	// Verify config was set correctly
	if client.config.DSN != "postgres://localhost/db1" {
		t.Errorf("Expected DSN 'postgres://localhost/db1', got '%s'", client.config.DSN)
	}

	// Update config
	pbConfig2 := &conf.Pgsql{
		Driver:  "pgx",
		Source:  "postgres://localhost/db2",
		MinConn: 10,
		MaxConn: 20,
	}

	rt2 := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: pbConfig2,
		},
	}

	err = client.InitializeResources(rt2)
	if err != nil {
		t.Fatalf("InitializeResources failed on second call: %v", err)
	}

	// Verify config was updated correctly
	if client.config.DSN != "postgres://localhost/db2" {
		t.Errorf("Expected DSN 'postgres://localhost/db2', got '%s'", client.config.DSN)
	}
}

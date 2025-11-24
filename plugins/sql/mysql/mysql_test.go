package mysql

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/interfaces"
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
		if config, ok := dest.(*interfaces.Config); ok {
			if cfg, ok := val.(*interfaces.Config); ok {
				*config = *cfg
				return nil
			}
		}
	}
	return nil // Return nil to use default config
}

func TestNewMysqlClient(t *testing.T) {
	client := NewMysqlClient()
	if client == nil {
		t.Fatal("NewMysqlClient returned nil")
	}

	if client.Name() != pluginName {
		t.Errorf("Expected name '%s', got '%s'", pluginName, client.Name())
	}

	if client.config.Driver != "mysql" {
		t.Errorf("Expected driver 'mysql', got '%s'", client.config.Driver)
	}
}

func TestDBMysqlClient_InitializeResources(t *testing.T) {
	client := NewMysqlClient()

	config := &interfaces.Config{
		Driver:       "mysql",
		DSN:          "user:password@tcp(localhost:3306)/testdb",
		MaxOpenConns: 20,
		MaxIdleConns: 10,
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: config,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	if client.config.Driver != "mysql" {
		t.Errorf("Expected driver 'mysql', got '%s'", client.config.Driver)
	}
}

func TestDBMysqlClient_StartupTasks(t *testing.T) {
	// Skip test if MySQL is not available
	// This test requires a real MySQL connection or will fail
	t.Skip("Skipping test that requires MySQL connection")

	client := NewMysqlClient()

	config := &interfaces.Config{
		Driver:              "mysql",
		DSN:                 "root:password@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True",
		MaxOpenConns:        10,
		MaxIdleConns:        5,
		HealthCheckInterval: 0,
		AutoReconnectInterval: 0,
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: config,
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

func TestDBMysqlClient_CleanupTasks(t *testing.T) {
	client := NewMysqlClient()

	config := &interfaces.Config{
		Driver:              "mysql",
		DSN:                 "user:password@tcp(localhost:3306)/testdb",
		MaxOpenConns:        10,
		MaxIdleConns:        5,
		HealthCheckInterval: 0,
		AutoReconnectInterval: 0,
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: config,
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

func TestDBMysqlClient_GetDB(t *testing.T) {
	client := NewMysqlClient()

	// Test GetDB before initialization
	_, err := GetDB()
	if err == nil {
		t.Error("GetDB should return error when plugin not initialized")
	}
}

func TestDBMysqlClient_IsConnected(t *testing.T) {
	client := NewMysqlClient()

	// Test IsConnected before initialization
	if IsConnected() {
		t.Error("IsConnected should return false when plugin not initialized")
	}
}

func TestDBMysqlClient_CheckHealth(t *testing.T) {
	// Test CheckHealth before initialization
	err := CheckHealth()
	if err == nil {
		t.Error("CheckHealth should return error when plugin not initialized")
	}
}

func TestDBMysqlClient_GetDialect(t *testing.T) {
	client := NewMysqlClient()

	config := &interfaces.Config{
		Driver:              "mysql",
		DSN:                 "user:password@tcp(localhost:3306)/testdb",
		MaxOpenConns:        10,
		MaxIdleConns:        5,
		HealthCheckInterval: 0,
		AutoReconnectInterval: 0,
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: config,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	dialect := GetDialect()
	if dialect != "mysql" {
		t.Errorf("Expected dialect 'mysql', got '%s'", dialect)
	}
}

func TestDBMysqlClient_DefaultConfig(t *testing.T) {
	client := NewMysqlClient()

	if client.config.Driver != "mysql" {
		t.Errorf("Expected default driver 'mysql', got '%s'", client.config.Driver)
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

func TestDBMysqlClient_ConfigurationValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *interfaces.Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &interfaces.Config{
				Driver:       "mysql",
				DSN:          "user:password@tcp(localhost:3306)/testdb",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			wantErr: false,
		},
		{
			name: "missing DSN",
			config: &interfaces.Config{
				Driver:       "mysql",
				MaxOpenConns: 10,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewMysqlClient()

			rt := &mockRuntime{
				config: map[string]interface{}{
					confPrefix: tt.config,
				},
			}

			err := client.InitializeResources(rt)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitializeResources() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDBMysqlClient_ConcurrentAccess(t *testing.T) {
	client := NewMysqlClient()

	config := &interfaces.Config{
		Driver:              "mysql",
		DSN:                 "user:password@tcp(localhost:3306)/testdb",
		MaxOpenConns:        20,
		MaxIdleConns:        10,
		HealthCheckInterval: 0,
		AutoReconnectInterval: 0,
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: config,
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
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestDBMysqlClient_ContextSupport(t *testing.T) {
	client := NewMysqlClient()

	config := &interfaces.Config{
		Driver:              "mysql",
		DSN:                 "user:password@tcp(localhost:3306)/testdb",
		MaxOpenConns:        10,
		MaxIdleConns:        5,
		HealthCheckInterval: 0,
		AutoReconnectInterval: 0,
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: config,
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

func TestDBMysqlClient_TimeoutHandling(t *testing.T) {
	client := NewMysqlClient()

	config := &interfaces.Config{
		Driver:              "mysql",
		DSN:                 "user:password@tcp(localhost:3306)/testdb",
		MaxOpenConns:        10,
		MaxIdleConns:        5,
		HealthCheckInterval: 0,
		AutoReconnectInterval: 0,
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: config,
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

func TestDBMysqlClient_PluginMetadata(t *testing.T) {
	client := NewMysqlClient()

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


package mssql

import (
	"context"
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/mssql/conf"
	"google.golang.org/protobuf/types/known/durationpb"
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
		if mssqlConfig, ok := dest.(*conf.Mssql); ok {
			if cfg, ok := val.(*conf.Mssql); ok {
				*mssqlConfig = *cfg
				return nil
			}
		}
	}
	return nil // Return nil to use default config
}

func TestNewMssqlClient(t *testing.T) {
	client := NewMssqlClient()
	if client == nil {
		t.Fatal("NewMssqlClient returned nil")
	}

	if client.Name() != pluginName {
		t.Errorf("Expected name '%s', got '%s'", pluginName, client.Name())
	}

	if client.config.Driver != "mssql" {
		t.Errorf("Expected driver 'mssql', got '%s'", client.config.Driver)
	}
}

func TestDBMssqlClient_Configure(t *testing.T) {
	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver:  "mssql",
		Source:  "server=localhost;port=1433;database=testdb",
		MinConn: 5,
		MaxConn: 20,
	}

	err := client.Configure(mssqlConfig)
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}

	if client.config.Driver != "mssql" {
		t.Errorf("Expected driver 'mssql', got '%s'", client.config.Driver)
	}
}

func TestDBMssqlClient_Configure_InvalidType(t *testing.T) {
	client := NewMssqlClient()

	err := client.Configure("invalid")
	if err == nil {
		t.Error("Configure should return error for invalid type")
	}
}

func TestDBMssqlClient_InitializeResources(t *testing.T) {
	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver:      "mssql",
		Source:      "server=localhost;port=1433;database=testdb",
		MinConn:     5,
		MaxConn:     20,
		MaxIdleTime: &durationpb.Duration{Seconds: 300},
		MaxLifeTime: &durationpb.Duration{Seconds: 3600},
		ServerConfig: &conf.ServerConfig{
			InstanceName: "localhost",
			Port:         1433,
			Database:     "testdb",
		},
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: mssqlConfig,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	if client.config.Driver != "mssql" {
		t.Errorf("Expected driver 'mssql', got '%s'", client.config.Driver)
	}
}

func TestDBMssqlClient_InitializeResources_WithServerConfig(t *testing.T) {
	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver: "mssql",
		ServerConfig: &conf.ServerConfig{
			InstanceName:           "localhost",
			Port:                   1433,
			Database:               "testdb",
			Encrypt:                true,
			TrustServerCertificate: false,
			ConnectionTimeout:      30,
			CommandTimeout:         30,
			ApplicationName:        "Lynx-MSSQL-Test",
			ConnectionPooling:      true,
			MaxPoolSize:            20,
			MinPoolSize:            5,
		},
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: mssqlConfig,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	// Verify DSN was built from ServerConfig
	if client.config.Source == "" {
		t.Error("DSN should be built from ServerConfig")
	}
}

func TestDBMssqlClient_InitializeResources_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *conf.Mssql
		wantErr bool
	}{
		{
			name: "missing driver",
			config: &conf.Mssql{
				Source: "server=localhost",
			},
			wantErr: true,
		},
		{
			name: "missing source and server config",
			config: &conf.Mssql{
				Driver: "mssql",
			},
			wantErr: true,
		},
		{
			name: "valid with source",
			config: &conf.Mssql{
				Driver: "mssql",
				Source: "server=localhost;database=testdb",
			},
			wantErr: false,
		},
		{
			name: "valid with server config",
			config: &conf.Mssql{
				Driver: "mssql",
				ServerConfig: &conf.ServerConfig{
					InstanceName: "localhost",
					Database:     "testdb",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewMssqlClient()

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

func TestDBMssqlClient_StartupTasks(t *testing.T) {
	// Skip test if MSSQL is not available
	// This test requires a real MSSQL connection or will fail
	t.Skip("Skipping test that requires MSSQL connection")

	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver:  "mssql",
		Source:  "server=localhost;port=1433;database=testdb;user id=sa;password=Password123",
		MinConn: 5,
		MaxConn: 20,
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: mssqlConfig,
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

func TestDBMssqlClient_CleanupTasks(t *testing.T) {
	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver: "mssql",
		Source: "server=localhost;port=1433;database=testdb",
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: mssqlConfig,
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

	// Test double cleanup
	err = client.CleanupTasks()
	if err != nil {
		t.Errorf("Second CleanupTasks should not fail: %v", err)
	}
}

func TestDBMssqlClient_CheckHealth(t *testing.T) {
	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver: "mssql",
		Source: "server=localhost;port=1433;database=testdb",
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: mssqlConfig,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	// Test CheckHealth before connection
	err = client.CheckHealth()
	if err == nil {
		t.Error("CheckHealth should return error when not connected")
	}
}

func TestDBMssqlClient_IsConnected(t *testing.T) {
	client := NewMssqlClient()

	// Test IsConnected before initialization
	if client.IsConnected() {
		t.Error("IsConnected should return false when not initialized")
	}
}

func TestDBMssqlClient_GetMssqlConfig(t *testing.T) {
	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver: "mssql",
		Source: "server=localhost;port=1433;database=testdb",
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: mssqlConfig,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	config := client.GetMssqlConfig()
	if config == nil {
		t.Error("GetMssqlConfig should not return nil")
	}

	if config.Driver != "mssql" {
		t.Errorf("Expected driver 'mssql', got '%s'", config.Driver)
	}
}

func TestDBMssqlClient_TestConnection(t *testing.T) {
	// Skip test if MSSQL is not available
	t.Skip("Skipping test that requires MSSQL connection")

	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver: "mssql",
		Source: "server=localhost;port=1433;database=testdb;user id=sa;password=Password123",
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: mssqlConfig,
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

	ctx := context.Background()
	err = client.TestConnection(ctx)
	if err != nil {
		t.Fatalf("TestConnection failed: %v", err)
	}

	// Cleanup
	_ = client.CleanupTasks()
}

func TestDBMssqlClient_GetServerInfo(t *testing.T) {
	// Skip test if MSSQL is not available
	t.Skip("Skipping test that requires MSSQL connection")

	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver: "mssql",
		Source: "server=localhost;port=1433;database=testdb;user id=sa;password=Password123",
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: mssqlConfig,
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

	ctx := context.Background()
	info, err := client.GetServerInfo(ctx)
	if err != nil {
		t.Fatalf("GetServerInfo failed: %v", err)
	}

	if info == nil {
		t.Error("GetServerInfo should not return nil")
	}

	// Cleanup
	_ = client.CleanupTasks()
}

func TestDBMssqlClient_GetConnectionStats(t *testing.T) {
	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver: "mssql",
		Source: "server=localhost;port=1433;database=testdb",
		ServerConfig: &conf.ServerConfig{
			InstanceName: "localhost",
			Port:         1433,
			Database:     "testdb",
		},
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: mssqlConfig,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	stats := client.GetConnectionStats()
	if stats == nil {
		t.Error("GetConnectionStats should not return nil")
	}

	if stats["driver"] != "mssql" {
		t.Errorf("Expected driver 'mssql', got '%v'", stats["driver"])
	}
}

func TestDBMssqlClient_BuildDSN(t *testing.T) {
	tests := []struct {
		name   string
		config *conf.Mssql
		check  func(*testing.T, string)
	}{
		{
			name: "basic server config",
			config: &conf.Mssql{
				Driver: "mssql",
				ServerConfig: &conf.ServerConfig{
					InstanceName: "localhost",
					Port:         1433,
					Database:     "testdb",
				},
			},
			check: func(t *testing.T, dsn string) {
				if dsn == "" {
					t.Error("DSN should not be empty")
				}
			},
		},
		{
			name: "with encryption",
			config: &conf.Mssql{
				Driver: "mssql",
				ServerConfig: &conf.ServerConfig{
					InstanceName:           "localhost",
					Port:                   1433,
					Database:               "testdb",
					Encrypt:                true,
					TrustServerCertificate: true,
				},
			},
			check: func(t *testing.T, dsn string) {
				if dsn == "" {
					t.Error("DSN should not be empty")
				}
			},
		},
		{
			name: "with connection pooling",
			config: &conf.Mssql{
				Driver: "mssql",
				ServerConfig: &conf.ServerConfig{
					InstanceName:      "localhost",
					Port:              1433,
					Database:          "testdb",
					ConnectionPooling: true,
					MaxPoolSize:       20,
					MinPoolSize:       5,
				},
			},
			check: func(t *testing.T, dsn string) {
				if dsn == "" {
					t.Error("DSN should not be empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewMssqlClient()

			rt := &mockRuntime{
				config: map[string]interface{}{
					confPrefix: tt.config,
				},
			}

			err := client.InitializeResources(rt)
			if err != nil {
				t.Fatalf("InitializeResources failed: %v", err)
			}

			tt.check(t, client.config.Source)
		})
	}
}

func TestDBMssqlClient_ContextSupport(t *testing.T) {
	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver: "mssql",
		Source: "server=localhost;port=1433;database=testdb",
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: mssqlConfig,
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

func TestDBMssqlClient_TimeoutHandling(t *testing.T) {
	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver: "mssql",
		Source: "server=localhost;port=1433;database=testdb",
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: mssqlConfig,
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

func TestDBMssqlClient_PluginMetadata(t *testing.T) {
	client := NewMssqlClient()

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

func TestDBMssqlClient_BackgroundTasks(t *testing.T) {
	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver: "mssql",
		Source: "server=localhost;port=1433;database=testdb",
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: mssqlConfig,
		},
	}

	err := client.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources failed: %v", err)
	}

	err = client.StartupTasks()
	if err != nil {
		// Startup may fail without real database, that's OK for this test
		t.Logf("StartupTasks failed (expected): %v", err)
	}

	// Wait a bit for background tasks to start
	time.Sleep(100 * time.Millisecond)

	// Cleanup should stop background tasks
	err = client.CleanupTasks()
	if err != nil {
		t.Errorf("CleanupTasks failed: %v", err)
	}

	// Wait a bit to ensure background tasks are stopped
	time.Sleep(100 * time.Millisecond)
}

func TestDBMssqlClient_DefaultConfig(t *testing.T) {
	client := NewMssqlClient()

	if client.config.Driver != "mssql" {
		t.Errorf("Expected default driver 'mssql', got '%s'", client.config.Driver)
	}

	if client.config.MinConn != 5 {
		t.Errorf("Expected default MinConn 5, got %d", client.config.MinConn)
	}

	if client.config.MaxConn != 20 {
		t.Errorf("Expected default MaxConn 20, got %d", client.config.MaxConn)
	}

	if client.config.ServerConfig == nil {
		t.Error("ServerConfig should not be nil")
	}

	if client.config.ServerConfig.InstanceName != "localhost" {
		t.Errorf("Expected default InstanceName 'localhost', got '%s'", client.config.ServerConfig.InstanceName)
	}

	if client.config.ServerConfig.Port != 1433 {
		t.Errorf("Expected default Port 1433, got %d", client.config.ServerConfig.Port)
	}
}

func TestDBMssqlClient_ConvertToBaseConfig(t *testing.T) {
	mssqlConfig := &conf.Mssql{
		Driver:      "mssql",
		Source:      "server=localhost;database=testdb",
		MinConn:     10,
		MaxConn:     50,
		MaxIdleTime: &durationpb.Duration{Seconds: 300},
		MaxLifeTime: &durationpb.Duration{Seconds: 3600},
	}

	baseConfig := convertToBaseConfig(mssqlConfig)

	if baseConfig.Driver != "mssql" {
		t.Errorf("Expected driver 'mssql', got '%s'", baseConfig.Driver)
	}

	if baseConfig.MaxOpenConns != 50 {
		t.Errorf("Expected MaxOpenConns 50, got %d", baseConfig.MaxOpenConns)
	}

	if baseConfig.MaxIdleConns != 10 {
		t.Errorf("Expected MaxIdleConns 10, got %d", baseConfig.MaxIdleConns)
	}

	if baseConfig.ConnMaxLifetime != 3600 {
		t.Errorf("Expected ConnMaxLifetime 3600, got %d", baseConfig.ConnMaxLifetime)
	}

	if baseConfig.ConnMaxIdleTime != 300 {
		t.Errorf("Expected ConnMaxIdleTime 300, got %d", baseConfig.ConnMaxIdleTime)
	}
}

func TestDBMssqlClient_BeginTransaction(t *testing.T) {
	// Skip test if MSSQL is not available
	t.Skip("Skipping test that requires MSSQL connection")

	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver: "mssql",
		Source: "server=localhost;port=1433;database=testdb;user id=sa;password=Password123",
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: mssqlConfig,
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

	ctx := context.Background()
	tx, err := client.BeginTransaction(ctx)
	if err != nil {
		t.Fatalf("BeginTransaction failed: %v", err)
	}

	if tx == nil {
		t.Error("BeginTransaction should not return nil")
	}

	// Rollback transaction
	_ = tx.Rollback()

	// Cleanup
	_ = client.CleanupTasks()
}

func TestDBMssqlClient_ExecuteStoredProcedure(t *testing.T) {
	// Skip test if MSSQL is not available
	t.Skip("Skipping test that requires MSSQL connection")

	client := NewMssqlClient()

	mssqlConfig := &conf.Mssql{
		Driver: "mssql",
		Source: "server=localhost;port=1433;database=testdb;user id=sa;password=Password123",
	}

	rt := &mockRuntime{
		config: map[string]interface{}{
			confPrefix: mssqlConfig,
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

	ctx := context.Background()
	rows, err := client.ExecuteStoredProcedure(ctx, "sp_helpdb", "master")
	if err != nil {
		t.Fatalf("ExecuteStoredProcedure failed: %v", err)
	}

	if rows == nil {
		t.Error("ExecuteStoredProcedure should not return nil")
	}

	_ = rows.Close()

	// Cleanup
	_ = client.CleanupTasks()
}

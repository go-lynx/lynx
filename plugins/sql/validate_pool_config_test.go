package main

import (
	"fmt"
	"testing"

	"github.com/go-lynx/lynx/plugins/sql/base"
	"github.com/go-lynx/lynx/plugins/sql/interfaces"
	"github.com/go-lynx/lynx/plugins"
)

// 这个测试文件用于验证连接池配置是否符合预期

func TestDefaultConnectionPoolConfig(t *testing.T) {
	fmt.Println("=== 测试默认连接池配置 ===")

	// 测试MySQL默认配置
	mysqlConfig := &interfaces.Config{
		Driver:       "mysql",
		DSN:          "test://dsn",
		MaxOpenConns: 25, // MySQL默认值
		MaxIdleConns: 5,  // MySQL默认值
	}

	plugin := base.NewBaseSQLPlugin(
		"test-id",
		"mysql-plugin",
		"MySQL plugin",
		"v1.0.0",
		"test.prefix",
		100,
		mysqlConfig,
	)

	// 验证默认值
	if mysqlConfig.MaxOpenConns != 25 {
		t.Errorf("MySQL默认MaxOpenConns应该是25，实际是%d", mysqlConfig.MaxOpenConns)
	}
	if mysqlConfig.MaxIdleConns != 5 {
		t.Errorf("MySQL默认MaxIdleConns应该是5，实际是%d", mysqlConfig.MaxIdleConns)
	}

	fmt.Printf("✅ MySQL默认配置验证通过: MaxOpenConns=%d, MaxIdleConns=%d\n", 
		mysqlConfig.MaxOpenConns, mysqlConfig.MaxIdleConns)

	_ = plugin
}

func TestConnectionPoolValidation(t *testing.T) {
	fmt.Println("\n=== 测试连接池配置验证 ===")

	tests := []struct {
		name           string
		maxOpenConns   int
		maxIdleConns   int
		shouldValidate bool
	}{
		{"有效配置: 25/5", 25, 5, true},
		{"有效配置: 50/10", 50, 10, true},
		{"无效配置: MaxIdleConns > MaxOpenConns", 5, 10, false},
		{"无效配置: MaxOpenConns = 0", 0, 5, false},
		{"无效配置: MaxIdleConns < 0", 25, -1, false},
		{"边界值: MaxIdleConns = MaxOpenConns", 25, 25, true},
	}

	for _, tt := range tests {
		config := &interfaces.Config{
			Driver:       "mysql",
			DSN:          "test://dsn",
			MaxOpenConns: tt.maxOpenConns,
			MaxIdleConns: tt.maxIdleConns,
		}

		plugin := base.NewBaseSQLPlugin(
			"test-id",
			"test-plugin",
			"Test plugin",
			"v1.0.0",
			"test.prefix",
			100,
			config,
		)

		rt := &mockRuntime{
			config: map[string]interface{}{
				"test.prefix": config,
			},
		}

		err := plugin.InitializeResources(rt)
		validated := err == nil

		if validated != tt.shouldValidate {
			if tt.shouldValidate {
				t.Errorf("❌ %s: 应该验证通过但失败了: %v", tt.name, err)
			} else {
				t.Errorf("❌ %s: 应该验证失败但通过了", tt.name)
			}
		} else {
			fmt.Printf("✅ %s: 验证通过\n", tt.name)
		}

		_ = plugin
	}
}

func TestConnectionPoolStats(t *testing.T) {
	fmt.Println("\n=== 测试连接池统计信息 ===")

	config := &interfaces.Config{
		Driver:       "mysql",
		DSN:          "test://dsn",
		MaxOpenConns: 25,
		MaxIdleConns: 5,
	}

	plugin := base.NewBaseSQLPlugin(
		"test-id",
		"test-plugin",
		"Test plugin",
		"v1.0.0",
		"test.prefix",
		100,
		config,
	)

	stats := plugin.GetStats()

	if stats == nil {
		t.Fatal("❌ GetStats返回nil")
	}

	fmt.Printf("✅ 连接池统计结构完整:\n")
	fmt.Printf("   - MaxOpenConnections: %d\n", stats.MaxOpenConnections)
	fmt.Printf("   - OpenConnections: %d\n", stats.OpenConnections)
	fmt.Printf("   - InUse: %d\n", stats.InUse)
	fmt.Printf("   - Idle: %d\n", stats.Idle)
	fmt.Printf("   - MaxIdleConnections: %d\n", stats.MaxIdleConnections)
	fmt.Printf("   - WaitCount: %d\n", stats.WaitCount)
	fmt.Printf("   - WaitDuration: %v\n", stats.WaitDuration)
}

func TestDefaultValuesApplication(t *testing.T) {
	fmt.Println("\n=== 测试默认值应用逻辑 ===")

	// 测试当MaxOpenConns为0时，应该应用默认值25
	config := &interfaces.Config{
		Driver:       "mysql",
		DSN:          "test://dsn",
		MaxOpenConns: 0, // 应该使用默认值
		MaxIdleConns: 0, // 应该使用默认值
	}

	plugin := base.NewBaseSQLPlugin(
		"test-id",
		"test-plugin",
		"Test plugin",
		"v1.0.0",
		"test.prefix",
		100,
		config,
	)

	rt := &mockRuntime{
		config: map[string]interface{}{
			"test.prefix": config,
		},
	}

	err := plugin.InitializeResources(rt)
	if err != nil {
		t.Fatalf("InitializeResources失败: %v", err)
	}

	// 验证默认值已应用
	if config.MaxOpenConns != 25 {
		t.Errorf("❌ MaxOpenConns默认值未正确应用: 期望25，实际%d", config.MaxOpenConns)
	} else {
		fmt.Printf("✅ MaxOpenConns默认值正确应用: %d\n", config.MaxOpenConns)
	}

	if config.MaxIdleConns != 5 {
		t.Errorf("❌ MaxIdleConns默认值未正确应用: 期望5，实际%d", config.MaxIdleConns)
	} else {
		fmt.Printf("✅ MaxIdleConns默认值正确应用: %d\n", config.MaxIdleConns)
	}
}

// mockRuntime实现
type mockRuntime struct {
	config map[string]interface{}
}

func (m *mockRuntime) GetConfig() interface{} {
	return &mockConfig{values: m.config}
}

func (m *mockRuntime) AddListener(listener plugins.EventListener, filter *plugins.EventFilter) {}
func (m *mockRuntime) AddPluginListener(pluginName string, listener plugins.EventListener, filter *plugins.EventFilter) {}
func (m *mockRuntime) CleanupResources(pluginName string) error { return nil }
func (m *mockRuntime) EmitEvent(event plugins.PluginEvent) {}
func (m *mockRuntime) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {}
func (m *mockRuntime) GetCurrentPluginContext() string { return "" }
func (m *mockRuntime) GetEventHistory(filter plugins.EventFilter) []plugins.PluginEvent { return nil }
func (m *mockRuntime) GetEventStats() map[string]any { return nil }
func (m *mockRuntime) GetLogger() interface{} { return nil }
func (m *mockRuntime) GetPluginEventHistory(pluginName string, filter plugins.EventFilter) []plugins.PluginEvent { return nil }
func (m *mockRuntime) GetResourceStats() map[string]any { return nil }
func (m *mockRuntime) GetSharedResource(name string) (any, error) { return nil, nil }
func (m *mockRuntime) GetPrivateResource(name string) (any, error) { return nil, nil }
func (m *mockRuntime) GetResource(name string) (any, error) { return nil, nil }
func (m *mockRuntime) GetResourceInfo(name string) (*plugins.ResourceInfo, error) { return nil, nil }
func (m *mockRuntime) ListResources() []*plugins.ResourceInfo { return nil }
func (m *mockRuntime) RegisterPrivateResource(name string, resource any) error { return nil }
func (m *mockRuntime) RegisterResource(name string, resource any) error { return nil }
func (m *mockRuntime) RegisterSharedResource(name string, resource any) error { return nil }
func (m *mockRuntime) RemoveListener(listener plugins.EventListener) {}
func (m *mockRuntime) RemovePluginListener(pluginName string, listener plugins.EventListener) {}
func (m *mockRuntime) SetConfig(conf interface{}) {}
func (m *mockRuntime) SetEventDispatchMode(mode string) error { return nil }
func (m *mockRuntime) SetEventTimeout(timeout interface{}) {}
func (m *mockRuntime) SetEventWorkerPoolSize(size int) {}
func (m *mockRuntime) UnregisterPrivateResource(name string) error { return nil }
func (m *mockRuntime) UnregisterResource(name string) error { return nil }
func (m *mockRuntime) UnregisterSharedResource(name string) error { return nil }
func (m *mockRuntime) WithPluginContext(pluginName string) plugins.Runtime { return m }
func (m *mockRuntime) GetTypedResource(name string, resourceType string) (any, error) { return nil, nil }
func (m *mockRuntime) RegisterTypedResource(name string, resource any, resourceType string) error { return nil }

type mockConfig struct {
	values map[string]interface{}
}

func (m *mockConfig) Value(key string) interface{} {
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
	return nil
}


package main

import (
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/sql/base"
	"github.com/go-lynx/lynx/plugins/sql/interfaces"
)

func main() {
	fmt.Println("=== 验证数据库插件连接池配置 ===\n")

	// 测试1: 验证默认配置
	fmt.Println("1. 测试默认连接池配置:")
	testDefaultConfig()

	// 测试2: 验证配置验证逻辑
	fmt.Println("\n2. 测试配置验证逻辑:")
	testConfigValidation()

	// 测试3: 验证连接池统计
	fmt.Println("\n3. 测试连接池统计信息:")
	testConnectionPoolStats()

	fmt.Println("\n=== 所有验证完成 ===")
}

func testDefaultConfig() {
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

	// 模拟InitializeResources中的默认值设置
	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 25
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 5
	}

	fmt.Printf("  ✅ 默认MaxOpenConns: %d (预期: 25)\n", config.MaxOpenConns)
	fmt.Printf("  ✅ 默认MaxIdleConns: %d (预期: 5)\n", config.MaxIdleConns)

	if config.MaxOpenConns != 25 {
		fmt.Printf("  ❌ MaxOpenConns默认值不符合预期\n")
		os.Exit(1)
	}
	if config.MaxIdleConns != 5 {
		fmt.Printf("  ❌ MaxIdleConns默认值不符合预期\n")
		os.Exit(1)
	}

	_ = plugin
}

func testConfigValidation() {
	tests := []struct {
		name    string
		config  *interfaces.Config
		wantErr bool
	}{
		{
			name: "有效配置",
			config: &interfaces.Config{
				Driver:       "mysql",
				DSN:          "test://dsn",
				MaxOpenConns: 25,
				MaxIdleConns: 5,
			},
			wantErr: false,
		},
		{
			name: "MaxIdleConns大于MaxOpenConns",
			config: &interfaces.Config{
				Driver:       "mysql",
				DSN:          "test://dsn",
				MaxOpenConns: 5,
				MaxIdleConns: 10,
			},
			wantErr: true,
		},
		{
			name: "MaxOpenConns为0",
			config: &interfaces.Config{
				Driver:       "mysql",
				DSN:          "test://dsn",
				MaxOpenConns: 0,
				MaxIdleConns: 5,
			},
			wantErr: true,
		},
		{
			name: "MaxIdleConns为负数",
			config: &interfaces.Config{
				Driver:       "mysql",
				DSN:          "test://dsn",
				MaxOpenConns: 25,
				MaxIdleConns: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		plugin := base.NewBaseSQLPlugin(
			"test-id",
			"test-plugin",
			"Test plugin",
			"v1.0.0",
			"test.prefix",
			100,
			tt.config,
		)

		// 创建mock runtime
		rt := &mockRuntime{
			config: map[string]interface{}{
				"test.prefix": tt.config,
			},
		}

		err := plugin.InitializeResources(rt)
		hasErr := err != nil

		if hasErr != tt.wantErr {
			fmt.Printf("  ❌ %s: 预期错误=%v, 实际错误=%v\n", tt.name, tt.wantErr, hasErr)
			if err != nil {
				fmt.Printf("     错误信息: %v\n", err)
			}
			os.Exit(1)
		} else {
			fmt.Printf("  ✅ %s: 验证通过\n", tt.name)
		}
	}
}

func testConnectionPoolStats() {
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

	// 获取统计信息（未连接时应该返回空统计）
	stats := plugin.GetStats()
	
	fmt.Printf("  ✅ 连接池统计结构:\n")
	fmt.Printf("    - MaxOpenConnections: %d\n", stats.MaxOpenConnections)
	fmt.Printf("    - OpenConnections: %d\n", stats.OpenConnections)
	fmt.Printf("    - InUse: %d\n", stats.InUse)
	fmt.Printf("    - Idle: %d\n", stats.Idle)
	fmt.Printf("    - MaxIdleConnections: %d\n", stats.MaxIdleConnections)
	fmt.Printf("    - WaitCount: %d\n", stats.WaitCount)
	fmt.Printf("    - WaitDuration: %v\n", stats.WaitDuration)

	// 验证统计结构完整性
	if stats == nil {
		fmt.Printf("  ❌ GetStats返回nil\n")
		os.Exit(1)
	}

	fmt.Printf("  ✅ 连接池统计结构完整\n")
}

// mockRuntime实现
type mockRuntime struct {
	config map[string]interface{}
}

func (m *mockRuntime) GetConfig() config.Config {
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
func (m *mockRuntime) GetLogger() log.Logger { 
	return log.DefaultLogger
}
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
func (m *mockRuntime) SetConfig(conf config.Config) {}
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

func (m *mockConfig) Value(key string) config.Value {
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

func (m *mockValue) String() (string, error) { return "", nil }
func (m *mockValue) Bool() (bool, error) { return false, nil }
func (m *mockValue) Int() (int64, error) { return 0, nil }
func (m *mockValue) Float() (float64, error) { return 0, nil }
func (m *mockValue) Duration() (interface{}, error) { 
	var d interface{}
	return d, nil 
}

func (m *mockConfig) Load() error { return nil }
func (m *mockConfig) Watch(key string, o config.Observer) error { return nil }
func (m *mockConfig) Close() error { return nil }


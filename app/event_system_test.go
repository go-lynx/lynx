package app

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins"
)

// TestEventSystemIntegration 测试事件系统集成
func TestEventSystemIntegration(t *testing.T) {
	runtime := NewTypedRuntimePlugin()

	// 测试基本事件发送
	t.Run("BasicEventEmission", func(t *testing.T) {
		// 创建测试事件
		testEvent := plugins.PluginEvent{
			Type:      "test.event",
			Priority:  plugins.PriorityNormal,
			Source:    "test",
			Category:  "test",
			PluginID:  "test-plugin",
			Status:    plugins.StatusActive,
			Timestamp: time.Now().Unix(),
			Metadata: map[string]any{
				"test": "data",
			},
		}

		// 发送事件（不应该出错）
		runtime.EmitEvent(testEvent)

		// 等待事件处理
		time.Sleep(50 * time.Millisecond)

		t.Log("Event emitted successfully")
	})

	// 测试插件事件发送
	t.Run("PluginEventEmission", func(t *testing.T) {
		pluginRuntime := runtime.WithPluginContext("test-plugin")

		// 发送插件事件
		pluginRuntime.EmitPluginEvent("test-plugin", "test.plugin.event", map[string]any{
			"plugin": "test-data",
		})

		// 等待事件处理
		time.Sleep(50 * time.Millisecond)

		t.Log("Plugin event emitted successfully")
	})

	// 测试事件系统配置
	t.Run("EventSystemConfiguration", func(t *testing.T) {
		// 设置事件分发模式
		runtime.SetEventDispatchMode("async")

		// 设置工作池大小
		runtime.SetEventWorkerPoolSize(5)

		// 设置事件超时
		runtime.SetEventTimeout(30 * time.Second)

		// 获取事件统计
		stats := runtime.GetEventStats()
		if stats == nil {
			t.Error("Event stats should not be nil")
		} else {
			t.Logf("Event stats: %+v", stats)
		}
	})

	// 测试事件历史
	t.Run("EventHistory", func(t *testing.T) {
		// 发送一些测试事件
		for i := 0; i < 3; i++ {
			testEvent := plugins.PluginEvent{
				Type:      plugins.EventType("test.history." + string(rune('0'+i))),
				Priority:  plugins.PriorityNormal,
				Source:    "test",
				Category:  "history",
				PluginID:  "history-test",
				Status:    plugins.StatusActive,
				Timestamp: time.Now().Unix(),
				Metadata: map[string]any{
					"index": i,
				},
			}
			runtime.EmitEvent(testEvent)
		}

		// 等待事件处理
		time.Sleep(100 * time.Millisecond)

		// 获取事件历史
		filter := plugins.EventFilter{
			PluginIDs:  []string{"history-test"},
			Categories: []string{"history"},
		}

		history := runtime.GetEventHistory(filter)
		t.Logf("Found %d events in history", len(history))

		// 获取插件特定的事件历史
		pluginHistory := runtime.GetPluginEventHistory("history-test", filter)
		t.Logf("Found %d plugin events in history", len(pluginHistory))
	})
}

// TestEventSystemPerformance 测试事件系统性能
func TestEventSystemPerformance(t *testing.T) {
	runtime := NewTypedRuntimePlugin()

	// 设置异步模式以提高性能
	runtime.SetEventDispatchMode("async")
	runtime.SetEventWorkerPoolSize(10)

	// 发送大量事件
	numEvents := 1000
	start := time.Now()

	for i := 0; i < numEvents; i++ {
		testEvent := plugins.PluginEvent{
			Type:      "performance.test",
			Priority:  plugins.PriorityLow,
			Source:    "performance-test",
			Category:  "benchmark",
			PluginID:  "perf-test",
			Status:    plugins.StatusActive,
			Timestamp: time.Now().Unix(),
			Metadata: map[string]any{
				"index": i,
			},
		}
		runtime.EmitEvent(testEvent)
	}

	duration := time.Since(start)
	t.Logf("Emitted %d events in %v (%.2f events/sec)",
		numEvents, duration, float64(numEvents)/duration.Seconds())

	// 等待所有事件处理完成
	time.Sleep(500 * time.Millisecond)

	// 获取最终统计
	stats := runtime.GetEventStats()
	if stats != nil {
		t.Logf("Final event stats: %+v", stats)
	}
}

// TestPluginContextFunctionality 测试插件上下文基本功能
func TestPluginContextFunctionalityFixed(t *testing.T) {
	runtime := NewTypedRuntimePlugin()

	// 测试基本的插件上下文创建
	t.Run("BasicPluginContext", func(t *testing.T) {
		pluginName := "test-plugin"
		contextRuntime := runtime.WithPluginContext(pluginName)

		if contextRuntime == nil {
			t.Fatal("WithPluginContext returned nil")
		}

		// 验证当前插件上下文
		currentContext := contextRuntime.GetCurrentPluginContext()
		if currentContext != pluginName {
			t.Errorf("Expected plugin context '%s', got '%s'", pluginName, currentContext)
		}
	})

	// 测试插件上下文隔离
	t.Run("PluginContextIsolation", func(t *testing.T) {
		plugin1 := runtime.WithPluginContext("plugin1")
		plugin2 := runtime.WithPluginContext("plugin2")

		// 验证不同插件的上下文是独立的
		context1 := plugin1.GetCurrentPluginContext()
		context2 := plugin2.GetCurrentPluginContext()

		if context1 == context2 {
			t.Error("Plugin contexts should be isolated")
		}

		if context1 != "plugin1" {
			t.Errorf("Plugin1 context: expected 'plugin1', got '%s'", context1)
		}

		if context2 != "plugin2" {
			t.Errorf("Plugin2 context: expected 'plugin2', got '%s'", context2)
		}
	})

	// 测试插件上下文的资源隔离
	t.Run("PluginContextResourceIsolation", func(t *testing.T) {
		plugin1 := runtime.WithPluginContext("plugin1")
		plugin2 := runtime.WithPluginContext("plugin2")

		// 在plugin1中注册私有资源
		err := plugin1.RegisterPrivateResource("test-resource", "plugin1-value")
		if err != nil {
			t.Fatalf("Failed to register private resource in plugin1: %v", err)
		}

		// 在plugin2中注册同名私有资源
		err = plugin2.RegisterPrivateResource("test-resource", "plugin2-value")
		if err != nil {
			t.Fatalf("Failed to register private resource in plugin2: %v", err)
		}

		// 验证资源隔离
		value1, err := plugin1.GetPrivateResource("test-resource")
		if err != nil {
			t.Fatalf("Failed to get private resource from plugin1: %v", err)
		}
		if value1 != "plugin1-value" {
			t.Errorf("Plugin1 resource: expected 'plugin1-value', got %v", value1)
		}

		value2, err := plugin2.GetPrivateResource("test-resource")
		if err != nil {
			t.Fatalf("Failed to get private resource from plugin2: %v", err)
		}
		if value2 != "plugin2-value" {
			t.Errorf("Plugin2 resource: expected 'plugin2-value', got %v", value2)
		}
	})
}

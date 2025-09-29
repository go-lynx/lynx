package app

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins"
)

// TestTypedRuntimePluginImplementsRuntime 验证TypedRuntimePlugin是否正确实现了plugins.Runtime接口
func TestTypedRuntimePluginImplementsRuntime(t *testing.T) {
	// 创建TypedRuntimePlugin实例
	runtime := NewTypedRuntimePlugin()
	
	// 验证是否实现了plugins.Runtime接口
	var _ plugins.Runtime = runtime
	
	t.Log("TypedRuntimePlugin correctly implements plugins.Runtime interface")
}

// TestTypedRuntimePluginBasicFunctionality 测试TypedRuntimePlugin的基本功能
func TestTypedRuntimePluginBasicFunctionality(t *testing.T) {
	runtime := NewTypedRuntimePlugin()
	
	// 测试资源管理
	t.Run("ResourceManagement", func(t *testing.T) {
		// 测试共享资源
		testResource := "test-shared-resource"
		err := runtime.RegisterSharedResource("test-shared", testResource)
		if err != nil {
			t.Fatalf("Failed to register shared resource: %v", err)
		}
		
		retrieved, err := runtime.GetSharedResource("test-shared")
		if err != nil {
			t.Fatalf("Failed to get shared resource: %v", err)
		}
		
		if retrieved != testResource {
			t.Errorf("Expected %v, got %v", testResource, retrieved)
		}
		
		// 测试私有资源（需要插件上下文）
		contextRuntime := runtime.WithPluginContext("test-plugin")
		err = contextRuntime.RegisterPrivateResource("test-private", "private-resource")
		if err != nil {
			t.Fatalf("Failed to register private resource: %v", err)
		}
		
		privateResource, err := contextRuntime.GetPrivateResource("test-private")
		if err != nil {
			t.Fatalf("Failed to get private resource: %v", err)
		}
		
		if privateResource != "private-resource" {
			t.Errorf("Expected 'private-resource', got %v", privateResource)
		}
	})
	
	// 测试插件上下文
	t.Run("PluginContext", func(t *testing.T) {
		contextRuntime := runtime.WithPluginContext("test-plugin")
		
		// 验证返回的是plugins.Runtime接口
		var _ plugins.Runtime = contextRuntime
		
		// 测试获取当前插件上下文
		context := contextRuntime.GetCurrentPluginContext()
		if context != "test-plugin" {
			t.Errorf("Expected 'test-plugin', got %s", context)
		}
	})
	
	// 测试事件系统
	t.Run("EventSystem", func(t *testing.T) {
		// 测试事件配置
		err := runtime.SetEventDispatchMode("async")
		if err != nil {
			t.Logf("SetEventDispatchMode returned error: %v", err)
		}
		
		runtime.SetEventWorkerPoolSize(5)
		runtime.SetEventTimeout(time.Second * 30)
		
		// 测试获取事件统计
		stats := runtime.GetEventStats()
		if stats == nil {
			t.Error("GetEventStats returned nil")
		}
		
		// 测试插件事件
		runtime.EmitPluginEvent("test-plugin", "test-event", map[string]any{
			"test": "data",
		})
	})
	
	// 测试配置管理
	t.Run("ConfigManagement", func(t *testing.T) {
		// 获取配置（可能为nil）
		conf := runtime.GetConfig()
		t.Logf("Current config: %v", conf)
		
		// 设置配置
		runtime.SetConfig(nil) // 测试设置nil配置
	})
	
	// 测试资源信息
	t.Run("ResourceInfo", func(t *testing.T) {
		// 注册一个资源
		err := runtime.RegisterSharedResource("info-test", "test-value")
		if err != nil {
			t.Fatalf("Failed to register resource: %v", err)
		}
		
		// 获取资源信息
		info, err := runtime.GetResourceInfo("info-test")
		if err != nil {
			t.Fatalf("Failed to get resource info: %v", err)
		}
		
		if info == nil {
			t.Error("Resource info is nil")
		} else {
			t.Logf("Resource info: %+v", info)
		}
		
		// 列出所有资源
		resources := runtime.ListResources()
		if len(resources) == 0 {
			t.Error("No resources found")
		}
		
		// 获取资源统计
		stats := runtime.GetResourceStats()
		if stats == nil {
			t.Error("Resource stats is nil")
		}
	})
}

// TestTypedRuntimePluginEventHandling 测试事件处理功能
func TestTypedRuntimePluginEventHandling(t *testing.T) {
	runtime := NewTypedRuntimePlugin()
	
	// 创建一个简单的事件监听器
	listener := &testEventListener{
		id:     "test-listener",
		events: make([]plugins.PluginEvent, 0),
	}
	
	// 添加监听器
	filter := &plugins.EventFilter{
		Types: []plugins.EventType{"test-event"},
	}
	runtime.AddListener(listener, filter)
	
	// 发送事件
	event := plugins.PluginEvent{
		Type:     "test-event",
		PluginID: "test-plugin",
		Metadata: map[string]any{"test": "data"},
	}
	runtime.EmitEvent(event)
	
	// 等待事件处理
	time.Sleep(100 * time.Millisecond)
	
	// 验证事件历史
	history := runtime.GetEventHistory(plugins.EventFilter{
		Types: []plugins.EventType{"test-event"},
	})
	
	t.Logf("Event history length: %d", len(history))
	
	// 移除监听器
	runtime.RemoveListener(listener)
}

// testEventListener 测试用的事件监听器
type testEventListener struct {
	id     string
	events []plugins.PluginEvent
}

func (l *testEventListener) HandleEvent(event plugins.PluginEvent) {
	l.events = append(l.events, event)
}

func (l *testEventListener) GetListenerID() string {
	return l.id
}
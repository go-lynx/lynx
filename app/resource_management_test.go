package app

import (
	"sync"
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins"
)

// TestResourceManagementConcurrency 测试资源管理的并发安全性
func TestResourceManagementConcurrency(t *testing.T) {
	runtime := NewTypedRuntimePlugin()
	
	// 并发注册共享资源
	t.Run("ConcurrentSharedResourceRegistration", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 10
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				resourceName := "shared-resource-" + string(rune('0'+id))
				resourceValue := "value-" + string(rune('0'+id))
				
				err := runtime.RegisterSharedResource(resourceName, resourceValue)
				if err != nil {
					t.Errorf("Failed to register shared resource %s: %v", resourceName, err)
					return
				}
				
				// 立即尝试获取资源
				retrieved, err := runtime.GetSharedResource(resourceName)
				if err != nil {
					t.Errorf("Failed to get shared resource %s: %v", resourceName, err)
					return
				}
				
				if retrieved != resourceValue {
					t.Errorf("Resource value mismatch for %s: expected %s, got %v", resourceName, resourceValue, retrieved)
				}
			}(i)
		}
		
		wg.Wait()
		
		// 验证所有资源都已注册
		resources := runtime.ListResources()
		if len(resources) < numGoroutines {
			t.Errorf("Expected at least %d resources, got %d", numGoroutines, len(resources))
		}
	})
	
	// 并发注册私有资源
	t.Run("ConcurrentPrivateResourceRegistration", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 5
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				pluginName := "plugin-" + string(rune('0'+id))
				contextRuntime := runtime.WithPluginContext(pluginName)
				
				resourceName := "private-resource-" + string(rune('0'+id))
				resourceValue := "private-value-" + string(rune('0'+id))
				
				err := contextRuntime.RegisterPrivateResource(resourceName, resourceValue)
				if err != nil {
					t.Errorf("Failed to register private resource %s: %v", resourceName, err)
					return
				}
				
				// 立即尝试获取资源
				retrieved, err := contextRuntime.GetPrivateResource(resourceName)
				if err != nil {
					t.Errorf("Failed to get private resource %s: %v", resourceName, err)
					return
				}
				
				if retrieved != resourceValue {
					t.Errorf("Private resource value mismatch for %s: expected %s, got %v", resourceName, resourceValue, retrieved)
				}
			}(i)
		}
		
		wg.Wait()
	})
}

// TestResourceIsolation 测试资源隔离
func TestResourceIsolation(t *testing.T) {
	runtime := NewTypedRuntimePlugin()
	
	// 创建两个不同的插件上下文
	plugin1Runtime := runtime.WithPluginContext("plugin1")
	plugin2Runtime := runtime.WithPluginContext("plugin2")
	
	// 在plugin1中注册私有资源
	err := plugin1Runtime.RegisterPrivateResource("private-resource", "plugin1-value")
	if err != nil {
		t.Fatalf("Failed to register private resource in plugin1: %v", err)
	}
	
	// 在plugin2中注册同名私有资源
	err = plugin2Runtime.RegisterPrivateResource("private-resource", "plugin2-value")
	if err != nil {
		t.Fatalf("Failed to register private resource in plugin2: %v", err)
	}
	
	// 验证plugin1只能访问自己的私有资源
	value1, err := plugin1Runtime.GetPrivateResource("private-resource")
	if err != nil {
		t.Fatalf("Failed to get private resource from plugin1: %v", err)
	}
	if value1 != "plugin1-value" {
		t.Errorf("Plugin1 private resource: expected 'plugin1-value', got %v", value1)
	}
	
	// 验证plugin2只能访问自己的私有资源
	value2, err := plugin2Runtime.GetPrivateResource("private-resource")
	if err != nil {
		t.Fatalf("Failed to get private resource from plugin2: %v", err)
	}
	if value2 != "plugin2-value" {
		t.Errorf("Plugin2 private resource: expected 'plugin2-value', got %v", value2)
	}
	
	// 注册共享资源
	err = runtime.RegisterSharedResource("shared-resource", "shared-value")
	if err != nil {
		t.Fatalf("Failed to register shared resource: %v", err)
	}
	
	// 验证两个插件都能访问共享资源
	sharedValue1, err := plugin1Runtime.GetSharedResource("shared-resource")
	if err != nil {
		t.Fatalf("Plugin1 failed to get shared resource: %v", err)
	}
	if sharedValue1 != "shared-value" {
		t.Errorf("Plugin1 shared resource: expected 'shared-value', got %v", sharedValue1)
	}
	
	sharedValue2, err := plugin2Runtime.GetSharedResource("shared-resource")
	if err != nil {
		t.Fatalf("Plugin2 failed to get shared resource: %v", err)
	}
	if sharedValue2 != "shared-value" {
		t.Errorf("Plugin2 shared resource: expected 'shared-value', got %v", sharedValue2)
	}
}

// TestResourceLifecycle 测试资源生命周期管理
func TestResourceLifecycle(t *testing.T) {
	runtime := NewTypedRuntimePlugin()
	
	// 注册资源
	testResource := &TestResource{
		Name:      "test-resource",
		CreatedAt: time.Now(),
	}
	
	err := runtime.RegisterSharedResource("lifecycle-test", testResource)
	if err != nil {
		t.Fatalf("Failed to register resource: %v", err)
	}
	
	// 获取资源信息
	info, err := runtime.GetResourceInfo("lifecycle-test")
	if err != nil {
		t.Fatalf("Failed to get resource info: %v", err)
	}
	
	if info == nil {
		t.Fatal("Resource info is nil")
	}
	
	t.Logf("Resource info: Name=%s, Type=%s, Size=%d, CreatedAt=%v", 
		info.Name, info.Type, info.Size, info.CreatedAt)
	
	// 验证资源信息
	if info.Name != "lifecycle-test" {
		t.Errorf("Expected name 'lifecycle-test', got %s", info.Name)
	}
	
	if info.Type != "*app.TestResource" {
		t.Errorf("Expected type '*app.TestResource', got %s", info.Type)
	}
	
	// 获取资源统计
	stats := runtime.GetResourceStats()
	if stats == nil {
		t.Error("Resource stats is nil")
	} else {
		t.Logf("Resource stats: %+v", stats)
	}
	
	// 清理资源
	err = runtime.CleanupResources("test-plugin")
	if err != nil {
		t.Errorf("Failed to cleanup resources: %v", err)
	}
}

// TestTypedResourceAccess 测试类型安全的资源访问
func TestTypedResourceAccess(t *testing.T) {
	runtime := NewTypedRuntimePlugin()
	
	// 注册一个具体类型的资源
	testResource := &TestResource{
		Name:      "typed-resource",
		CreatedAt: time.Now(),
	}
	
	err := runtime.RegisterSharedResource("typed-test", testResource)
	if err != nil {
		t.Fatalf("Failed to register typed resource: %v", err)
	}
	
	// 使用类型安全的方式获取资源
	retrieved, err := plugins.GetTypedResource[*TestResource](runtime, "typed-test")
	if err != nil {
		t.Fatalf("Failed to get typed resource: %v", err)
	}
	
	if retrieved.Name != "typed-resource" {
		t.Errorf("Expected name 'typed-resource', got %s", retrieved.Name)
	}
	
	// 尝试获取错误类型的资源
	_, err = plugins.GetTypedResource[string](runtime, "typed-test")
	if err == nil {
		t.Error("Expected error when getting resource with wrong type, but got nil")
	}
}

// TestResource 测试用的资源类型
type TestResource struct {
	Name      string
	CreatedAt time.Time
	Data      map[string]any
}

// Close 实现资源清理接口
func (r *TestResource) Close() error {
	r.Data = nil
	return nil
}
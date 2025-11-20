package app

import (
	"sync"
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins"
)

// TestResourceManagementConcurrency tests the concurrency safety of resource management
func TestResourceManagementConcurrency(t *testing.T) {
	runtime := NewTypedRuntimePlugin()

	// Concurrently register shared resources
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

				// Immediately try to get the resource
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

		// Verify all resources are registered
		resources := runtime.ListResources()
		if len(resources) < numGoroutines {
			t.Errorf("Expected at least %d resources, got %d", numGoroutines, len(resources))
		}
	})

	// Concurrently register private resources
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

				// Immediately try to get the resource
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

// TestResourceIsolation tests resource isolation
func TestResourceIsolation(t *testing.T) {
	runtime := NewTypedRuntimePlugin()

	// Create two different plugin contexts
	plugin1Runtime := runtime.WithPluginContext("plugin1")
	plugin2Runtime := runtime.WithPluginContext("plugin2")

	// Register private resource in plugin1
	err := plugin1Runtime.RegisterPrivateResource("private-resource", "plugin1-value")
	if err != nil {
		t.Fatalf("Failed to register private resource in plugin1: %v", err)
	}

	// Register private resource with the same name in plugin2
	err = plugin2Runtime.RegisterPrivateResource("private-resource", "plugin2-value")
	if err != nil {
		t.Fatalf("Failed to register private resource in plugin2: %v", err)
	}

	// Verify plugin1 can only access its own private resource
	value1, err := plugin1Runtime.GetPrivateResource("private-resource")
	if err != nil {
		t.Fatalf("Failed to get private resource from plugin1: %v", err)
	}
	if value1 != "plugin1-value" {
		t.Errorf("Plugin1 private resource: expected 'plugin1-value', got %v", value1)
	}

	// Verify plugin2 can only access its own private resource
	value2, err := plugin2Runtime.GetPrivateResource("private-resource")
	if err != nil {
		t.Fatalf("Failed to get private resource from plugin2: %v", err)
	}
	if value2 != "plugin2-value" {
		t.Errorf("Plugin2 private resource: expected 'plugin2-value', got %v", value2)
	}

	// Register shared resource
	err = runtime.RegisterSharedResource("shared-resource", "shared-value")
	if err != nil {
		t.Fatalf("Failed to register shared resource: %v", err)
	}

	// Verify both plugins can access the shared resource
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

// TestResourceLifecycle tests resource lifecycle management
func TestResourceLifecycle(t *testing.T) {
	runtime := NewTypedRuntimePlugin()

	// Register resource
	testResource := &TestResource{
		Name:      "test-resource",
		CreatedAt: time.Now(),
	}

	err := runtime.RegisterSharedResource("lifecycle-test", testResource)
	if err != nil {
		t.Fatalf("Failed to register resource: %v", err)
	}

	// Get resource information
	info, err := runtime.GetResourceInfo("lifecycle-test")
	if err != nil {
		t.Fatalf("Failed to get resource info: %v", err)
	}

	if info == nil {
		t.Fatal("Resource info is nil")
	}

	t.Logf("Resource info: Name=%s, Type=%s, Size=%d, CreatedAt=%v",
		info.Name, info.Type, info.Size, info.CreatedAt)

	// Verify resource information
	if info.Name != "lifecycle-test" {
		t.Errorf("Expected name 'lifecycle-test', got %s", info.Name)
	}

	if info.Type != "*app.TestResource" {
		t.Errorf("Expected type '*app.TestResource', got %s", info.Type)
	}

	// Get resource statistics
	stats := runtime.GetResourceStats()
	if stats == nil {
		t.Error("Resource stats is nil")
	} else {
		t.Logf("Resource stats: %+v", stats)
	}

	// Clean up resources
	err = runtime.CleanupResources("test-plugin")
	if err != nil {
		t.Errorf("Failed to cleanup resources: %v", err)
	}
}

// TestTypedResourceAccess tests type-safe resource access
func TestTypedResourceAccess(t *testing.T) {
	runtime := NewTypedRuntimePlugin()

	// Register a resource of a specific type
	testResource := &TestResource{
		Name:      "typed-resource",
		CreatedAt: time.Now(),
	}

	err := runtime.RegisterSharedResource("typed-test", testResource)
	if err != nil {
		t.Fatalf("Failed to register typed resource: %v", err)
	}

	// Get resource using type-safe method
	retrieved, err := plugins.GetTypedResource[*TestResource](runtime, "typed-test")
	if err != nil {
		t.Fatalf("Failed to get typed resource: %v", err)
	}

	if retrieved.Name != "typed-resource" {
		t.Errorf("Expected name 'typed-resource', got %s", retrieved.Name)
	}

	// Try to get resource with wrong type
	_, err = plugins.GetTypedResource[string](runtime, "typed-test")
	if err == nil {
		t.Error("Expected error when getting resource with wrong type, but got nil")
	}
}

// TestResource is a test resource type
type TestResource struct {
	Name      string
	CreatedAt time.Time
	Data      map[string]any
}

// Close implements the resource cleanup interface
func (r *TestResource) Close() error {
	r.Data = nil
	return nil
}

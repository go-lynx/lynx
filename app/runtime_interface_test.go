package app

import (
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins"
)

// TestTypedRuntimePluginImplementsRuntime verifies that TypedRuntimePlugin correctly implements the plugins.Runtime interface
func TestTypedRuntimePluginImplementsRuntime(t *testing.T) {
	// Create TypedRuntimePlugin instance
	runtime := NewTypedRuntimePlugin()
	
	// Verify it implements plugins.Runtime interface
	var _ plugins.Runtime = runtime
	
	t.Log("TypedRuntimePlugin correctly implements plugins.Runtime interface")
}

// TestTypedRuntimePluginBasicFunctionality tests the basic functionality of TypedRuntimePlugin
func TestTypedRuntimePluginBasicFunctionality(t *testing.T) {
	runtime := NewTypedRuntimePlugin()
	
	// Test resource management
	t.Run("ResourceManagement", func(t *testing.T) {
		// Test shared resources
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
		
		// Test private resources (requires plugin context)
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
	
	// Test plugin context
	t.Run("PluginContext", func(t *testing.T) {
		contextRuntime := runtime.WithPluginContext("test-plugin")
		
		// Verify it returns plugins.Runtime interface
		var _ plugins.Runtime = contextRuntime
		
		// Test getting current plugin context
		context := contextRuntime.GetCurrentPluginContext()
		if context != "test-plugin" {
			t.Errorf("Expected 'test-plugin', got %s", context)
		}
	})
	
	// Test event system
	t.Run("EventSystem", func(t *testing.T) {
		// Test event configuration
		err := runtime.SetEventDispatchMode("async")
		if err != nil {
			t.Logf("SetEventDispatchMode returned error: %v", err)
		}
		
		runtime.SetEventWorkerPoolSize(5)
		runtime.SetEventTimeout(time.Second * 30)
		
		// Test getting event statistics
		stats := runtime.GetEventStats()
		if stats == nil {
			t.Error("GetEventStats returned nil")
		}
		
		// Test plugin events
		runtime.EmitPluginEvent("test-plugin", "test-event", map[string]any{
			"test": "data",
		})
	})
	
	// Test configuration management
	t.Run("ConfigManagement", func(t *testing.T) {
		// Get configuration (may be nil)
		conf := runtime.GetConfig()
		t.Logf("Current config: %v", conf)
		
		// Set configuration
		runtime.SetConfig(nil) // Test setting nil configuration
	})
	
	// Test resource information
	t.Run("ResourceInfo", func(t *testing.T) {
		// Register a resource
		err := runtime.RegisterSharedResource("info-test", "test-value")
		if err != nil {
			t.Fatalf("Failed to register resource: %v", err)
		}
		
		// Get resource information
		info, err := runtime.GetResourceInfo("info-test")
		if err != nil {
			t.Fatalf("Failed to get resource info: %v", err)
		}
		
		if info == nil {
			t.Error("Resource info is nil")
		} else {
			t.Logf("Resource info: %+v", info)
		}
		
		// List all resources
		resources := runtime.ListResources()
		if len(resources) == 0 {
			t.Error("No resources found")
		}
		
		// Get resource statistics
		stats := runtime.GetResourceStats()
		if stats == nil {
			t.Error("Resource stats is nil")
		}
	})
}

// TestTypedRuntimePluginEventHandling tests event handling functionality
func TestTypedRuntimePluginEventHandling(t *testing.T) {
	runtime := NewTypedRuntimePlugin()
	
	// Create a simple event listener
	listener := &testEventListener{
		id:     "test-listener",
		events: make([]plugins.PluginEvent, 0),
	}
	
	// Add listener
	filter := &plugins.EventFilter{
		Types: []plugins.EventType{"test-event"},
	}
	runtime.AddListener(listener, filter)
	
	// Emit event
	event := plugins.PluginEvent{
		Type:     "test-event",
		PluginID: "test-plugin",
		Metadata: map[string]any{"test": "data"},
	}
	runtime.EmitEvent(event)
	
	// Wait for event processing
	time.Sleep(100 * time.Millisecond)
	
	// Verify event history
	history := runtime.GetEventHistory(plugins.EventFilter{
		Types: []plugins.EventType{"test-event"},
	})
	
	t.Logf("Event history length: %d", len(history))
	
	// Remove listener
	runtime.RemoveListener(listener)
}

// testEventListener is a test event listener
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
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
)

// ExamplePlugin example plugin
type ExamplePlugin struct {
	*plugins.TypedBasePlugin[*ExamplePlugin]
	name string
}

func NewExamplePlugin(name string) *ExamplePlugin {
	plugin := &ExamplePlugin{
		name: name,
	}
	plugin.TypedBasePlugin = plugins.NewTypedBasePlugin(name, "Example Plugin", "A simple example plugin", "1.0.0", "example", 1, plugin)
	return plugin
}

func (p *ExamplePlugin) Initialize(plugin plugins.Plugin, rt plugins.Runtime) error {
	fmt.Printf("Plugin %s initializing...\n", p.name)

	// Register private resources
	if err := rt.RegisterPrivateResource("private_config", map[string]string{
		"plugin_name": p.name,
		"created_at":  time.Now().Format(time.RFC3339),
	}); err != nil {
		return err
	}

	// Register shared resources
	if err := rt.RegisterSharedResource("shared_counter", 0); err != nil {
		return err
	}

	// Emit initialization event
	rt.EmitPluginEvent(p.name, "initialized", map[string]any{
		"timestamp": time.Now().Unix(),
		"status":    "ready",
	})

	return nil
}

func (p *ExamplePlugin) Start(plugin plugins.Plugin) error {
	fmt.Printf("Plugin %s starting...\n", p.name)

	// Emit start event
	p.EmitEvent(plugins.PluginEvent{
		Type:     "started",
		PluginID: p.name,
		Source:   p.name,
		Metadata: map[string]any{
			"timestamp": time.Now().Unix(),
			"status":    "running",
		},
		Timestamp: time.Now().Unix(),
	})

	return nil
}

func (p *ExamplePlugin) Stop(plugin plugins.Plugin) error {
	fmt.Printf("Plugin %s stopping...\n", p.name)

	// Emit stop event
	p.EmitEvent(plugins.PluginEvent{
		Type:     "stopped",
		PluginID: p.name,
		Source:   p.name,
		Metadata: map[string]any{
			"timestamp": time.Now().Unix(),
			"status":    "stopped",
		},
		Timestamp: time.Now().Unix(),
	})

	return nil
}

func (p *ExamplePlugin) Status(plugin plugins.Plugin) plugins.PluginStatus {
	return plugins.StatusActive
}

func (p *ExamplePlugin) GetDependencies() []plugins.Dependency {
	return []plugins.Dependency{}
}

// EventListener event listener
type EventListener struct {
	name string
}

func NewEventListener(name string) *EventListener {
	return &EventListener{name: name}
}

func (l *EventListener) HandleEvent(event plugins.PluginEvent) {
	fmt.Printf("[%s] Event: %s from plugin %s at %d\n",
		l.name, event.Type, event.PluginID, event.Timestamp)
}

func (l *EventListener) GetListenerID() string {
	return l.name
}

func main() {
	fmt.Println("=== Layered Runtime Design Example ===")

	// Create plugin manager (Typed version)
	manager := app.NewTypedPluginManager()

	// Create example plugins (simplified version)
	fmt.Println("Creating plugins...")

	// Get runtime
	runtime := manager.GetRuntime()

	// Add event listeners
	listener1 := NewEventListener("global_listener")
	runtime.AddListener(listener1, nil)

	listener2 := NewEventListener("plugin1_listener")
	runtime.AddPluginListener("plugin1", listener2, nil)

	// Create configuration
	conf := config.New()

	// Load plugins
	fmt.Println("\n--- Loading Plugins ---")
	if err := manager.LoadPlugins(conf); err != nil {
		log.Fatalf("Failed to load plugins: %v", err)
	}

	// Wait for event processing
	time.Sleep(1 * time.Second)

	// Display resource statistics
	fmt.Println("\n--- Resource Statistics ---")
	stats := manager.GetResourceStats()
	for key, value := range stats {
		fmt.Printf("%s: %v\n", key, value)
	}

	// List all resources
	fmt.Println("\n--- Resource List ---")
	resources := manager.ListResources()
	for _, resource := range resources {
		fmt.Printf("Resource: %s (Type: %s, Plugin: %s, Private: %v, Size: %d bytes)\n",
			resource.Name, resource.Type, resource.PluginID, resource.IsPrivate, resource.Size)
	}

	// Display event history
	fmt.Println("\n--- Event History ---")
	events := runtime.GetEventHistory(plugins.EventFilter{})
	for _, event := range events {
		fmt.Printf("Event: %s from %s at %d\n", event.Type, event.PluginID, event.Timestamp)
	}

	// Display specific plugin event history
	fmt.Println("\n--- Plugin1 Event History ---")
	plugin1Events := runtime.GetPluginEventHistory("plugin1", plugins.EventFilter{})
	for _, event := range plugin1Events {
		fmt.Printf("Plugin1 Event: %s at %d\n", event.Type, event.Timestamp)
	}

	// Stop plugins
	fmt.Println("\n--- Stopping Plugins ---")
	if err := manager.StopPlugin("plugin1"); err != nil {
		log.Printf("Failed to stop plugin1: %v", err)
	}

	// Display resource statistics after stopping
	fmt.Println("\n--- Resource Statistics After Stopping ---")
	stats = manager.GetResourceStats()
	for key, value := range stats {
		fmt.Printf("%s: %v\n", key, value)
	}

	fmt.Println("\n=== Example Completed ===")
}

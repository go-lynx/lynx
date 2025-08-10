package main

import (
	"fmt"
	"log"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
)

// ExamplePlugin 示例插件
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
	
	// 注册私有资源
	if err := rt.RegisterPrivateResource("private_config", map[string]string{
		"plugin_name": p.name,
		"created_at":  time.Now().Format(time.RFC3339),
	}); err != nil {
		return err
	}
	
	// 注册共享资源
	if err := rt.RegisterSharedResource("shared_counter", 0); err != nil {
		return err
	}
	
	// 发出初始化事件
	rt.EmitPluginEvent(p.name, "initialized", map[string]any{
		"timestamp": time.Now().Unix(),
		"status":    "ready",
	})
	
	return nil
}

func (p *ExamplePlugin) Start(plugin plugins.Plugin) error {
	fmt.Printf("Plugin %s starting...\n", p.name)
	
	// 发出启动事件
	p.EmitEvent(plugins.PluginEvent{
		Type:      "started",
		PluginID:  p.name,
		Source:    p.name,
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
	
	// 发出停止事件
	p.EmitEvent(plugins.PluginEvent{
		Type:      "stopped",
		PluginID:  p.name,
		Source:    p.name,
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

// EventListener 事件监听器
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
	fmt.Println("=== 分层 Runtime 设计示例 ===")
	
	// 创建插件管理器
	manager := app.NewPluginManager()
	
	// 创建示例插件（简化版本）
	fmt.Println("创建插件...")
	
	// 获取运行时
	runtime := manager.GetRuntime()
	
	// 添加事件监听器
	listener1 := NewEventListener("global_listener")
	runtime.AddListener(listener1, nil)
	
	listener2 := NewEventListener("plugin1_listener")
	runtime.AddPluginListener("plugin1", listener2, nil)
	
	// 创建配置
	conf := config.New()
	
	// 加载插件
	fmt.Println("\n--- 加载插件 ---")
	if err := manager.LoadPlugins(conf); err != nil {
		log.Fatalf("Failed to load plugins: %v", err)
	}
	
	// 等待一段时间让事件处理
	time.Sleep(1 * time.Second)
	
	// 显示资源统计
	fmt.Println("\n--- 资源统计 ---")
	stats := manager.GetResourceStats()
	for key, value := range stats {
		fmt.Printf("%s: %v\n", key, value)
	}
	
	// 列出所有资源
	fmt.Println("\n--- 资源列表 ---")
	resources := manager.ListResources()
	for _, resource := range resources {
		fmt.Printf("Resource: %s (Type: %s, Plugin: %s, Private: %v, Size: %d bytes)\n",
			resource.Name, resource.Type, resource.PluginID, resource.IsPrivate, resource.Size)
	}
	
	// 显示事件历史
	fmt.Println("\n--- 事件历史 ---")
	events := runtime.GetEventHistory(plugins.EventFilter{})
	for _, event := range events {
		fmt.Printf("Event: %s from %s at %d\n", event.Type, event.PluginID, event.Timestamp)
	}
	
	// 显示特定插件的事件历史
	fmt.Println("\n--- Plugin1 事件历史 ---")
	plugin1Events := runtime.GetPluginEventHistory("plugin1", plugins.EventFilter{})
	for _, event := range plugin1Events {
		fmt.Printf("Plugin1 Event: %s at %d\n", event.Type, event.Timestamp)
	}
	
	// 停止插件
	fmt.Println("\n--- 停止插件 ---")
	if err := manager.StopPlugin("plugin1"); err != nil {
		log.Printf("Failed to stop plugin1: %v", err)
	}
	
	// 显示停止后的资源统计
	fmt.Println("\n--- 停止后的资源统计 ---")
	stats = manager.GetResourceStats()
	for key, value := range stats {
		fmt.Printf("%s: %v\n", key, value)
	}
	
	fmt.Println("\n=== 示例完成 ===")
}

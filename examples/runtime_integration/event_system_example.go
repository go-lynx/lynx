package main

import (
	"fmt"
	"log"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
)

// 示例：展示改进的事件系统

func main() {
	// 创建插件管理器
	manager := app.NewPluginManager()

	// 模拟配置
	conf := createMockConfig()

	// 加载插件
	manager.LoadPlugins(conf)

	// 获取统一的 runtime
	runtime := manager.GetRuntime()

	// 演示事件系统
	demonstrateEventSystem(runtime)

	// 演示热更新场景
	demonstrateHotUpdateScenario(runtime)
}

func createMockConfig() config.Config {
	// 这里应该创建实际的配置
	return nil
}

func demonstrateEventSystem(runtime plugins.Runtime) {
	fmt.Println("=== 改进的事件系统演示 ===")

	// 1. 演示命名空间事件
	fmt.Println("\n1. 命名空间事件演示:")
	
	// HTTP 插件发出事件
	runtime.EmitPluginEvent("http-plugin", "server.started", map[string]any{
		"port":     8080,
		"protocol": "http",
		"status":   "running",
	})
	fmt.Println("✓ HTTP 插件发出了 server.started 事件")

	// 数据库插件发出事件
	runtime.EmitPluginEvent("db-plugin", "connection.established", map[string]any{
		"host":     "localhost",
		"port":     3306,
		"database": "production",
		"status":   "connected",
	})
	fmt.Println("✓ 数据库插件发出了 connection.established 事件")

	// 缓存插件发出事件
	runtime.EmitPluginEvent("cache-plugin", "cache.initialized", map[string]any{
		"type":     "redis",
		"host":     "localhost",
		"port":     6379,
		"status":   "ready",
	})
	fmt.Println("✓ 缓存插件发出了 cache.initialized 事件")

	// 2. 演示事件监听
	fmt.Println("\n2. 事件监听演示:")
	
	// 创建事件监听器
	monitorListener := &MonitorEventListener{
		name: "system-monitor",
	}
	
	// 添加全局监听器
	runtime.AddListener(monitorListener, nil)
	fmt.Println("✓ 添加了全局事件监听器")

	// 添加特定插件的监听器
	httpListener := &HTTPEventListener{
		name: "http-monitor",
	}
	runtime.AddPluginListener("http-plugin", httpListener, nil)
	fmt.Println("✓ 添加了 HTTP 插件的专用监听器")

	// 3. 演示事件历史
	fmt.Println("\n3. 事件历史演示:")
	
	// 获取所有事件历史
	allEvents := runtime.GetEventHistory(plugins.EventFilter{})
	fmt.Printf("✓ 获取到 %d 个历史事件\n", len(allEvents))

	// 获取特定插件的事件历史
	httpEvents := runtime.GetPluginEventHistory("http-plugin", plugins.EventFilter{})
	fmt.Printf("✓ 获取到 %d 个 HTTP 插件事件\n", len(httpEvents))
}

func demonstrateHotUpdateScenario(runtime plugins.Runtime) {
	fmt.Println("\n=== 热更新场景演示 ===")

	// 模拟插件热更新过程
	fmt.Println("\n1. 开始热更新过程:")
	
	// 发出升级开始事件
	runtime.EmitPluginEvent("http-plugin", "upgrade.started", map[string]any{
		"version":     "2.0.0",
		"old_version": "1.0.0",
		"timestamp":   "2024-01-01T10:00:00Z",
	})
	fmt.Println("✓ HTTP 插件开始升级")

	// 发出升级进行中事件
	runtime.EmitPluginEvent("http-plugin", "upgrade.in_progress", map[string]any{
		"progress": 50,
		"step":     "configuring",
	})
	fmt.Println("✓ HTTP 插件升级进行中")

	// 发出升级完成事件
	runtime.EmitPluginEvent("http-plugin", "upgrade.completed", map[string]any{
		"new_version": "2.0.0",
		"status":      "success",
		"restart":     false,
	})
	fmt.Println("✓ HTTP 插件升级完成")

	// 发出服务重启事件
	runtime.EmitPluginEvent("http-plugin", "server.restarted", map[string]any{
		"port":     8080,
		"protocol": "http",
		"status":   "running",
	})
	fmt.Println("✓ HTTP 服务重启完成")

	fmt.Println("\n2. 事件隔离演示:")
	
	// 数据库插件在 HTTP 插件升级期间继续正常工作
	runtime.EmitPluginEvent("db-plugin", "query.executed", map[string]any{
		"query":    "SELECT * FROM users",
		"duration": "10ms",
		"rows":     100,
	})
	fmt.Println("✓ 数据库插件继续正常工作，不受 HTTP 插件升级影响")

	// 缓存插件也继续工作
	runtime.EmitPluginEvent("cache-plugin", "cache.hit", map[string]any{
		"key":      "user:123",
		"duration": "1ms",
	})
	fmt.Println("✓ 缓存插件继续正常工作")
}

// MonitorEventListener 系统监控事件监听器
type MonitorEventListener struct {
	name string
}

func (m *MonitorEventListener) HandleEvent(event plugins.PluginEvent) {
	fmt.Printf("[%s] 收到事件: %s 来自 %s\n", m.name, event.Type, event.PluginID)
	
	// 根据事件类型进行不同的处理
	switch event.Type {
	case plugins.EventType("server.started"):
		fmt.Printf("  → 服务器启动: %v\n", event.Metadata)
	case plugins.EventType("connection.established"):
		fmt.Printf("  → 连接建立: %v\n", event.Metadata)
	case plugins.EventType("cache.initialized"):
		fmt.Printf("  → 缓存初始化: %v\n", event.Metadata)
	case plugins.EventType("upgrade.started"):
		fmt.Printf("  → 升级开始: %v\n", event.Metadata)
	case plugins.EventType("upgrade.completed"):
		fmt.Printf("  → 升级完成: %v\n", event.Metadata)
	}
}

func (m *MonitorEventListener) GetListenerID() string {
	return m.name
}

// HTTPEventListener HTTP 插件专用事件监听器
type HTTPEventListener struct {
	name string
}

func (h *HTTPEventListener) HandleEvent(event plugins.PluginEvent) {
	fmt.Printf("[%s] HTTP 插件事件: %s\n", h.name, event.Type)
	
	// 专门处理 HTTP 插件的事件
	switch event.Type {
	case plugins.EventType("server.started"):
		fmt.Printf("  → HTTP 服务器启动成功\n")
	case plugins.EventType("server.restarted"):
		fmt.Printf("  → HTTP 服务器重启成功\n")
	case plugins.EventType("upgrade.started"):
		fmt.Printf("  → HTTP 插件开始升级\n")
	case plugins.EventType("upgrade.completed"):
		fmt.Printf("  → HTTP 插件升级完成\n")
	}
}

func (h *HTTPEventListener) GetListenerID() string {
	return h.name
}

// 示例：插件如何使用改进的事件系统
type ExamplePlugin struct {
	*plugins.BasePlugin
	name string
}

func NewExamplePlugin(name string) *ExamplePlugin {
	return &ExamplePlugin{
		BasePlugin: plugins.NewBasePlugin(name, name, "示例插件", "1.0.0", "示例插件描述", 10),
		name:       name,
	}
}

func (p *ExamplePlugin) InitializeResources(rt plugins.Runtime) error {
	fmt.Printf("插件 %s 正在初始化资源...\n", p.name)

	// 发出初始化开始事件
	rt.EmitPluginEvent(p.name, "initialization.started", map[string]any{
		"plugin": p.name,
		"step":   "starting",
	})

	// 模拟初始化过程
	// ... 初始化逻辑 ...

	// 发出初始化完成事件
	rt.EmitPluginEvent(p.name, "initialization.completed", map[string]any{
		"plugin":  p.name,
		"status":  "success",
		"version": p.Version(),
	})

	return nil
}

func (p *ExamplePlugin) Start(plugin plugins.Plugin) error {
	// 发出启动事件
	runtime := plugin.(*ExamplePlugin).getRuntime()
	if runtime != nil {
		runtime.EmitPluginEvent(p.name, "plugin.started", map[string]any{
			"plugin": p.name,
			"status": "running",
		})
	}
	return nil
}

func (p *ExamplePlugin) getRuntime() plugins.Runtime {
	// 这里需要从插件管理器中获取 runtime
	// 实际实现中可能需要通过依赖注入或其他方式获取
	return nil
}

func (p *ExamplePlugin) Name() string {
	return p.name
}

func (p *ExamplePlugin) ID() string {
	return p.name + "-id"
}

func (p *ExamplePlugin) Description() string {
	return "示例插件 " + p.name
}

func (p *ExamplePlugin) Version() string {
	return "1.0.0"
}

func (p *ExamplePlugin) Weight() int {
	return 10
}

func (p *ExamplePlugin) GetDependencies() []plugins.Dependency {
	return []plugins.Dependency{
		{
			ID:       "other-plugin",
			Required: false,
		},
	}
}

package main

import (
	"fmt"
	"log"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
)

// 示例：展示如何使用统一的 Runtime 进行插件间资源共享

func main() {
	// 创建插件管理器
	manager := app.NewPluginManager()

	// 模拟配置
	conf := createMockConfig()

	// 加载插件
	manager.LoadPlugins(conf)

	// 获取统一的 runtime
	runtime := manager.GetRuntime()

	// 演示资源共享
	demonstrateResourceSharing(runtime)

	// 演示配置访问
	demonstrateConfigAccess(runtime)

	// 演示类型安全的资源访问
	demonstrateTypedResourceAccess(runtime)
}

func createMockConfig() config.Config {
	// 这里应该创建实际的配置
	// 为了演示，我们返回一个简单的配置
	return nil
}

func demonstrateResourceSharing(runtime plugins.Runtime) {
	fmt.Println("=== 资源共享演示 ===")

	// 插件 A 注册一个资源
	err := runtime.RegisterResource("database-connection", "mysql://localhost:3306/mydb")
	if err != nil {
		log.Printf("注册资源失败: %v", err)
		return
	}
	fmt.Println("✓ 插件 A 注册了数据库连接资源")

	// 插件 B 获取这个资源
	dbConn, err := runtime.GetResource("database-connection")
	if err != nil {
		log.Printf("获取资源失败: %v", err)
		return
	}
	fmt.Printf("✓ 插件 B 获取到数据库连接: %v\n", dbConn)

	// 插件 C 注册另一个资源
	err = runtime.RegisterResource("cache-client", "redis://localhost:6379")
	if err != nil {
		log.Printf("注册缓存资源失败: %v", err)
		return
	}
	fmt.Println("✓ 插件 C 注册了缓存客户端资源")

	// 插件 D 获取缓存资源
	cacheClient, err := runtime.GetResource("cache-client")
	if err != nil {
		log.Printf("获取缓存资源失败: %v", err)
		return
	}
	fmt.Printf("✓ 插件 D 获取到缓存客户端: %v\n", cacheClient)
}

func demonstrateConfigAccess(runtime plugins.Runtime) {
	fmt.Println("\n=== 配置访问演示 ===")

	// 获取配置
	conf := runtime.GetConfig()
	if conf != nil {
		fmt.Println("✓ 成功获取到配置")
		// 这里可以访问具体的配置值
		// 例如：conf.Value("database").Scan(&dbConfig)
	} else {
		fmt.Println("⚠ 配置为空（这是正常的，因为我们使用的是模拟配置）")
	}
}

func demonstrateTypedResourceAccess(runtime plugins.Runtime) {
	fmt.Println("\n=== 类型安全资源访问演示 ===")

	// 注册一个类型化的资源
	type DatabaseConfig struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Database string `json:"database"`
	}

	dbConfig := &DatabaseConfig{
		Host:     "localhost",
		Port:     3306,
		Database: "mydb",
	}

	// 使用类型安全的注册
	err := plugins.RegisterTypedResource(runtime, "typed-db-config", dbConfig)
	if err != nil {
		log.Printf("注册类型化资源失败: %v", err)
		return
	}
	fmt.Println("✓ 注册了类型化的数据库配置资源")

	// 使用类型安全的获取
	retrievedConfig, err := plugins.GetTypedResource[*DatabaseConfig](runtime, "typed-db-config")
	if err != nil {
		log.Printf("获取类型化资源失败: %v", err)
		return
	}
	fmt.Printf("✓ 成功获取类型化资源: Host=%s, Port=%d, Database=%s\n",
		retrievedConfig.Host, retrievedConfig.Port, retrievedConfig.Database)

	// 演示类型不匹配的错误处理
	_, err = plugins.GetTypedResource[string](runtime, "typed-db-config")
	if err != nil {
		fmt.Printf("✓ 类型安全检查生效: %v\n", err)
	}
}

// 示例：插件如何使用 Runtime
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

	// 注册自己的资源
	err := rt.RegisterResource(p.name+"-resource", "这是 "+p.name+" 的资源")
	if err != nil {
		return err
	}

	// 尝试获取其他插件的资源
	if otherResource, err := rt.GetResource("other-plugin-resource"); err == nil {
		fmt.Printf("插件 %s 发现其他插件的资源: %v\n", p.name, otherResource)
	}

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

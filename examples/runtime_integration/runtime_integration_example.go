package main

import (
	"fmt"
	"log"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/plugins"
)

// 示例：展示逻辑分离的 Runtime 设计

func main() {
	// 创建插件管理器
	manager := app.NewPluginManager()

	// 模拟配置
	conf := createMockConfig()

	// 加载插件
	manager.LoadPlugins(conf)

	// 获取统一的 runtime
	runtime := manager.GetRuntime()

	// 演示逻辑分离的资源管理
	demonstrateLogicalSeparation(runtime)

	// 演示共享资源管理
	demonstrateSharedResources(runtime)

	// 演示类型安全的资源访问
	demonstrateTypedResourceAccess(runtime)
}

func createMockConfig() config.Config {
	// 这里应该创建实际的配置
	// 为了演示，我们返回一个简单的配置
	return nil
}

func demonstrateLogicalSeparation(runtime plugins.Runtime) {
	fmt.Println("=== 逻辑分离的资源管理演示 ===")

	// 演示共享资源（所有插件都可以访问）
	fmt.Println("\n1. 共享资源管理:")

	// 注册共享资源
	err := runtime.RegisterSharedResource("global-database", "mysql://localhost:3306/global_db")
	if err != nil {
		log.Printf("注册共享数据库资源失败: %v", err)
		return
	}
	fmt.Println("✓ 注册了全局数据库连接（共享资源）")

	// 获取共享资源
	globalDB, err := runtime.GetSharedResource("global-database")
	if err != nil {
		log.Printf("获取共享数据库资源失败: %v", err)
		return
	}
	fmt.Printf("✓ 获取到共享数据库连接: %v\n", globalDB)

	// 演示私有资源（插件内部使用）
	fmt.Println("\n2. 私有资源管理:")

	// 注意：当前的简单实现中，私有资源暂时注册为共享资源
	// 在实际使用中，需要插件上下文来区分私有和共享资源
	err = runtime.RegisterPrivateResource("private-cache", "redis://localhost:6379/private")
	if err != nil {
		log.Printf("注册私有缓存资源失败: %v", err)
		return
	}
	fmt.Println("✓ 注册了私有缓存连接（私有资源）")

	// 获取私有资源
	privateCache, err := runtime.GetPrivateResource("private-cache")
	if err != nil {
		fmt.Printf("⚠ 获取私有资源失败（这是预期的，因为需要插件上下文）: %v\n", err)
	} else {
		fmt.Printf("✓ 获取到私有缓存连接: %v\n", privateCache)
	}
}

func demonstrateSharedResources(runtime plugins.Runtime) {
	fmt.Println("\n=== 共享资源管理演示 ===")

	// 模拟多个插件注册不同类型的共享资源
	resources := map[string]string{
		"http-server":   "HTTP服务器实例",
		"database-pool": "数据库连接池",
		"cache-client":  "Redis缓存客户端",
		"message-queue": "消息队列客户端",
		"config-center": "配置中心客户端",
	}

	for name, description := range resources {
		err := runtime.RegisterSharedResource(name, description)
		if err != nil {
			log.Printf("注册资源 %s 失败: %v", name, err)
			continue
		}
		fmt.Printf("✓ 插件注册了共享资源: %s (%s)\n", name, description)
	}

	// 演示其他插件获取这些共享资源
	fmt.Println("\n其他插件获取共享资源:")
	for name := range resources {
		resource, err := runtime.GetSharedResource(name)
		if err != nil {
			log.Printf("获取资源 %s 失败: %v", name, err)
			continue
		}
		fmt.Printf("✓ 获取到共享资源: %s = %v\n", name, resource)
	}
}

func demonstrateTypedResourceAccess(runtime plugins.Runtime) {
	fmt.Println("\n=== 类型安全资源访问演示 ===")

	// 定义类型化的配置结构
	type DatabaseConfig struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Database string `json:"database"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	type CacheConfig struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Database int    `json:"database"`
		Password string `json:"password"`
	}

	// 注册类型化的共享资源
	dbConfig := &DatabaseConfig{
		Host:     "localhost",
		Port:     3306,
		Database: "production_db",
		Username: "admin",
		Password: "secret",
	}

	cacheConfig := &CacheConfig{
		Host:     "localhost",
		Port:     6379,
		Database: 0,
		Password: "redis_secret",
	}

	// 使用类型安全的注册
	err := plugins.RegisterTypedResource(runtime, "typed-db-config", dbConfig)
	if err != nil {
		log.Printf("注册类型化数据库配置失败: %v", err)
		return
	}
	fmt.Println("✓ 注册了类型化的数据库配置资源")

	err = plugins.RegisterTypedResource(runtime, "typed-cache-config", cacheConfig)
	if err != nil {
		log.Printf("注册类型化缓存配置失败: %v", err)
		return
	}
	fmt.Println("✓ 注册了类型化的缓存配置资源")

	// 使用类型安全的获取
	retrievedDBConfig, err := plugins.GetTypedResource[*DatabaseConfig](runtime, "typed-db-config")
	if err != nil {
		log.Printf("获取类型化数据库配置失败: %v", err)
		return
	}
	fmt.Printf("✓ 成功获取类型化数据库配置: Host=%s, Port=%d, Database=%s\n",
		retrievedDBConfig.Host, retrievedDBConfig.Port, retrievedDBConfig.Database)

	retrievedCacheConfig, err := plugins.GetTypedResource[*CacheConfig](runtime, "typed-cache-config")
	if err != nil {
		log.Printf("获取类型化缓存配置失败: %v", err)
		return
	}
	fmt.Printf("✓ 成功获取类型化缓存配置: Host=%s, Port=%d, Database=%d\n",
		retrievedCacheConfig.Host, retrievedCacheConfig.Port, retrievedCacheConfig.Database)

	// 演示类型不匹配的错误处理
	_, err = plugins.GetTypedResource[string](runtime, "typed-db-config")
	if err != nil {
		fmt.Printf("✓ 类型安全检查生效: %v\n", err)
	}
}

// 示例：插件如何使用逻辑分离的 Runtime
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

	// 注册私有资源（插件内部使用）
	privateResourceName := p.name + "-private-resource"
	err := rt.RegisterPrivateResource(privateResourceName, "这是 "+p.name+" 的私有资源")
	if err != nil {
		fmt.Printf("插件 %s 注册私有资源失败: %v\n", p.name, err)
	} else {
		fmt.Printf("插件 %s 注册了私有资源: %s\n", p.name, privateResourceName)
	}

	// 注册共享资源（其他插件可以使用）
	sharedResourceName := p.name + "-shared-resource"
	err = rt.RegisterSharedResource(sharedResourceName, "这是 "+p.name+" 的共享资源")
	if err != nil {
		fmt.Printf("插件 %s 注册共享资源失败: %v\n", p.name, err)
	} else {
		fmt.Printf("插件 %s 注册了共享资源: %s\n", p.name, sharedResourceName)
	}

	// 尝试获取其他插件的共享资源
	otherPlugins := []string{"database-plugin", "cache-plugin", "http-plugin"}
	for _, otherPlugin := range otherPlugins {
		if otherPlugin != p.name {
			resourceName := otherPlugin + "-shared-resource"
			if resource, err := rt.GetSharedResource(resourceName); err == nil {
				fmt.Printf("插件 %s 发现其他插件的共享资源: %s = %v\n", p.name, resourceName, resource)
			}
		}
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

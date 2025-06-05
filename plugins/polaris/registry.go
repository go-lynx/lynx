package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-lynx/lynx/app/log"
)

// NewServiceRegistry 方法用于创建一个新的 Polaris 服务注册器
// The NewServiceRegistry method is used to create a new Polaris service registrar.
// 该方法会记录服务注册的日志信息，并根据配置初始化 Polaris 注册器
// This method logs service registration information and initializes the Polaris registrar based on the configuration.
func (p *PlugPolaris) NewServiceRegistry() registry.Registrar {
	// 使用 Lynx 应用的日志工具记录服务正在注册的信息
	// Use the Lynx application's logging tool to record that the service is being registered.
	log.Infof("Service registration in progress")
	// 调用 GetPolaris() 函数获取 Polaris 实例，并通过一系列选项配置注册器
	// Call the GetPolaris() function to obtain a Polaris instance and configure the registrar with a series of options.
	reg := GetPolaris().Registry(
		// 使用 WithRegistryServiceToken 方法设置服务令牌
		// Use the WithRegistryServiceToken method to set the service token.
		polaris.WithRegistryServiceToken(GetPlugin().conf.Token),
		// 使用 WithRegistryTimeout 方法设置注册超时时间
		// Use the WithRegistryTimeout method to set the registration timeout.
		polaris.WithRegistryTimeout(GetPlugin().conf.Timeout.AsDuration()),
		// 使用 WithRegistryTTL 方法设置注册的 TTL（生存时间）
		// Use the WithRegistryTTL method to set the registration TTL (Time To Live).
		polaris.WithRegistryTTL(int(GetPlugin().conf.Ttl)),
		// 使用 WithRegistryWeight 方法设置注册的权重
		// Use the WithRegistryWeight method to set the registration weight.
		polaris.WithRegistryWeight(int(GetPlugin().conf.Weight)),
	)
	// 返回创建好的服务注册器实例
	// Return the created service registrar instance.
	return reg
}

// NewServiceDiscovery 方法用于创建一个新的 Polaris 服务发现器
// The NewServiceDiscovery method is used to create a new Polaris service discoverer.
// 该方法会记录服务发现的日志信息，并根据配置初始化 Polaris 发现器
// This method logs service discovery information and initializes the Polaris discoverer based on the configuration.
func (p *PlugPolaris) NewServiceDiscovery() registry.Discovery {
	// 使用 Lynx 应用的日志工具记录服务正在进行发现的信息
	// Use the Lynx application's logging tool to record that the service discovery is in progress.
	log.Infof("Service discovery in progress")
	// 调用 GetPolaris() 函数获取 Polaris 实例，并通过一系列选项配置发现器
	// Call the GetPolaris() function to obtain a Polaris instance and configure the discoverer with a series of options.
	reg := GetPolaris().Registry(
		// 使用 WithRegistryServiceToken 方法设置服务令牌
		// Use the WithRegistryServiceToken method to set the service token.
		polaris.WithRegistryServiceToken(GetPlugin().conf.Token),
		// 使用 WithRegistryTimeout 方法设置发现超时时间
		// Use the WithRegistryTimeout method to set the discovery timeout.
		polaris.WithRegistryTimeout(GetPlugin().conf.Timeout.AsDuration()),
		// 使用 WithRegistryTTL 方法设置发现的 TTL（生存时间）
		// Use the WithRegistryTTL method to set the discovery TTL (Time To Live).
		polaris.WithRegistryTTL(int(GetPlugin().conf.Ttl)),
		// 使用 WithRegistryWeight 方法设置发现的权重
		// Use the WithRegistryWeight method to set the discovery weight.
		polaris.WithRegistryWeight(int(GetPlugin().conf.Weight)),
	)
	// 返回创建好的服务发现器实例
	// Return the created service discoverer instance.
	return reg
}

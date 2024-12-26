package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-lynx/lynx/app"
)

// NewServiceRegistry 方法用于创建一个新的 Polaris 服务注册器
func (p *PlugPolaris) NewServiceRegistry() registry.Registrar {
	// 使用 Lynx 应用的 Helper 记录服务注册正在进行的信息
	app.Lynx().Helper().Infof("Service registration in progress")
	// 调用 GetPolaris() 函数获取 Polaris 实例，并使用 WithRegistryServiceToken 方法设置服务令牌
	r := GetPolaris().Registry(
		polaris.WithRegistryServiceToken(GetPlugin().conf.Token),
		// 使用 WithRegistryTimeout 方法设置注册超时时间
		polaris.WithRegistryTimeout(GetPlugin().conf.Timeout.AsDuration()),
		// 使用 WithRegistryTTL 方法设置注册 TTL
		polaris.WithRegistryTTL(int(GetPlugin().conf.Ttl)),
		// 使用 WithRegistryWeight 方法设置注册权重
		polaris.WithRegistryWeight(int(GetPlugin().conf.Weight)),
	)
	// 返回创建的服务注册器实例
	return r
}

// NewServiceDiscovery 方法用于创建一个新的 Polaris 服务发现器
func (p *PlugPolaris) NewServiceDiscovery() registry.Discovery {
	// 使用 Lynx 应用的 Helper 记录服务发现正在进行的信息
	app.Lynx().Helper().Infof("Service discovery in progress")
	// 调用 GetPolaris() 函数获取 Polaris 实例，并使用 WithRegistryServiceToken 方法设置服务令牌
	r := GetPolaris().Registry(
		polaris.WithRegistryServiceToken(GetPlugin().conf.Token),
		// 使用 WithRegistryTimeout 方法设置注册超时时间
		polaris.WithRegistryTimeout(GetPlugin().conf.Timeout.AsDuration()),
		// 使用 WithRegistryTTL 方法设置注册 TTL
		polaris.WithRegistryTTL(int(GetPlugin().conf.Ttl)),
		// 使用 WithRegistryWeight 方法设置注册权重
		polaris.WithRegistryWeight(int(GetPlugin().conf.Weight)),
	)
	// 返回创建的服务发现器实例
	return r
}

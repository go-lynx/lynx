package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-lynx/lynx/app"
)

// NewServiceRegistry PolarisRegistry
func (p *PlugPolaris) NewServiceRegistry() registry.Registrar {
	app.Lynx().Helper().Infof("Service registration in progress")
	r := GetPolaris().Registry(
		polaris.WithRegistryServiceToken(GetPlugin().conf.Token),
		polaris.WithRegistryTimeout(GetPlugin().conf.Timeout.AsDuration()),
		polaris.WithRegistryTTL(int(GetPlugin().conf.Ttl)),
		polaris.WithRegistryWeight(int(GetPlugin().conf.Weight)),
	)
	return r
}

// NewServiceDiscovery PolarisDiscovery
func (p *PlugPolaris) NewServiceDiscovery() registry.Discovery {
	app.Lynx().Helper().Infof("Service discovery in progress")
	r := GetPolaris().Registry(
		polaris.WithRegistryServiceToken(GetPlugin().conf.Token),
		polaris.WithRegistryTimeout(GetPlugin().conf.Timeout.AsDuration()),
		polaris.WithRegistryTTL(int(GetPlugin().conf.Ttl)),
		polaris.WithRegistryWeight(int(GetPlugin().conf.Weight)),
	)
	return r
}

package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/conf"
)

// NewServiceRegistry PolarisRegistry
func NewServiceRegistry(lynx *conf.Lynx) registry.Registrar {
	app.GetHelper().Infof("Service registration in progress")
	r := GetPolaris().Registry(
		polaris.WithRegistryServiceToken(lynx.Polaris.Token),
		polaris.WithRegistryTimeout(lynx.Polaris.Timeout.AsDuration()),
		polaris.WithRegistryTTL(int(lynx.Polaris.Ttl)),
		polaris.WithRegistryWeight(int(lynx.Polaris.Weight)),
	)
	return r
}

// NewServiceDiscovery PolarisDiscovery
func NewServiceDiscovery(lynx *conf.Lynx) registry.Discovery {
	app.GetHelper().Infof("Service discovery in progress")
	r := GetPolaris().Registry(
		polaris.WithRegistryServiceToken(lynx.Polaris.Token),
		polaris.WithRegistryTimeout(lynx.Polaris.Timeout.AsDuration()),
		polaris.WithRegistryTTL(int(lynx.Polaris.Ttl)),
		polaris.WithRegistryWeight(int(lynx.Polaris.Weight)),
	)
	return r
}

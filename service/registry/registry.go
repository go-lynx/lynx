package registry

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-lynx/lynx/boot"
)

// NewServiceRegistry PolarisRegistry
func NewServiceRegistry(lynx *boot.Lynx) registry.Registrar {
	boot.dfLog.Infof("Service registration in progress")
	r := boot.p.Registry(
		polaris.WithRegistryServiceToken(lynx.Polaris.Token),
		polaris.WithRegistryTimeout(lynx.Polaris.Timeout.AsDuration()),
		polaris.WithRegistryTTL(int(lynx.Polaris.Ttl)),
		polaris.WithRegistryWeight(int(lynx.Polaris.Weight)),
	)
	return r
}

// NewServiceDiscovery PolarisDiscovery
func NewServiceDiscovery(lynx *boot.Lynx) registry.Discovery {
	boot.dfLog.Infof("Service discovery in progress")
	r := boot.p.Registry(
		polaris.WithRegistryServiceToken(lynx.Polaris.Token),
		polaris.WithRegistryTimeout(lynx.Polaris.Timeout.AsDuration()),
		polaris.WithRegistryTTL(int(lynx.Polaris.Ttl)),
		polaris.WithRegistryWeight(int(lynx.Polaris.Weight)),
	)
	return r
}

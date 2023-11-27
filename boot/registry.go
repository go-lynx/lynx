package boot

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/registry"
)

// NewServiceRegistry PolarisRegistry
func NewServiceRegistry(lynx *Lynx) registry.Registrar {
	dfLog.Infof("Service registration in progress")
	r := GetPolaris().Registry(
		polaris.WithRegistryServiceToken(lynx.Polaris.Token),
		polaris.WithRegistryTimeout(lynx.Polaris.Timeout.AsDuration()),
		polaris.WithRegistryTTL(int(lynx.Polaris.Ttl)),
		polaris.WithRegistryWeight(int(lynx.Polaris.Weight)),
	)
	return r
}

// NewServiceDiscovery PolarisDiscovery
func NewServiceDiscovery(lynx *Lynx) registry.Discovery {
	dfLog.Infof("Service discovery in progress")
	r := GetPolaris().Registry(
		polaris.WithRegistryServiceToken(lynx.Polaris.Token),
		polaris.WithRegistryTimeout(lynx.Polaris.Timeout.AsDuration()),
		polaris.WithRegistryTTL(int(lynx.Polaris.Ttl)),
		polaris.WithRegistryWeight(int(lynx.Polaris.Weight)),
	)
	return r
}

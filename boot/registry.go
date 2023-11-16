package boot

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-lynx/lynx/conf"
)

// NewServiceRegistry PolarisRegistry
func NewServiceRegistry(c *conf.Bootstrap) registry.Registrar {
	dfLog.Infof("Service registration in progress")
	r := Polaris().Registry(
		polaris.WithRegistryServiceToken(c.Server.Polaris.Token),
		polaris.WithRegistryTimeout(c.Server.Polaris.Timeout.AsDuration()),
		polaris.WithRegistryTTL(int(c.Server.Polaris.Ttl)),
		polaris.WithRegistryWeight(int(c.Server.Polaris.Weight)),
	)
	return r
}

// NewServiceDiscovery PolarisDiscovery
func NewServiceDiscovery(c *conf.Bootstrap) registry.Discovery {
	dfLog.Infof("Service discovery in progress")
	r := Polaris().Registry(
		polaris.WithRegistryServiceToken(c.Server.Polaris.Token),
		polaris.WithRegistryTimeout(c.Server.Polaris.Timeout.AsDuration()),
		polaris.WithRegistryTTL(int(c.Server.Polaris.Ttl)),
		polaris.WithRegistryWeight(int(c.Server.Polaris.Weight)),
	)
	return r
}

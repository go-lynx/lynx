package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-lynx/lynx/app"
)

// NewServiceRegistry method is used to create a new Polaris service registrar
func (p *PlugPolaris) NewServiceRegistry() registry.Registrar {
	// Use the Lynx application's Helper to record information about the service registration in progress
	app.Lynx().GetLogHelper().Infof("Service registration in progress")
	// Call the GetPolaris() function to obtain a Polaris instance and use the WithRegistryServiceToken method to set the service token
	r := GetPolaris().Registry(
		polaris.WithRegistryServiceToken(GetPlugin().conf.Token),
		// Use the WithRegistryTimeout method to set the registration timeout time
		polaris.WithRegistryTimeout(GetPlugin().conf.Timeout.AsDuration()),
		// Use the WithRegistryTTL method to set the registration TTL
		polaris.WithRegistryTTL(int(GetPlugin().conf.Ttl)),
		// Use the WithRegistryWeight method to set the registration weight
		polaris.WithRegistryWeight(int(GetPlugin().conf.Weight)),
	)
	// Return the created service registrar instance
	return r
}

// NewServiceDiscovery method is used to create a new Polaris service discoverer
func (p *PlugPolaris) NewServiceDiscovery() registry.Discovery {
	// Use the Lynx application's Helper to record information about the service discovery in progress
	app.Lynx().GetLogHelper().Infof("Service discovery in progress")
	// Call the GetPolaris() function to obtain a Polaris instance and use the WithRegistryServiceToken method to set the service token
	r := GetPolaris().Registry(
		polaris.WithRegistryServiceToken(GetPlugin().conf.Token),
		// Use the WithRegistryTimeout method to set the registration timeout time
		polaris.WithRegistryTimeout(GetPlugin().conf.Timeout.AsDuration()),
		// Use the WithRegistryTTL method to set the registration TTL
		polaris.WithRegistryTTL(int(GetPlugin().conf.Ttl)),
		// Use the WithRegistryWeight method to set the registration weight
		polaris.WithRegistryWeight(int(GetPlugin().conf.Weight)),
	)
	// Return the created service discoverer instance
	return r
}

package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/app/log"
)

// NewNodeRouter method is used to create a new Polaris node filter for synchronizing the routing policies of remote services
func (p *PlugPolaris) NewNodeRouter(name string) selector.NodeFilter {
	// Use the Lynx application's Helper to record information about the routing policy for the specified name being synchronized
	log.Infof("Synchronizing [%v] routing policy", name)
	// Call the GetPolaris().NodeFilter method to obtain a node filter instance and set the service name
	return GetPolaris().NodeFilter(polaris.WithRouterService(name))
}

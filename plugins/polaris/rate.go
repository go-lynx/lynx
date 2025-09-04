package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/app/log"
)

// NewNodeRouter creates Polaris node filter
// Used for synchronizing remote service routing policies
func (p *PlugPolaris) NewNodeRouter(name string) selector.NodeFilter {
	log.Infof("Synchronizing [%v] routing policy", name)
	return GetPolaris().NodeFilter(polaris.WithRouterService(name))
}

package subscribe

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/boot"
	polaris2 "github.com/go-lynx/lynx/plugin/polaris"
)

// NewNodeRouter Synchronize Remote Service Routing Strategy
func NewNodeRouter(name string) selector.NodeFilter {
	boot.GetHelper().Infof("Synchronizing [%v] routing policy", name)
	return polaris2.GetPolaris().NodeFilter(polaris.WithRouterService(name))
}

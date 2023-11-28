package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/app"
)

// NewNodeRouter Synchronize Remote Service Routing Strategy
func NewNodeRouter(name string) selector.NodeFilter {
	app.GetHelper().Infof("Synchronizing [%v] routing policy", name)
	return GetPolaris().NodeFilter(polaris.WithRouterService(name))
}

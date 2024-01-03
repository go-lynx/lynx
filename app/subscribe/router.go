package subscribe

import (
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/app"
)

func (g *GrpcSubscribe) nodeFilter() selector.NodeFilter {
	if app.Lynx().ControlPlane() == nil {
		return nil
	}
	return app.Lynx().ControlPlane().NewNodeRouter(g.name)
}

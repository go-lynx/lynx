package subscribe

import (
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/app"
)

func (g *GrpcSubscribe) nodeFilter() selector.NodeFilter {
	if app.Lynx().GetControlPlane() == nil {
		return nil
	}
	return app.Lynx().GetControlPlane().NewNodeRouter(g.name)
}

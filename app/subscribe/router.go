package subscribe

import (
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/app"
)

// nodeFilter 方法为 GrpcSubscribe 结构体生成一个 selector.NodeFilter 实例。
// 若控制平面不可用，则返回 nil；否则，调用控制平面的方法创建一个新的节点路由器。
func (g *GrpcSubscribe) nodeFilter() selector.NodeFilter {
	// 检查应用的控制平面是否为 nil
	if app.Lynx().GetControlPlane() == nil {
		// 若控制平面为 nil，返回 nil 表示不使用节点过滤器
		return nil
	}
	// 若控制平面存在，调用其 NewNodeRouter 方法创建一个新的节点路由器
	return app.Lynx().GetControlPlane().NewNodeRouter(g.svcName)
}

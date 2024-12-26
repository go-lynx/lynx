package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/app"
)

// NewNodeRouter 方法用于创建一个新的 Polaris 节点过滤器，用于同步远程服务的路由策略
func (p *PlugPolaris) NewNodeRouter(name string) selector.NodeFilter {
	// 使用 Lynx 应用的 Helper 记录正在同步指定名称的路由策略的信息
	app.Lynx().Helper().Infof("Synchronizing [%v] routing policy", name)
	// 调用 GetPolaris().NodeFilter 方法获取一个节点过滤器实例，并设置服务名称
	return GetPolaris().NodeFilter(polaris.WithRouterService(name))
}

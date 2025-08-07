package polaris

import (
	"github.com/go-kratos/kratos/contrib/polaris/v2"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-lynx/lynx/app/log"
)

// NewNodeRouter 创建 Polaris 节点过滤器
// 用于同步远程服务的路由策略
func (p *PlugPolaris) NewNodeRouter(name string) selector.NodeFilter {
	log.Infof("Synchronizing [%v] routing policy", name)
	return GetPolaris().NodeFilter(polaris.WithRouterService(name))
}

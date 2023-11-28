package subscribe

import (
	"github.com/go-kratos/kratos/v2/selector"
)

type Router interface {
	NewNodeRouter(name string) selector.NodeFilter
}

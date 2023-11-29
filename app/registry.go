package app

import (
	"github.com/go-lynx/lynx/app/conf"
)

type Registry interface {
	NewServiceRegistry(lynx *conf.Lynx)
	NewServiceDiscovery(lynx *conf.Lynx)
}

package plugin

import (
	"github.com/go-lynx/lynx/conf"
)

type Plugin interface {
	Weight() int
	Name() string
	Load(b *conf.Bootstrap) (Plugin, error)
	Unload() error
}

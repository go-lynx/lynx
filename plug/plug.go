package plug

import (
	"github.com/go-lynx/lynx/conf"
)

type Plug interface {
	Weight() int
	Name() string
	Load(b *conf.Bootstrap) (Plug, error)
	Unload() error
}

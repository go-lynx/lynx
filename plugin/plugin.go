package plugin

import (
	"github.com/go-kratos/kratos/v2/config"
)

type Plugin interface {
	Weight() int
	Name() string
	ConfigPrefix() string
	Load(config.Value) (Plugin, error)
	Unload() error
}

package plugin

import (
	"github.com/go-kratos/kratos/v2/config"
)

type Plugin interface {
	pluginLoader
	pluginSupport
}

type pluginLoader interface {
	Load(config.Value) (Plugin, error)
	Unload() error
}

type pluginSupport interface {
	Name() string
	Weight() int
	DependsOn() []string
	ConfPrefix() string
}

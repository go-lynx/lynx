package plugin

import (
	"github.com/go-kratos/kratos/v2/config"
)

type Plugin interface {
	LoaderPlugin
	SupportPlugin
}

type LoaderPlugin interface {
	Load(config.Value) (Plugin, error)
	Unload() error
}

type SupportPlugin interface {
	Name() string
	Weight() int
	DependsOn(config.Value) []string
	ConfPrefix() string
}

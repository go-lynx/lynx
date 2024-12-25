package plugin

import (
	"github.com/go-kratos/kratos/v2/config"
)

type Plugin interface {
	Load(config.Value) (Plugin, error)
	Unload() error
	SupportPlugin
}

type SupportPlugin interface {
	Name() string
	Weight() int
	DependsOn(config.Value) []string
	ConfPrefix() string
}

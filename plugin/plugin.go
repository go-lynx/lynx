package plugin

import (
	"errors"
)

var (
	factory = newGlobalPluginFactory()
)

func GlobalPluginFactory() *Factory {
	return factory
}

type Plugin interface {
	Weight() int
	Name() string
	Load(config interface{}) (Plugin, error)
	Unload() error
}

type Manger interface {
	LoadPlugins()
	UnloadPlugins()
	LoadSpecificPlugins(plugins []string)
	UnloadSpecificPlugins(plugins []string)
}

type Factory struct {
	creators map[string]func() Plugin
}

func newGlobalPluginFactory() *Factory {
	return &Factory{
		creators: make(map[string]func() Plugin),
	}
}

func (f *Factory) Register(name string, creator func() Plugin) {
	f.creators[name] = creator
}

func (f *Factory) Create(name string) (Plugin, error) {
	creator, exists := f.creators[name]
	if !exists {
		return nil, errors.New("invalid plugin name")
	}
	return creator(), nil
}

func (f *Factory) Exists(name string) bool {
	_, exists := f.creators[name]
	return exists
}

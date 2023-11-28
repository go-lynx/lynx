package plugin

import (
	"errors"
)

type Plugin interface {
	Weight() int
	Name() string
	Load(config interface{}) (Plugin, error)
	Unload() error
}

type Manger interface {
	LoadPlugins()
	UnloadPlugins()
}

type Factory struct {
	creators map[string]func() Plugin
}

func NewFactory() *Factory {
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

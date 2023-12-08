package plugin

import (
	"errors"
	"github.com/go-kratos/kratos/v2/config"
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
	ConfigPrefix() string
	Load(config.Value) (Plugin, error)
	Unload() error
}

type Factory struct {
	// registerTable: configPrefix -> pluginNames
	registerTable map[string][]string
	creators      map[string]func() Plugin
}

func newGlobalPluginFactory() *Factory {
	return &Factory{
		registerTable: make(map[string][]string),
		creators:      make(map[string]func() Plugin),
	}
}

func (f *Factory) Register(name string, configPrefix string, creator func() Plugin) {
	// For security considerations, plugins with the same name cannot be overwritten.
	if _, exists := f.creators[name]; exists {
		panic(errors.New("plugin with the same name already exists pluginName:" + name))
	}
	f.creators[name] = creator
	pluginNames, exists := f.registerTable[configPrefix]
	if !exists {
		newPluginNames := make([]string, 0)
		f.registerTable[configPrefix] = append(newPluginNames, name)
	} else {
		f.registerTable[configPrefix] = append(pluginNames, name)
	}
}

func (f *Factory) GetRegisterTable() map[string][]string {
	return f.registerTable
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

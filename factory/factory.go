package factory

import (
	"errors"
	"github.com/go-lynx/lynx/plugin"
)

var (
	factory = newGlobalPluginFactory()
)

type PluginFactory interface {
	CreateByName(pluginName string) (plugin.Plugin, error)
	PluginDefinitionRegistry
}

type PluginDefinitionRegistry interface {
	Register(pluginName string, confPrefix string, creator func() plugin.Plugin)
	GetRegisterTable() map[string][]string
	Exists(pluginName string) bool
}

func GlobalPluginFactory() PluginFactory {
	return factory
}

type Factory struct {
	// registerTable: configPrefix -> pluginNames
	registerTable map[string][]string
	creators      map[string]func() plugin.Plugin
}

func newGlobalPluginFactory() *Factory {
	return &Factory{
		registerTable: make(map[string][]string),
		creators:      make(map[string]func() plugin.Plugin),
	}
}

func (f *Factory) Register(name string, confPrefix string, creator func() plugin.Plugin) {
	// For security considerations, plugins with the same name cannot be overwritten.
	if _, exists := f.creators[name]; exists {
		panic(errors.New("plugin with the same name already exists pluginName:" + name))
	}
	f.creators[name] = creator
	pluginNames, exists := f.registerTable[confPrefix]
	if !exists {
		newPluginNames := make([]string, 0)
		f.registerTable[confPrefix] = append(newPluginNames, name)
	} else {
		f.registerTable[confPrefix] = append(pluginNames, name)
	}
}

func (f *Factory) GetRegisterTable() map[string][]string {
	return f.registerTable
}

func (f *Factory) CreateByName(name string) (plugin.Plugin, error) {
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

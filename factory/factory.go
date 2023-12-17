package factory

import (
	"errors"
	"github.com/go-lynx/lynx/plugin"
)

var (
	globalFactory = newPluginFactory()
)

// PluginFactory combines the responsibilities of creating and managing plugins.
type PluginFactory interface {
	PluginCreator
	PluginRegistry
}

// PluginCreator provides a method for creating plugins by name.
type PluginCreator interface {
	CreateByName(pluginName string) (plugin.Plugin, error)
}

// PluginRegistry provides methods for registering, checking existence, and removing plugins.
type PluginRegistry interface {
	Register(pluginName string, confPrefix string, creator func() plugin.Plugin)
	GetRegisterTable() map[string][]string
	Exists(pluginName string) bool
	Remove(pluginName string)
}

// GlobalPluginFactory returns a global instance of PluginFactory.
func GlobalPluginFactory() PluginFactory {
	return globalFactory
}

// LynxPluginFactory is an implementation of PluginFactory.
type LynxPluginFactory struct {
	// registerTable maps configuration prefixes to plugin names.
	registerTable map[string][]string
	// creators stores the creator functions for each plugin.
	creators map[string]func() plugin.Plugin
}

// newPluginFactory creates a new instance of LynxPluginFactory.
func newPluginFactory() *LynxPluginFactory {
	return &LynxPluginFactory{
		registerTable: make(map[string][]string),
		creators:      make(map[string]func() plugin.Plugin),
	}
}

// Register adds a new plugin to the factory.
func (f *LynxPluginFactory) Register(name string, confPrefix string, creator func() plugin.Plugin) {
	if _, exists := f.creators[name]; exists {
		panic(errors.New("plugin with the same name already exists pluginName:" + name))
	}
	f.creators[name] = creator
	pluginNames, exists := f.registerTable[confPrefix]
	if !exists {
		f.registerTable[confPrefix] = []string{name}
	} else {
		f.registerTable[confPrefix] = append(pluginNames, name)
	}
}

// Remove deletes a plugin from the factory.
func (f *LynxPluginFactory) Remove(name string) {
	delete(f.creators, name)
	for confPrefix, pluginNames := range f.registerTable {
		for i, pluginName := range pluginNames {
			if pluginName == name {
				f.registerTable[confPrefix] = append(pluginNames[:i], pluginNames[i+1:]...)
				break
			}
		}
	}
	if len(f.registerTable[name]) == 0 {
		delete(f.registerTable, name)
	}
}

// GetRegisterTable returns a map of all registered plugins and their corresponding configuration prefixes.
func (f *LynxPluginFactory) GetRegisterTable() map[string][]string {
	return f.registerTable
}

// CreateByName creates a new plugin instance given its name.
func (f *LynxPluginFactory) CreateByName(name string) (plugin.Plugin, error) {
	creator, exists := f.creators[name]
	if !exists {
		return nil, errors.New("invalid plugin name")
	}
	return creator(), nil
}

// Exists checks if a plugin is registered in the factory.
func (f *LynxPluginFactory) Exists(name string) bool {
	_, exists := f.creators[name]
	return exists
}

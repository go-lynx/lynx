package app

import (
	"github.com/go-lynx/lynx/plugin"
)

type LynxPluginManager struct {
	plugMap map[string]plugin.Plugin
	plugins []plugin.Plugin
	factory *plugin.Factory
}

func NewLynxPluginManager(p ...plugin.Plugin) *LynxPluginManager {
	m := &LynxPluginManager{
		plugins: p,
		factory: plugin.GlobalPluginFactory(),
		plugMap: make(map[string]plugin.Plugin),
	}
	return m
}

func (m *LynxPluginManager) LoadPlugins() {
	// Load plugins based on weight
	for i := 0; i < len(m.plugins); i++ {
		p, err := m.plugins[i].Load(nil)
		if err != nil {
			Lynx().dfLog.Errorf("Exception in initializing %v plugin :", p.Name(), err)
			panic(err)
		}
		m.pluginCheck(p.Name())
		m.plugMap[p.Name()] = p
	}
}

func (m *LynxPluginManager) pluginCheck(name string) {
	// Check for duplicate plugin names
	if existingPlugin, exists := m.plugMap[name]; exists {
		Lynx().dfLog.Errorf("Duplicate plugin name: %v . Existing Plugin: %v", name, existingPlugin)
		panic("Duplicate plugin name: " + name)
	}
}

func (m *LynxPluginManager) UnloadPlugins() {
	for i := 0; i < len(m.plugins); i++ {
		err := m.plugins[i].Unload()
		if err != nil {
			Lynx().dfLog.Errorf("Exception in uninstalling %v plugin", m.plugins[i].Name(), err)
		}
	}
}

func (m *LynxPluginManager) GetPlugin(name string) plugin.Plugin {
	return m.plugMap[name]
}

type ByWeight []plugin.Plugin

func (a ByWeight) Len() int           { return len(a) }
func (a ByWeight) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByWeight) Less(i, j int) bool { return a[i].Weight() > a[j].Weight() }

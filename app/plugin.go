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
		plugins: make([]plugin.Plugin, 10),
		factory: plugin.GlobalPluginFactory(),
		plugMap: make(map[string]plugin.Plugin),
	}

	// Manually set plugins
	if p != nil && len(p) > 1 {
		m.plugins = append(m.plugins, p...)
		for i := 0; i < len(p); i++ {
			m.plugMap[p[i].Name()] = p[i]
		}
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

func (m *LynxPluginManager) LoadSpecificPlugins(plugins []string) {
	// Load plugins based on weight
	for i := 0; i < len(plugins); i++ {
		p, err := m.plugMap[plugins[i]].Load(nil)
		if err != nil {
			Lynx().dfLog.Errorf("Exception in initializing %v plugin :", p.Name(), err)
			panic(err)
		}
	}
}

func (m *LynxPluginManager) UnloadSpecificPlugins(plugins []string) {
	for i := 0; i < len(plugins); i++ {
		err := m.plugMap[plugins[i]].Unload()
		if err != nil {
			Lynx().dfLog.Errorf("Exception in uninstalling %v plugin", plugins[i], err)
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

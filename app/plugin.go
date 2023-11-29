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

	if p != nil && len(p) > 1 {
		m.plugins = append(m.plugins, p...)
		for i := 0; i < len(p); i++ {
			m.pluginCheck(p[i].Name())
			m.plugMap[p[i].Name()] = p[i]
		}
	}
	return m
}

func (m *LynxPluginManager) LoadPlugins() {
	plugMarks := ParseConfig()
	for i := 0; i < len(plugMarks); i++ {
		if _, exists := m.plugMap[plugMarks[i]]; !exists {
			p, err := m.factory.Create(plugMarks[i])
			if err != nil {
				Lynx().dfLog.Errorf("Plugin factory load error: %v", err)
				panic(err)
			}
			m.plugins = append(m.plugins, p)
			m.pluginCheck(p.Name())
			m.plugMap[p.Name()] = p
		}
	}

	// Load plugins based on weight
	for i := 0; i < len(m.plugins); i++ {
		p, err := m.plugins[i].Load(nil)
		if err != nil {
			Lynx().dfLog.Errorf("Exception in initializing %v plugin :", p.Name(), err)
			panic(err)
		}
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

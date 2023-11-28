package app

import (
	"github.com/go-lynx/lynx/plugin"
)

type LynxPluginManager struct {
	plugMap map[string]plugin.Plugin
	plugins []plugin.Plugin
	factory *plugin.Factory
}

func (m *LynxPluginManager) Init(p ...plugin.Plugin) {
	m.factory = plugin.NewFactory()
	// Which plugins to load through the configuration file
}

func (m *LynxPluginManager) LoadPlugins() {
	if m.plugins == nil {
		m.Init()
	}
	// Load plugins based on weight
	for i := 0; i < len(m.plugins); i++ {
		p, err := m.plugins[i].Load(nil)
		if err != nil {
			dfLog.Errorf("Exception in initializing %v plugin :", p.Name(), err)
			panic(err)
		}
		m.pluginCheck(p.Name())
		m.plugMap[p.Name()] = p
	}
}

func (m *LynxPluginManager) pluginCheck(name string) {
	// Check for duplicate plugin names
	if existingPlugin, exists := m.plugMap[name]; exists {
		dfLog.Errorf("Duplicate plugin name: %v . Existing Plugin: %v", name, existingPlugin)
		panic("Duplicate plugin name: " + name)
	}
}

func (m *LynxPluginManager) UnloadPlugins() {
	for i := 0; i < len(m.plugins); i++ {
		err := m.plugins[i].Unload()
		if err != nil {
			dfLog.Errorf("Exception in uninstalling %v plugin", m.plugins[i].Name(), err)
		}
	}
}

type ByWeight []plugin.Plugin

func (a ByWeight) Len() int           { return len(a) }
func (a ByWeight) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByWeight) Less(i, j int) bool { return a[i].Weight() > a[j].Weight() }

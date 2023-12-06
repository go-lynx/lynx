package app

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/plugin"
	"sort"
)

type LynxPluginManager struct {
	plugMap map[string]plugin.Plugin
	plugins []plugin.Plugin
	factory *plugin.Factory
}

func NewLynxPluginManager(p ...plugin.Plugin) *LynxPluginManager {
	m := &LynxPluginManager{
		plugins: make([]plugin.Plugin, 0),
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

func (m *LynxPluginManager) LoadPlugins(conf config.Config) {
	if m.plugins == nil || len(m.plugins) == 0 {
		return
	}

	size := len(m.plugins)
	sort.Sort(ByWeight(m.plugins))
	for i := 0; i < size; i++ {
		_, err := m.plugins[i].Load(conf.Value(m.plugins[i].ConfigPrefix()))
		if err != nil {
			Lynx().GetHelper().Errorf("Exception in initializing %v plugin :", m.plugins[i].Name(), err)
			panic(err)
		}
	}
}

func (m *LynxPluginManager) UnloadPlugins() {
	size := len(m.plugins)
	for i := 0; i < size; i++ {
		err := m.plugins[i].Unload()
		if err != nil {
			Lynx().GetHelper().Errorf("Exception in uninstalling %v plugin", m.plugins[i].Name(), err)
		}
	}
}

func (m *LynxPluginManager) LoadSpecificPlugins(name []string, conf config.Config) {
	if name == nil || len(name) == 0 {
		return
	}

	// Load plugins based on weight
	var plugs []plugin.Plugin
	for i := 0; i < len(name); i++ {
		plugs = append(plugs, m.plugMap[name[i]])
	}

	sort.Sort(ByWeight(plugs))
	for i := 0; i < len(plugs); i++ {
		_, err := plugs[i].Load(conf.Value(plugs[i].ConfigPrefix()))
		if err != nil {
			Lynx().GetHelper().Errorf("Exception in initializing %v plugin :", plugs[i].Name(), err)
			panic(err)
		}
	}
}

func (m *LynxPluginManager) UnloadSpecificPlugins(name []string) {
	for i := 0; i < len(name); i++ {
		err := m.plugMap[name[i]].Unload()
		if err != nil {
			Lynx().GetHelper().Errorf("Exception in uninstalling %v plugin", name[i], err)
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

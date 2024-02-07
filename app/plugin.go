package app

import (
	"fmt"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/factory"
	"github.com/go-lynx/lynx/plugin"
	"sort"
)

type LynxPluginManager interface {
	LoadPlugins(config.Config)
	UnloadPlugins()
	LoadPluginsByName([]string, config.Config)
	UnloadPluginsByName([]string)
	GetPlugin(name string) plugin.Plugin
	PreparePlug(config config.Config) []string
}

type DefaultLynxPluginManager struct {
	pluginMap  map[string]plugin.Plugin
	pluginList []plugin.Plugin
	factory    factory.PluginFactory
}

func NewLynxPluginManager(p ...plugin.Plugin) LynxPluginManager {
	m := &DefaultLynxPluginManager{
		pluginList: make([]plugin.Plugin, 0),
		factory:    factory.GlobalPluginFactory(),
		pluginMap:  make(map[string]plugin.Plugin),
	}

	// Manually set pluginList
	if p != nil && len(p) > 1 {
		m.pluginList = append(m.pluginList, p...)
		for i := 0; i < len(p); i++ {
			m.pluginMap[p[i].Name()] = p[i]
		}
	}
	return m
}

type PluginWithLevel struct {
	plugin.Plugin
	level int
}

func (m *DefaultLynxPluginManager) TopologicalSort(plugins []plugin.Plugin) ([]PluginWithLevel, error) {
	// First, build a map from plugin name to the actual plugin.
	nameToPlugin := make(map[string]plugin.Plugin)
	for _, p := range plugins {
		nameToPlugin[p.Name()] = p
	}

	// Then, build the adjacency list for the graph.
	graph := make(map[string][]string)
	for _, p := range plugins {
		var on []string
		if Lynx() != nil && Lynx().GlobalConfig() != nil {
			on = p.DependsOn(Lynx().GlobalConfig().Value(p.ConfPrefix()))
		} else {
			on = p.DependsOn(nil)
		}

		for _, dep := range on {
			// If the dependency exists, add it to the graph.
			if _, ok := nameToPlugin[dep]; ok {
				graph[p.Name()] = append(graph[p.Name()], dep)
			} else {
				panic(fmt.Sprintf("Plugin %s depends on unknown plugin %s", p.Name(), dep))
			}
		}
	}

	// Perform the topological sort.
	result := make([]PluginWithLevel, 0, len(plugins))
	visited := make(map[string]bool)
	level := make(map[string]int)
	var visit func(string) error
	visit = func(name string) error {
		if !visited[name] {
			visited[name] = true
			maxLevel := 0
			for _, dep := range graph[name] {
				if err := visit(dep); err != nil {
					return err
				}
				if level[dep] > maxLevel {
					maxLevel = level[dep]
				}
			}
			level[name] = maxLevel + 1
			result = append(result, PluginWithLevel{nameToPlugin[name], level[name]})
		} else if !contains(result, nameToPlugin[name]) {
			return fmt.Errorf("cyclic dependency involving %s", name)
		}
		return nil
	}
	for _, p := range plugins {
		if err := visit(p.Name()); err != nil {
			return nil, err
		}
	}

	// Sort pluginList with the same level by weight.
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].level == result[j].level {
			return result[i].Weight() > result[j].Weight()
		}
		return result[i].level < result[j].level
	})

	return result, nil
}

func contains(slice []PluginWithLevel, item plugin.Plugin) bool {
	for _, v := range slice {
		if v.Plugin == item {
			return true
		}
	}
	return false
}

func (m *DefaultLynxPluginManager) LoadPlugins(conf config.Config) {
	plugins, err := m.TopologicalSort(m.pluginList)
	if err != nil {
		Lynx().Helper().Errorf("Exception in topological sorting pluginList :", err)
		panic(err)
	}

	size := len(plugins)
	for i := 0; i < size; i++ {
		_, err := plugins[i].Load(conf.Value(plugins[i].ConfPrefix()))
		if err != nil {
			Lynx().Helper().Errorf("Exception in initializing %v plugin :", plugins[i].Name(), err)
			panic(err)
		}
	}
}

func (m *DefaultLynxPluginManager) UnloadPlugins() {
	size := len(m.pluginList)
	for i := 0; i < size; i++ {
		err := m.pluginList[i].Unload()
		if err != nil {
			Lynx().Helper().Errorf("Exception in uninstalling %v plugin", m.pluginList[i].Name(), err)
		}
	}
}

func (m *DefaultLynxPluginManager) LoadPluginsByName(name []string, conf config.Config) {
	if name == nil || len(name) == 0 {
		return
	}

	var pluginList []plugin.Plugin
	for i := 0; i < len(name); i++ {
		pluginList = append(pluginList, m.pluginMap[name[i]])
	}

	// Sort pluginList with the same level by weight.
	plugins, err := m.TopologicalSort(pluginList)
	if err != nil {
		Lynx().Helper().Errorf("Exception in topological sorting pluginList :", err)
		panic(err)
	}

	for i := 0; i < len(plugins); i++ {
		_, err := plugins[i].Load(conf.Value(plugins[i].ConfPrefix()))
		if err != nil {
			Lynx().Helper().Errorf("Exception in initializing %v plugin :", plugins[i].Name(), err)
			panic(err)
		}
	}
}

func (m *DefaultLynxPluginManager) UnloadPluginsByName(name []string) {
	for i := 0; i < len(name); i++ {
		err := m.pluginMap[name[i]].Unload()
		if err != nil {
			Lynx().Helper().Errorf("Exception in uninstalling %v plugin", name[i], err)
		}
	}
}

func (m *DefaultLynxPluginManager) GetPlugin(name string) plugin.Plugin {
	return m.pluginMap[name]
}

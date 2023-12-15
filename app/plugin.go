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
	LoadSpecificPlugins([]string, config.Config)
	UnloadSpecificPlugins([]string)
	GetPlugin(name string) plugin.Plugin
	PreparePlug(config config.Config) []string
}

type DefaultLynxPluginManager struct {
	plugMap map[string]plugin.Plugin
	plugins []plugin.Plugin
	factory factory.PluginFactory
}

func NewDefaultLynxPluginManager(p ...plugin.Plugin) LynxPluginManager {
	m := &DefaultLynxPluginManager{
		plugins: make([]plugin.Plugin, 0),
		factory: factory.GlobalPluginFactory(),
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
		for _, dep := range p.DependsOn() {
			graph[p.Name()] = append(graph[p.Name()], dep)
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

	// Sort plugins with the same level by weight.
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
	plugins, err := m.TopologicalSort(m.plugins)
	if err != nil {
		Lynx().Helper().Errorf("Exception in topological sorting plugins :", err)
		panic(err)
	}

	for _, p := range plugins {
		_, err := p.Plugin.Load(conf.Value(p.Plugin.ConfPrefix()))
		if err != nil {
			Lynx().Helper().Errorf("Exception in initializing %v plugin :", p.Plugin.Name(), err)
			panic(err)
		}
	}
}

func (m *DefaultLynxPluginManager) UnloadPlugins() {
	size := len(m.plugins)
	for i := 0; i < size; i++ {
		err := m.plugins[i].Unload()
		if err != nil {
			Lynx().Helper().Errorf("Exception in uninstalling %v plugin", m.plugins[i].Name(), err)
		}
	}
}

func (m *DefaultLynxPluginManager) LoadSpecificPlugins(name []string, conf config.Config) {
	if name == nil || len(name) == 0 {
		return
	}

	// Load plugins based on weight
	var pluginList []plugin.Plugin
	for i := 0; i < len(name); i++ {
		pluginList = append(pluginList, m.plugMap[name[i]])
	}
	plugins, err := m.TopologicalSort(pluginList)
	if err != nil {
		Lynx().Helper().Errorf("Exception in topological sorting plugins :", err)
		panic(err)
	}

	for i := 0; i < len(plugins); i++ {
		_, err := pluginList[i].Load(conf.Value(pluginList[i].ConfPrefix()))
		if err != nil {
			Lynx().Helper().Errorf("Exception in initializing %v plugin :", pluginList[i].Name(), err)
			panic(err)
		}
	}
}

func (m *DefaultLynxPluginManager) UnloadSpecificPlugins(name []string) {
	for i := 0; i < len(name); i++ {
		err := m.plugMap[name[i]].Unload()
		if err != nil {
			Lynx().Helper().Errorf("Exception in uninstalling %v plugin", name[i], err)
		}
	}
}

func (m *DefaultLynxPluginManager) GetPlugin(name string) plugin.Plugin {
	return m.plugMap[name]
}

type ByWeight []plugin.Plugin

func (a ByWeight) Len() int           { return len(a) }
func (a ByWeight) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByWeight) Less(i, j int) bool { return a[i].Weight() > a[j].Weight() }

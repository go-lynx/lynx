package app

import (
	"fmt"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-lynx/lynx/plugin"
	"testing"
)

type MockPlugin struct {
	name       string
	depends    []string
	weight     int
	confPrefix string
}

func (m *MockPlugin) Name() string {
	return m.name
}

func (m *MockPlugin) DependsOn(b config.Value) []string {
	return m.depends
}

func (m *MockPlugin) Weight() int {
	return m.weight
}

func (m *MockPlugin) ConfPrefix() string {
	return m.confPrefix
}

func (m *MockPlugin) Load(c config.Value) (plugin.Plugin, error) {
	return m, nil
}

func (m *MockPlugin) Unload() error {
	return nil
}

func TestTopologicalSort(t *testing.T) {
	manager := NewDefaultLynxPluginManager()

	pluginA := &MockPlugin{name: "A", depends: []string{}, weight: 1}
	pluginB := &MockPlugin{name: "B", depends: []string{"A"}, weight: 1}
	pluginC := &MockPlugin{name: "C", depends: []string{"B"}, weight: 1}
	pluginD := &MockPlugin{name: "D", depends: []string{"C", "A", "E"}, weight: 2}
	pluginE := &MockPlugin{name: "E", depends: []string{}, weight: 3}

	manager.(*DefaultLynxPluginManager).plugins = []plugin.Plugin{
		pluginA,
		pluginB,
		pluginC,
		pluginD,
		pluginE,
	}

	result, err := manager.(*DefaultLynxPluginManager).TopologicalSort(manager.(*DefaultLynxPluginManager).plugins)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expectedOrder := []string{"E", "A", "B", "C", "D"}
	for i, p := range result {
		fmt.Printf("%v\n", p.Plugin.Name())
		if p.Plugin.Name() != expectedOrder[i] {
			t.Errorf("Expected order %v, but got %v", expectedOrder[i], p.Plugin.Name())
		}
	}
}

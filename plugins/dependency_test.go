package plugins

import (
	"testing"
)

func TestDependencyGraph(t *testing.T) {
	graph := NewDependencyGraph()

	mockPlugin1 := &MockPlugin{id: "plugin1", name: "Plugin 1"}
	mockPlugin2 := &MockPlugin{id: "plugin2", name: "Plugin 2"}

	graph.AddPlugin(mockPlugin1)
	graph.AddPlugin(mockPlugin2)

	dependency := &Dependency{
		ID:       "plugin2",
		Name:     "Plugin 2",
		Type:     DependencyTypeRequired,
		Required: true,
	}

	graph.AddDependency("plugin1", dependency)

	deps := graph.GetDependencies("plugin1")
	if len(deps) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(deps))
	}
}

type MockPlugin struct {
	id   string
	name string
}

func (m *MockPlugin) ID() string {
	return m.id
}

func (m *MockPlugin) Name() string {
	return m.name
}

func (m *MockPlugin) Version() string {
	return "1.0.0"
}

func (m *MockPlugin) Description() string {
	return "Mock plugin for testing"
}

func (m *MockPlugin) Dependencies() []*Dependency {
	return nil
}

func (m *MockPlugin) Initialize(plugin Plugin, rt Runtime) error {
	return nil
}

func (m *MockPlugin) Start(plugin Plugin) error {
	return nil
}

func (m *MockPlugin) Stop(plugin Plugin) error {
	return nil
}

func (m *MockPlugin) CheckHealth() error {
	return nil
}

func (m *MockPlugin) CleanupTasks() error {
	return nil
}

func (m *MockPlugin) GetDependencies() []Dependency {
	return nil
}

func (m *MockPlugin) InitializeResources(rt Runtime) error {
	return nil
}

func (m *MockPlugin) StartupTasks() error {
	return nil
}

func (m *MockPlugin) Status(plugin Plugin) PluginStatus {
	return StatusActive
}

func (m *MockPlugin) Weight() int {
	return 1
}

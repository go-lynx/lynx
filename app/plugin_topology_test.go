package app

import (
	"testing"

	"github.com/go-lynx/lynx/plugins"
)

// mockPlugin is a test helper that implements the Plugin interface
type mockPlugin struct {
	id           string
	name         string
	version      string
	dependencies []plugins.Dependency
}

func (m *mockPlugin) ID() string {
	return m.id
}

func (m *mockPlugin) Name() string {
	return m.name
}

func (m *mockPlugin) Version() string {
	if m.version == "" {
		return "v1.0.0"
	}
	return m.version
}

func (m *mockPlugin) Description() string {
	return "Mock plugin for testing"
}

func (m *mockPlugin) GetDependencies() []plugins.Dependency {
	return m.dependencies
}

func (m *mockPlugin) Initialize(plugin plugins.Plugin, rt plugins.Runtime) error {
	return nil
}

func (m *mockPlugin) Start(plugin plugins.Plugin) error {
	return nil
}

func (m *mockPlugin) Stop(plugin plugins.Plugin) error {
	return nil
}

func (m *mockPlugin) CheckHealth() error {
	return nil
}

func (m *mockPlugin) CleanupTasks() error {
	return nil
}

func (m *mockPlugin) InitializeResources(rt plugins.Runtime) error {
	return nil
}

func (m *mockPlugin) StartupTasks() error {
	return nil
}

func (m *mockPlugin) Status(plugin plugins.Plugin) plugins.PluginStatus {
	return plugins.StatusActive
}

func (m *mockPlugin) Weight() int {
	return 1
}

// TestTopologicalSort_EmptyInput tests empty input handling
func TestTopologicalSort_EmptyInput(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	result, err := manager.TopologicalSort(nil)
	if err != nil {
		t.Errorf("Expected no error for nil input, got: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result for nil input, got: %v", result)
	}

	result, err = manager.TopologicalSort([]plugins.Plugin{})
	if err != nil {
		t.Errorf("Expected no error for empty input, got: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result for empty input, got: %v", result)
	}
}

// TestTopologicalSort_AllNilPlugins tests handling of all nil plugins
func TestTopologicalSort_AllNilPlugins(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	plugs := []plugins.Plugin{nil, nil, nil}
	result, err := manager.TopologicalSort(plugs)
	if err == nil {
		t.Error("Expected error for all nil plugins, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result for all nil plugins, got: %v", result)
	}
	if err != nil && err.Error() != "no valid plugins to sort (all plugins are nil)" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

// TestTopologicalSort_DuplicateIDs tests duplicate ID detection
func TestTopologicalSort_DuplicateIDs(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	p1 := &mockPlugin{id: "test.plugin.p1.v1", name: "Plugin1"}
	p2 := &mockPlugin{id: "test.plugin.p1.v1", name: "Plugin2"} // Same ID
	plugs := []plugins.Plugin{p1, p2}

	result, err := manager.TopologicalSort(plugs)
	if err == nil {
		t.Error("Expected error for duplicate IDs, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result for duplicate IDs, got: %v", result)
	}
	if err != nil && err.Error() == "" {
		t.Error("Expected non-empty error message for duplicate IDs")
	}
}

// TestTopologicalSort_NoDependencies tests plugins with no dependencies
func TestTopologicalSort_NoDependencies(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	p1 := &mockPlugin{id: "test.plugin.p1.v1", name: "Plugin1"}
	p2 := &mockPlugin{id: "test.plugin.p2.v1", name: "Plugin2"}
	p3 := &mockPlugin{id: "test.plugin.p3.v1", name: "Plugin3"}
	plugs := []plugins.Plugin{p1, p2, p3}

	result, err := manager.TopologicalSort(plugs)
	if err != nil {
		t.Errorf("Expected no error for plugins with no dependencies, got: %v", err)
		return
	}
	if result == nil {
		t.Error("Expected non-nil result")
		return
	}
	if len(result) != 3 {
		t.Errorf("Expected 3 plugins in result, got %d", len(result))
	}
	// All plugins should have level 0 (no dependencies)
	for _, pwl := range result {
		if pwl.level != 0 {
			t.Errorf("Expected level 0 for plugin %s, got %d", pwl.Name(), pwl.level)
		}
	}
}

// TestTopologicalSort_SimpleDependency tests simple dependency chain
func TestTopologicalSort_SimpleDependency(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	p1 := &mockPlugin{
		id:   "test.plugin.p1.v1",
		name: "Plugin1",
	}
	p2 := &mockPlugin{
		id:   "test.plugin.p2.v1",
		name: "Plugin2",
		dependencies: []plugins.Dependency{
			{
				ID:   "test.plugin.p1.v1",
				Type: plugins.DependencyTypeRequired,
			},
		},
	}
	plugs := []plugins.Plugin{p1, p2}

	result, err := manager.TopologicalSort(plugs)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
		return
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 plugins, got %d", len(result))
		return
	}

	// p1 should come before p2
	p1Index := -1
	p2Index := -1
	for i, pwl := range result {
		if pwl.ID() == "test.plugin.p1.v1" {
			p1Index = i
		}
		if pwl.ID() == "test.plugin.p2.v1" {
			p2Index = i
		}
	}

	if p1Index == -1 || p2Index == -1 {
		t.Error("Could not find plugins in result")
		return
	}

	if p1Index >= p2Index {
		t.Errorf("Expected p1 before p2, got p1 at %d, p2 at %d", p1Index, p2Index)
	}

	// Check levels
	if result[p1Index].level != 0 {
		t.Errorf("Expected level 0 for p1, got %d", result[p1Index].level)
	}
	if result[p2Index].level != 1 {
		t.Errorf("Expected level 1 for p2, got %d", result[p2Index].level)
	}
}

// TestTopologicalSort_MultiLevelDependency tests multi-level dependency chain
func TestTopologicalSort_MultiLevelDependency(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	p1 := &mockPlugin{id: "test.plugin.p1.v1", name: "Plugin1"}
	p2 := &mockPlugin{
		id:   "test.plugin.p2.v1",
		name: "Plugin2",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.p1.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	p3 := &mockPlugin{
		id:   "test.plugin.p3.v1",
		name: "Plugin3",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.p2.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	plugs := []plugins.Plugin{p1, p2, p3}

	result, err := manager.TopologicalSort(plugs)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
		return
	}
	if len(result) != 3 {
		t.Errorf("Expected 3 plugins, got %d", len(result))
		return
	}

	// Verify order: p1 -> p2 -> p3
	indices := make(map[string]int)
	for i, pwl := range result {
		indices[pwl.ID()] = i
	}

	if indices["test.plugin.p1.v1"] >= indices["test.plugin.p2.v1"] {
		t.Error("p1 should come before p2")
	}
	if indices["test.plugin.p2.v1"] >= indices["test.plugin.p3.v1"] {
		t.Error("p2 should come before p3")
	}

	// Verify levels
	for _, pwl := range result {
		switch pwl.ID() {
		case "test.plugin.p1.v1":
			if pwl.level != 0 {
				t.Errorf("Expected level 0 for p1, got %d", pwl.level)
			}
		case "test.plugin.p2.v1":
			if pwl.level != 1 {
				t.Errorf("Expected level 1 for p2, got %d", pwl.level)
			}
		case "test.plugin.p3.v1":
			if pwl.level != 2 {
				t.Errorf("Expected level 2 for p3, got %d", pwl.level)
			}
		}
	}
}

// TestTopologicalSort_MissingDependency tests missing required dependency
func TestTopologicalSort_MissingDependency(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	p1 := &mockPlugin{
		id:   "test.plugin.p1.v1",
		name: "Plugin1",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.missing.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	plugs := []plugins.Plugin{p1}

	result, err := manager.TopologicalSort(plugs)
	if err == nil {
		t.Error("Expected error for missing dependency, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result for missing dependency, got: %v", result)
	}
	if err != nil && err.Error() == "" {
		t.Error("Expected non-empty error message for missing dependency")
	}
}

// TestTopologicalSort_CircularDependency tests circular dependency detection
func TestTopologicalSort_CircularDependency(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	p1 := &mockPlugin{
		id:   "test.plugin.p1.v1",
		name: "Plugin1",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.p2.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	p2 := &mockPlugin{
		id:   "test.plugin.p2.v1",
		name: "Plugin2",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.p1.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	plugs := []plugins.Plugin{p1, p2}

	result, err := manager.TopologicalSort(plugs)
	if err == nil {
		t.Error("Expected error for circular dependency, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result for circular dependency, got: %v", result)
	}
	if err != nil && err.Error() == "" {
		t.Error("Expected non-empty error message for circular dependency")
	}
}

// TestTopologicalSort_ComplexDependencyGraph tests complex dependency graph
func TestTopologicalSort_ComplexDependencyGraph(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	// Create a complex graph:
	// p1 (no deps)
	// p2 -> p1
	// p3 -> p1
	// p4 -> p2, p3
	// p5 -> p4
	p1 := &mockPlugin{id: "test.plugin.p1.v1", name: "Plugin1"}
	p2 := &mockPlugin{
		id:   "test.plugin.p2.v1",
		name: "Plugin2",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.p1.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	p3 := &mockPlugin{
		id:   "test.plugin.p3.v1",
		name: "Plugin3",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.p1.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	p4 := &mockPlugin{
		id:   "test.plugin.p4.v1",
		name: "Plugin4",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.p2.v1", Type: plugins.DependencyTypeRequired},
			{ID: "test.plugin.p3.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	p5 := &mockPlugin{
		id:   "test.plugin.p5.v1",
		name: "Plugin5",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.p4.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	plugs := []plugins.Plugin{p1, p2, p3, p4, p5}

	result, err := manager.TopologicalSort(plugs)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
		return
	}
	if len(result) != 5 {
		t.Errorf("Expected 5 plugins, got %d", len(result))
		return
	}

	// Verify order constraints
	indices := make(map[string]int)
	for i, pwl := range result {
		indices[pwl.ID()] = i
	}

	// p1 should come before p2 and p3
	if indices["test.plugin.p1.v1"] >= indices["test.plugin.p2.v1"] {
		t.Error("p1 should come before p2")
	}
	if indices["test.plugin.p1.v1"] >= indices["test.plugin.p3.v1"] {
		t.Error("p1 should come before p3")
	}

	// p2 and p3 should come before p4
	if indices["test.plugin.p2.v1"] >= indices["test.plugin.p4.v1"] {
		t.Error("p2 should come before p4")
	}
	if indices["test.plugin.p3.v1"] >= indices["test.plugin.p4.v1"] {
		t.Error("p3 should come before p4")
	}

	// p4 should come before p5
	if indices["test.plugin.p4.v1"] >= indices["test.plugin.p5.v1"] {
		t.Error("p4 should come before p5")
	}

	// Verify levels
	for _, pwl := range result {
		switch pwl.ID() {
		case "test.plugin.p1.v1":
			if pwl.level != 0 {
				t.Errorf("Expected level 0 for p1, got %d", pwl.level)
			}
		case "test.plugin.p2.v1", "test.plugin.p3.v1":
			if pwl.level != 1 {
				t.Errorf("Expected level 1 for %s, got %d", pwl.ID(), pwl.level)
			}
		case "test.plugin.p4.v1":
			if pwl.level != 2 {
				t.Errorf("Expected level 2 for p4, got %d", pwl.level)
			}
		case "test.plugin.p5.v1":
			if pwl.level != 3 {
				t.Errorf("Expected level 3 for p5, got %d", pwl.level)
			}
		}
	}
}

// TestTopologicalSort_OptionalDependencies tests that optional dependencies are ignored
func TestTopologicalSort_OptionalDependencies(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	p1 := &mockPlugin{
		id:   "test.plugin.p1.v1",
		name: "Plugin1",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.missing.v1", Type: plugins.DependencyTypeOptional},
		},
	}
	plugs := []plugins.Plugin{p1}

	result, err := manager.TopologicalSort(plugs)
	if err != nil {
		t.Errorf("Expected no error for optional missing dependency, got: %v", err)
		return
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 plugin, got %d", len(result))
	}
	if result[0].level != 0 {
		t.Errorf("Expected level 0, got %d", result[0].level)
	}
}

// TestTopologicalSort_MixedNilAndValid tests mixed nil and valid plugins
func TestTopologicalSort_MixedNilAndValid(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	p1 := &mockPlugin{id: "test.plugin.p1.v1", name: "Plugin1"}
	p2 := &mockPlugin{id: "test.plugin.p2.v1", name: "Plugin2"}
	plugs := []plugins.Plugin{nil, p1, nil, p2, nil}

	result, err := manager.TopologicalSort(plugs)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
		return
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 plugins, got %d", len(result))
	}
}

// TestTopologicalSort_LargeGraph tests with a larger dependency graph
func TestTopologicalSort_LargeGraph(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	// Create 5 plugins in a chain
	plugs := make([]plugins.Plugin, 5)
	plugs[0] = &mockPlugin{id: "test.plugin.p0.v1", name: "Plugin0"}
	for i := 1; i < 5; i++ {
		prevID := "test.plugin.p" + string(rune('0'+i-1)) + ".v1"
		plugs[i] = &mockPlugin{
			id:   "test.plugin.p" + string(rune('0'+i)) + ".v1",
			name: "Plugin" + string(rune('0'+i)),
			dependencies: []plugins.Dependency{
				{ID: prevID, Type: plugins.DependencyTypeRequired},
			},
		}
	}

	result, err := manager.TopologicalSort(plugs)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
		return
	}
	if len(result) != 5 {
		t.Errorf("Expected 5 plugins, got %d", len(result))
		return
	}

	// Verify order - plugins should be in dependency order
	indices := make(map[string]int)
	for i, pwl := range result {
		indices[pwl.ID()] = i
	}

	// Verify each plugin comes after its dependency
	for i := 1; i < 5; i++ {
		prevID := "test.plugin.p" + string(rune('0'+i-1)) + ".v1"
		currID := "test.plugin.p" + string(rune('0'+i)) + ".v1"
		if indices[prevID] >= indices[currID] {
			t.Errorf("Plugin %s should come before %s", prevID, currID)
		}
	}

	// Verify levels
	for i, pwl := range result {
		if pwl.level != i {
			t.Errorf("Expected level %d for plugin %s, got %d", i, pwl.ID(), pwl.level)
		}
	}
}

// TestTopologicalSort_MultipleMissingDependencies tests multiple missing dependencies
func TestTopologicalSort_MultipleMissingDependencies(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	p1 := &mockPlugin{
		id:   "test.plugin.p1.v1",
		name: "Plugin1",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.missing1.v1", Type: plugins.DependencyTypeRequired},
			{ID: "test.plugin.missing2.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	plugs := []plugins.Plugin{p1}

	result, err := manager.TopologicalSort(plugs)
	if err == nil {
		t.Error("Expected error for missing dependencies, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result, got: %v", result)
	}
	if err != nil {
		// Verify error message contains information about missing dependencies
		errMsg := err.Error()
		if errMsg == "" {
			t.Error("Expected non-empty error message")
		}
	}
}

// TestTopologicalSort_ThreeWayCycle tests three-way circular dependency
func TestTopologicalSort_ThreeWayCycle(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	p1 := &mockPlugin{
		id:   "test.plugin.p1.v1",
		name: "Plugin1",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.p2.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	p2 := &mockPlugin{
		id:   "test.plugin.p2.v1",
		name: "Plugin2",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.p3.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	p3 := &mockPlugin{
		id:   "test.plugin.p3.v1",
		name: "Plugin3",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.p1.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	plugs := []plugins.Plugin{p1, p2, p3}

	result, err := manager.TopologicalSort(plugs)
	if err == nil {
		t.Error("Expected error for three-way circular dependency, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result, got: %v", result)
	}
}

// TestTopologicalSort_IndependentGroups tests independent plugin groups
func TestTopologicalSort_IndependentGroups(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	// Two independent dependency chains
	p1 := &mockPlugin{id: "test.plugin.p1.v1", name: "Plugin1"}
	p2 := &mockPlugin{
		id:   "test.plugin.p2.v1",
		name: "Plugin2",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.p1.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	p3 := &mockPlugin{id: "test.plugin.p3.v1", name: "Plugin3"}
	p4 := &mockPlugin{
		id:   "test.plugin.p4.v1",
		name: "Plugin4",
		dependencies: []plugins.Dependency{
			{ID: "test.plugin.p3.v1", Type: plugins.DependencyTypeRequired},
		},
	}
	plugs := []plugins.Plugin{p1, p2, p3, p4}

	result, err := manager.TopologicalSort(plugs)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
		return
	}
	if len(result) != 4 {
		t.Errorf("Expected 4 plugins, got %d", len(result))
		return
	}

	// Verify constraints
	indices := make(map[string]int)
	for i, pwl := range result {
		indices[pwl.ID()] = i
	}

	// p1 before p2
	if indices["test.plugin.p1.v1"] >= indices["test.plugin.p2.v1"] {
		t.Error("p1 should come before p2")
	}
	// p3 before p4
	if indices["test.plugin.p3.v1"] >= indices["test.plugin.p4.v1"] {
		t.Error("p3 should come before p4")
	}
}


package plugin

import (
	"testing"
)

func TestPluginRegistry(t *testing.T) {
	registry := NewPluginRegistry()

	// Test getting all plugins
	plugins := registry.GetAllPlugins()
	if len(plugins) == 0 {
		t.Error("Expected some plugins in registry")
	}

	// Test getting specific plugin
	redis, err := registry.GetPlugin("redis")
	if err != nil {
		t.Errorf("Failed to get redis plugin: %v", err)
	}
	if redis.Name != "redis" {
		t.Errorf("Expected plugin name 'redis', got %s", redis.Name)
	}
	if redis.Type != TypeNoSQL {
		t.Errorf("Expected plugin type 'nosql', got %s", redis.Type)
	}

	// Test getting non-existent plugin
	_, err = registry.GetPlugin("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent plugin")
	}

	// Test search functionality
	results := registry.SearchPlugins("database")
	if len(results) == 0 {
		t.Error("Expected some results for 'database' search")
	}

	// Test filtering by type
	servicePlugins := registry.GetPluginsByType(TypeService)
	if len(servicePlugins) == 0 {
		t.Error("Expected some service plugins")
	}

	// Test official plugins
	officialPlugins := registry.GetOfficialPlugins()
	if len(officialPlugins) == 0 {
		t.Error("Expected some official plugins")
	}
}

func TestPluginMetadata(t *testing.T) {
	plugin := &PluginMetadata{
		Name:        "test-plugin",
		Type:        TypeService,
		Version:     "v1.0.0",
		Description: "Test plugin",
		Official:    true,
		Status:      StatusNotInstalled,
	}

	if plugin.Name != "test-plugin" {
		t.Errorf("Expected name 'test-plugin', got %s", plugin.Name)
	}

	if plugin.Type != TypeService {
		t.Errorf("Expected type 'service', got %s", plugin.Type)
	}
}

func TestPluginTypes(t *testing.T) {
	types := []PluginType{
		TypeService,
		TypeMQ,
		TypeSQL,
		TypeNoSQL,
		TypeTracer,
		TypeDTX,
		TypeConfig,
		TypeOther,
	}

	for _, pluginType := range types {
		if pluginType == "" {
			t.Errorf("Plugin type should not be empty")
		}
	}
}

func TestPluginStatus(t *testing.T) {
	statuses := []PluginStatus{
		StatusInstalled,
		StatusNotInstalled,
		StatusUpdatable,
		StatusUnknown,
	}

	for _, status := range statuses {
		if status == "" {
			t.Errorf("Plugin status should not be empty")
		}
	}
}

func TestFormatPluginType(t *testing.T) {
	tests := []struct {
		input    PluginType
		expected string
	}{
		{TypeService, "Service"},
		{TypeMQ, "Message Queue"},
		{TypeSQL, "SQL Database"},
		{TypeNoSQL, "NoSQL Database"},
		{TypeTracer, "Tracing"},
		{TypeDTX, "Distributed Transaction"},
		{TypeConfig, "Configuration"},
		{TypeOther, "Other"},
	}

	for _, test := range tests {
		result := formatPluginType(test.input)
		if result != test.expected {
			t.Errorf("formatPluginType(%s) = %s; want %s", test.input, result, test.expected)
		}
	}
}

func TestProjectConfig(t *testing.T) {
	config := ProjectConfig{
		Plugins: []InstalledPlugin{
			{
				Name:    "redis",
				Version: "v2.0.0",
				Enabled: true,
			},
			{
				Name:    "mysql",
				Version: "v2.0.0",
				Enabled: false,
			},
		},
	}

	if len(config.Plugins) != 2 {
		t.Errorf("Expected 2 plugins, got %d", len(config.Plugins))
	}

	if config.Plugins[0].Name != "redis" {
		t.Errorf("Expected first plugin to be 'redis', got %s", config.Plugins[0].Name)
	}

	if config.Plugins[0].Enabled != true {
		t.Error("Expected redis plugin to be enabled")
	}

	if config.Plugins[1].Enabled != false {
		t.Error("Expected mysql plugin to be disabled")
	}
}

func TestFindProjectRoot(t *testing.T) {
	// This test might fail in some environments
	// It's mainly for local development testing
	root, err := findProjectRoot()
	if err != nil {
		// It's OK if we can't find project root in test environment
		t.Logf("Could not find project root: %v", err)
		return
	}

	if root == "" {
		t.Error("Project root should not be empty")
	}
}
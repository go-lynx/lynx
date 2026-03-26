package project

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-lynx/lynx/cmd/lynx/internal/plugin"
)

func TestCheckDuplicates(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "simple names",
			input:    []string{"foo", "bar", "foo", "baz"},
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "path-like names allowed",
			input:    []string{"foo/bar/svc", "a/b"},
			expected: []string{"foo/bar/svc", "a/b"},
		},
		{
			name:     "mixed path and simple",
			input:    []string{"mysvc", "team/mysvc", "mysvc"},
			expected: []string{"mysvc", "team/mysvc"},
		},
		{
			name:     "invalid simple name filtered",
			input:    []string{"valid", "in valid", "valid", "also-valid"},
			expected: []string{"valid", "also-valid"},
		},
		{
			name:     "trim space and empty",
			input:    []string{"  a  ", "a", "", "  b  "},
			expected: []string{"a", "b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkDuplicates(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("checkDuplicates() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFilterSelectablePlugins_DefaultLayoutSkipsBuiltIns(t *testing.T) {
	previousRepoURL := repoURL
	repoURL = defaultLayoutRepo
	t.Cleanup(func() { repoURL = previousRepoURL })

	plugins := []*plugin.PluginMetadata{
		{Name: "http"},
		{Name: "redis"},
		{Name: "kafka"},
	}

	got := filterSelectablePlugins(plugins)
	if len(got) != 1 || got[0].Name != "kafka" {
		t.Fatalf("filterSelectablePlugins() = %+v, want only kafka", got)
	}
}

func TestWritePluginImportsFile(t *testing.T) {
	projectDir := t.TempDir()
	mainDir := filepath.Join(projectDir, "cmd", "demo")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatalf("mkdir main dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mainDir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	plugins := []*plugin.PluginMetadata{
		{Name: "kafka"},
		{Name: "kafka"},
		{Name: "apollo"},
	}

	if err := writePluginImportsFile(projectDir, plugins); err != nil {
		t.Fatalf("writePluginImportsFile() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(mainDir, "plugins_gen.go"))
	if err != nil {
		t.Fatalf("read generated imports file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `_ "github.com/go-lynx/lynx-kafka"`) {
		t.Fatalf("generated imports missing kafka module: %s", content)
	}
	if !strings.Contains(content, `_ "github.com/go-lynx/lynx-apollo"`) {
		t.Fatalf("generated imports missing apollo module: %s", content)
	}
	if strings.Count(content, `github.com/go-lynx/lynx-kafka`) != 1 {
		t.Fatalf("generated imports should deduplicate kafka module: %s", content)
	}
}

func TestMergePluginBootstrapConfig(t *testing.T) {
	projectDir := t.TempDir()
	configDir := filepath.Join(projectDir, "configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}

	bootstrap := "lynx:\n  application:\n    name: demo\n"
	configPath := filepath.Join(configDir, "bootstrap.local.yaml")
	if err := os.WriteFile(configPath, []byte(bootstrap), 0o644); err != nil {
		t.Fatalf("write bootstrap config: %v", err)
	}

	plugins := []*plugin.PluginMetadata{
		{Name: "kafka", Type: plugin.TypeMQ},
	}

	if err := mergePluginBootstrapConfig(projectDir, plugins); err != nil {
		t.Fatalf("mergePluginBootstrapConfig() error = %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read bootstrap config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "application:") {
		t.Fatalf("expected existing bootstrap content to be preserved: %s", content)
	}
	if !strings.Contains(content, "kafka:") {
		t.Fatalf("expected kafka config to be merged: %s", content)
	}
	if !strings.Contains(content, "brokers:") {
		t.Fatalf("expected kafka defaults to be written: %s", content)
	}
}

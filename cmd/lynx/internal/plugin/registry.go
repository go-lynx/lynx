package plugin

import (
	"fmt"
	"strings"
	"sync"
)

// PluginType represents the type of plugin
type PluginType string

const (
	TypeService PluginType = "service"
	TypeMQ      PluginType = "mq"
	TypeSQL     PluginType = "sql"
	TypeNoSQL   PluginType = "nosql"
	TypeTracer  PluginType = "tracer"
	TypeDTX     PluginType = "dtx"
	TypeConfig  PluginType = "config"
	TypeOther   PluginType = "other"
)

// PluginStatus represents the installation status
type PluginStatus string

const (
	StatusInstalled    PluginStatus = "installed"
	StatusNotInstalled PluginStatus = "not_installed"
	StatusUpdatable    PluginStatus = "updatable"
	StatusUnknown      PluginStatus = "unknown"
)

// Dependency represents a plugin dependency
type Dependency struct {
	Name     string `json:"name" yaml:"name"`
	Version  string `json:"version" yaml:"version"`
	Required bool   `json:"required" yaml:"required"`
}

// PluginMetadata contains plugin information
type PluginMetadata struct {
	Name         string            `json:"name" yaml:"name"`
	Type         PluginType        `json:"type" yaml:"type"`
	Version      string            `json:"version" yaml:"version"`
	Description  string            `json:"description" yaml:"description"`
	Repository   string            `json:"repository" yaml:"repository"`
	ImportPath   string            `json:"import_path" yaml:"import_path"`
	Author       string            `json:"author" yaml:"author"`
	License      string            `json:"license" yaml:"license"`
	Dependencies []Dependency      `json:"dependencies" yaml:"dependencies"`
	Tags         []string          `json:"tags" yaml:"tags"`
	Compatible   string            `json:"compatible" yaml:"compatible"` // Compatible Lynx version
	Status       PluginStatus      `json:"status" yaml:"status"`
	InstalledVer string            `json:"installed_version,omitempty" yaml:"installed_version,omitempty"`
	ConfigFile   string            `json:"config_file,omitempty" yaml:"config_file,omitempty"`
	Enabled      bool              `json:"enabled" yaml:"enabled"`
	Official     bool              `json:"official" yaml:"official"`
	ExtraInfo    map[string]string `json:"extra_info,omitempty" yaml:"extra_info,omitempty"`
}

// PluginRegistry manages available plugins (loaded from GitHub go-lynx org).
type PluginRegistry struct {
	mu      sync.RWMutex
	plugins map[string]*PluginMetadata
}

// NewPluginRegistry creates a new empty plugin registry. Call LoadFromGitHub to fill from GitHub.
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string]*PluginMetadata),
	}
}

// LoadFromGitHub fetches the plugin list from GitHub (go-lynx org) and fills the registry.
// Optional projectRoot is used for file cache. On failure returns error (no built-in fallback).
func (r *PluginRegistry) LoadFromGitHub(projectRoot string) error {
	plugins, err := FetchPluginsFromGitHub(projectRoot)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins = make(map[string]*PluginMetadata)
	for _, p := range plugins {
		r.plugins[p.Name] = p
	}
	return nil
}

// normalizePluginName returns the registry key: "lynx-redis" -> "redis", "redis" -> "redis"
func normalizePluginName(name string) string {
	s := strings.TrimSpace(name)
	if strings.HasPrefix(s, "lynx-") {
		return strings.TrimPrefix(s, "lynx-")
	}
	return s
}

// GetPlugin returns a plugin by name (supports "redis" or "lynx-redis")
func (r *PluginRegistry) GetPlugin(name string) (*PluginMetadata, error) {
	key := normalizePluginName(name)
	r.mu.RLock()
	defer r.mu.RUnlock()
	plugin, exists := r.plugins[key]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", name)
	}
	return plugin, nil
}

// GetAllPlugins returns all plugins
func (r *PluginRegistry) GetAllPlugins() []*PluginMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()
	plugins := make([]*PluginMetadata, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}
	return plugins
}

// GetPluginsByType returns plugins filtered by type
func (r *PluginRegistry) GetPluginsByType(pluginType PluginType) []*PluginMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var plugins []*PluginMetadata
	for _, plugin := range r.plugins {
		if plugin.Type == pluginType {
			plugins = append(plugins, plugin)
		}
	}
	return plugins
}

// SearchPlugins searches plugins by keyword
func (r *PluginRegistry) SearchPlugins(keyword string) []*PluginMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()
	keyword = strings.ToLower(keyword)
	var plugins []*PluginMetadata
	for _, plugin := range r.plugins {
		if strings.Contains(strings.ToLower(plugin.Name), keyword) ||
			strings.Contains(strings.ToLower(plugin.Description), keyword) {
			plugins = append(plugins, plugin)
			continue
		}
		for _, tag := range plugin.Tags {
			if strings.Contains(strings.ToLower(tag), keyword) {
				plugins = append(plugins, plugin)
				break
			}
		}
	}
	return plugins
}

// GetOfficialPlugins returns only official plugins
func (r *PluginRegistry) GetOfficialPlugins() []*PluginMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var plugins []*PluginMetadata
	for _, plugin := range r.plugins {
		if plugin.Official {
			plugins = append(plugins, plugin)
		}
	}
	return plugins
}

// AddCustomPlugin adds a custom plugin to the registry (e.g. from URL install)
func (r *PluginRegistry) AddCustomPlugin(plugin *PluginMetadata) {
	plugin.Official = false
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[plugin.Name] = plugin
}

// UpdatePluginStatus updates the status of a plugin
func (r *PluginRegistry) UpdatePluginStatus(name string, status PluginStatus, installedVersion string) error {
	key := normalizePluginName(name)
	r.mu.Lock()
	defer r.mu.Unlock()
	plugin, exists := r.plugins[key]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}
	plugin.Status = status
	plugin.InstalledVer = installedVersion
	return nil
}

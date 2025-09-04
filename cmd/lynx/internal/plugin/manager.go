package plugin

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProjectConfig represents the project plugin configuration
type ProjectConfig struct {
	Plugins []InstalledPlugin `yaml:"plugins" json:"plugins"`
}

// InstalledPlugin represents an installed plugin configuration
type InstalledPlugin struct {
	Name       string `yaml:"name" json:"name"`
	Version    string `yaml:"version" json:"version"`
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	ConfigPath string `yaml:"config_path,omitempty" json:"config_path,omitempty"`
}

// PluginManager manages plugin operations
type PluginManager struct {
	registry      *PluginRegistry
	projectRoot   string
	configFile    string
	pluginsDir    string
	installedList []InstalledPlugin
}

// NewPluginManager creates a new plugin manager
func NewPluginManager() (*PluginManager, error) {
	// Find project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to find project root: %w", err)
	}

	manager := &PluginManager{
		registry:    NewPluginRegistry(),
		projectRoot: projectRoot,
		configFile:  filepath.Join(projectRoot, ".lynx", "plugins.yaml"),
		pluginsDir:  filepath.Join(projectRoot, "plugins"),
	}

	// Load installed plugins configuration
	if err := manager.loadInstalledPlugins(); err != nil {
		// It's OK if the config doesn't exist yet
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load plugin config: %w", err)
		}
	}

	// Scan actual installed plugins
	manager.scanInstalledPlugins()

	return manager, nil
}

// findProjectRoot finds the project root directory
func findProjectRoot() (string, error) {
	// Start from current directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Look for go.mod file
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory
			break
		}
		dir = parent
	}

	// If no go.mod found, use current directory
	return os.Getwd()
}

// loadInstalledPlugins loads the installed plugins configuration
func (m *PluginManager) loadInstalledPlugins() error {
	data, err := os.ReadFile(m.configFile)
	if err != nil {
		return err
	}

	var config ProjectConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	m.installedList = config.Plugins
	return nil
}

// saveInstalledPlugins saves the installed plugins configuration
func (m *PluginManager) saveInstalledPlugins() error {
	config := ProjectConfig{
		Plugins: m.installedList,
	}

	data, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}

	// Create .lynx directory if it doesn't exist
	configDir := filepath.Dir(m.configFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(m.configFile, data, 0644)
}

// scanInstalledPlugins scans the plugins directory for installed plugins
func (m *PluginManager) scanInstalledPlugins() {
	// Check if plugins directory exists
	if _, err := os.Stat(m.pluginsDir); os.IsNotExist(err) {
		return
	}

	// Scan plugin directories
	entries, err := os.ReadDir(m.pluginsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check each type directory (service, mq, sql, nosql, etc.)
		typeDir := filepath.Join(m.pluginsDir, entry.Name())
		pluginDirs, err := os.ReadDir(typeDir)
		if err != nil {
			continue
		}

		for _, pluginDir := range pluginDirs {
			if !pluginDir.IsDir() {
				continue
			}

			pluginName := pluginDir.Name()
			
			// Check if plugin has go.mod
			goModPath := filepath.Join(typeDir, pluginName, "go.mod")
			if _, err := os.Stat(goModPath); err == nil {
				// Update registry status
				m.registry.UpdatePluginStatus(pluginName, StatusInstalled, m.getInstalledVersion(pluginName))
			}
		}
	}
}

// getInstalledVersion gets the installed version of a plugin
func (m *PluginManager) getInstalledVersion(name string) string {
	for _, plugin := range m.installedList {
		if plugin.Name == name {
			return plugin.Version
		}
	}
	return "unknown"
}

// ListPlugins lists all available and installed plugins
func (m *PluginManager) ListPlugins(showAll bool, pluginType string) ([]*PluginMetadata, error) {
	var plugins []*PluginMetadata

	if pluginType != "" {
		// Filter by type
		plugins = m.registry.GetPluginsByType(PluginType(pluginType))
	} else if showAll {
		// Show all plugins
		plugins = m.registry.GetAllPlugins()
	} else {
		// Show only installed plugins
		for _, plugin := range m.registry.GetAllPlugins() {
			if plugin.Status == StatusInstalled {
				plugins = append(plugins, plugin)
			}
		}
	}

	return plugins, nil
}

// GetPluginInfo gets detailed information about a plugin
func (m *PluginManager) GetPluginInfo(name string) (*PluginMetadata, error) {
	return m.registry.GetPlugin(name)
}

// InstallPlugin installs a plugin
func (m *PluginManager) InstallPlugin(name string, version string, force bool) error {
	// Get plugin metadata
	plugin, err := m.registry.GetPlugin(name)
	if err != nil {
		// Try to install from URL if not in registry
		if strings.Contains(name, "/") || strings.Contains(name, ".") {
			return m.installFromURL(name, version, force)
		}
		return fmt.Errorf("plugin not found: %s", name)
	}

	// Check if already installed
	if plugin.Status == StatusInstalled && !force {
		return fmt.Errorf("plugin %s is already installed, use --force to reinstall", name)
	}

	fmt.Printf("Installing plugin: %s...\n", name)

	// Determine plugin directory
	pluginDir := filepath.Join(m.pluginsDir, string(plugin.Type), plugin.Name)

	// Create plugin directory
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Clone or download plugin
	if plugin.Repository != "" {
		if err := m.clonePlugin(plugin.Repository, pluginDir, version); err != nil {
			return fmt.Errorf("failed to clone plugin: %w", err)
		}
	} else {
		// Use go get to download
		if err := m.downloadPlugin(plugin.ImportPath, version); err != nil {
			return fmt.Errorf("failed to download plugin: %w", err)
		}
	}

	// Generate configuration template
	configFile := filepath.Join("conf", fmt.Sprintf("%s.yaml", plugin.Name))
	if err := m.generateConfigTemplate(plugin, configFile); err != nil {
		fmt.Printf("Warning: failed to generate config template: %v\n", err)
	}

	// Update installed plugins list
	m.addInstalledPlugin(plugin.Name, version, configFile)

	// Save configuration
	if err := m.saveInstalledPlugins(); err != nil {
		return fmt.Errorf("failed to save plugin configuration: %w", err)
	}

	// Update registry status
	m.registry.UpdatePluginStatus(name, StatusInstalled, version)

	// Run go mod tidy
	fmt.Println("Running go mod tidy...")
	if err := m.runGoModTidy(); err != nil {
		fmt.Printf("Warning: go mod tidy failed: %v\n", err)
	}

	fmt.Printf("✅ Plugin %s installed successfully!\n", name)
	if configFile != "" {
		fmt.Printf("📝 Configuration template created: %s\n", configFile)
	}

	return nil
}

// RemovePlugin removes a plugin
func (m *PluginManager) RemovePlugin(name string, keepConfig bool) error {
	// Get plugin metadata
	plugin, err := m.registry.GetPlugin(name)
	if err != nil {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// Check if installed
	if plugin.Status != StatusInstalled {
		return fmt.Errorf("plugin %s is not installed", name)
	}

	// Check dependencies
	if err := m.checkDependencies(name); err != nil {
		return fmt.Errorf("cannot remove plugin: %w", err)
	}

	fmt.Printf("Removing plugin: %s...\n", name)

	// Remove plugin directory
	pluginDir := filepath.Join(m.pluginsDir, string(plugin.Type), plugin.Name)
	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("failed to remove plugin directory: %w", err)
	}

	// Remove from installed list
	m.removeInstalledPlugin(name)

	// Save configuration
	if err := m.saveInstalledPlugins(); err != nil {
		return fmt.Errorf("failed to save plugin configuration: %w", err)
	}

	// Update registry status
	m.registry.UpdatePluginStatus(name, StatusNotInstalled, "")

	// Optionally remove config
	if !keepConfig {
		configFile := filepath.Join("conf", fmt.Sprintf("%s.yaml", plugin.Name))
		if err := os.Remove(configFile); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Warning: failed to remove config file: %v\n", err)
		}
	}

	// Run go mod tidy
	fmt.Println("Running go mod tidy...")
	if err := m.runGoModTidy(); err != nil {
		fmt.Printf("Warning: go mod tidy failed: %v\n", err)
	}

	fmt.Printf("✅ Plugin %s removed successfully!\n", name)
	return nil
}

// Helper functions

func (m *PluginManager) clonePlugin(repo, dir, version string) error {
	// Remove existing directory if it exists
	os.RemoveAll(dir)

	// Clone repository
	cmd := exec.Command("git", "clone", repo, dir)
	if err := cmd.Run(); err != nil {
		return err
	}

	// Checkout specific version if provided
	if version != "" && version != "latest" {
		cmd = exec.Command("git", "checkout", version)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	return nil
}

func (m *PluginManager) downloadPlugin(importPath, version string) error {
	if version == "" || version == "latest" {
		version = "@latest"
	} else if !strings.HasPrefix(version, "@") {
		version = "@" + version
	}

	cmd := exec.Command("go", "get", importPath+version)
	cmd.Dir = m.projectRoot
	return cmd.Run()
}

func (m *PluginManager) installFromURL(url, version string, force bool) error {
	fmt.Printf("Installing plugin from: %s\n", url)
	
	// Extract plugin name from URL
	parts := strings.Split(url, "/")
	name := parts[len(parts)-1]
	name = strings.TrimSuffix(name, ".git")

	// Create custom plugin metadata
	plugin := &PluginMetadata{
		Name:       name,
		Type:       TypeOther,
		Repository: url,
		ImportPath: url,
		Official:   false,
	}

	// Add to registry
	m.registry.AddCustomPlugin(plugin)

	// Install using the same process
	return m.InstallPlugin(name, version, force)
}

func (m *PluginManager) generateConfigTemplate(plugin *PluginMetadata, configFile string) error {
	// Create conf directory if it doesn't exist
	confDir := filepath.Join(m.projectRoot, "conf")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		return err
	}

	// Check if config already exists
	configPath := filepath.Join(m.projectRoot, configFile)
	if _, err := os.Stat(configPath); err == nil {
		return nil // Config already exists
	}

	// Generate basic config template
	config := map[string]interface{}{
		plugin.Name: map[string]interface{}{
			"enabled": true,
			"config": map[string]interface{}{
				"// TODO": "Add your plugin configuration here",
			},
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func (m *PluginManager) addInstalledPlugin(name, version, configPath string) {
	// Check if already in list
	for i, plugin := range m.installedList {
		if plugin.Name == name {
			m.installedList[i].Version = version
			m.installedList[i].ConfigPath = configPath
			return
		}
	}

	// Add new plugin
	m.installedList = append(m.installedList, InstalledPlugin{
		Name:       name,
		Version:    version,
		Enabled:    true,
		ConfigPath: configPath,
	})
}

func (m *PluginManager) removeInstalledPlugin(name string) {
	var newList []InstalledPlugin
	for _, plugin := range m.installedList {
		if plugin.Name != name {
			newList = append(newList, plugin)
		}
	}
	m.installedList = newList
}

func (m *PluginManager) checkDependencies(name string) error {
	// Check if other plugins depend on this one
	for _, plugin := range m.registry.GetAllPlugins() {
		if plugin.Status != StatusInstalled || plugin.Name == name {
			continue
		}
		
		for _, dep := range plugin.Dependencies {
			if dep.Name == name && dep.Required {
				return fmt.Errorf("plugin %s depends on %s", plugin.Name, name)
			}
		}
	}
	return nil
}

func (m *PluginManager) runGoModTidy() error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = m.projectRoot
	return cmd.Run()
}

// SearchPlugins searches for plugins by keyword
func (m *PluginManager) SearchPlugins(keyword string) []*PluginMetadata {
	return m.registry.SearchPlugins(keyword)
}

// ExportConfig exports the plugin configuration
func (m *PluginManager) ExportConfig(writer io.Writer, format string) error {
	config := ProjectConfig{
		Plugins: m.installedList,
	}

	switch format {
	case "json":
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(config)
	case "yaml":
		data, err := yaml.Marshal(config)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		return err
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// ImportConfig imports plugin configuration and installs missing plugins
func (m *PluginManager) ImportConfig(reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	data := ""
	for scanner.Scan() {
		data += scanner.Text() + "\n"
	}

	var config ProjectConfig
	if err := yaml.Unmarshal([]byte(data), &config); err != nil {
		// Try JSON
		if err := json.Unmarshal([]byte(data), &config); err != nil {
			return fmt.Errorf("failed to parse configuration")
		}
	}

	// Install missing plugins
	for _, plugin := range config.Plugins {
		if _, err := m.registry.GetPlugin(plugin.Name); err != nil {
			fmt.Printf("Plugin %s not found in registry, skipping...\n", plugin.Name)
			continue
		}

		if err := m.InstallPlugin(plugin.Name, plugin.Version, false); err != nil {
			fmt.Printf("Failed to install %s: %v\n", plugin.Name, err)
		}
	}

	return nil
}
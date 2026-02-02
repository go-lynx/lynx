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

// NewPluginManager creates a new plugin manager. Requires running from a project root (with go.mod).
func NewPluginManager() (*PluginManager, error) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to find project root: %w", err)
	}

	registry := NewPluginRegistry()
	if err := registry.LoadFromGitHub(projectRoot); err != nil {
		return nil, fmt.Errorf("failed to load plugin list from GitHub: %w (check network or set GITHUB_TOKEN)", err)
	}

	manager := &PluginManager{
		registry:    registry,
		projectRoot: projectRoot,
		configFile:  filepath.Join(projectRoot, ".lynx", "plugins.yaml"),
		pluginsDir:  filepath.Join(projectRoot, "plugins"),
	}

	if err := manager.loadInstalledPlugins(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load plugin config: %w", err)
	}
	manager.scanInstalledPlugins()
	return manager, nil
}

// findProjectRoot finds the project root directory (directory containing go.mod).
// Returns error if no go.mod found, so plugin commands must be run from a Go project root.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found in current or parent directories: plugin commands must be run from a Lynx/Go project root")
		}
		dir = parent
	}
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

// scanInstalledPlugins scans the plugins directory for installed plugins and updates registry status.
// Resets all plugins to NotInstalled first so that cached list does not carry over status from another project.
func (m *PluginManager) scanInstalledPlugins() {
	// Reset status for all plugins (cache may hold previous run's status from another project)
	for _, p := range m.registry.GetAllPlugins() {
		_ = m.registry.UpdatePluginStatus(p.Name, StatusNotInstalled, "")
	}

	if _, err := os.Stat(m.pluginsDir); os.IsNotExist(err) {
		return
	}

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
		// Filter by type (normalize to lowercase to match TypeService etc.)
		plugins = m.registry.GetPluginsByType(PluginType(strings.ToLower(pluginType)))
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

	// Check dependencies (use normalized plugin name; installed list stores by plugin.Name)
	if err := m.checkDependencies(plugin.Name); err != nil {
		return fmt.Errorf("cannot remove plugin: %w", err)
	}

	fmt.Printf("Removing plugin: %s...\n", name)

	// Remove plugin directory
	pluginDir := filepath.Join(m.pluginsDir, string(plugin.Type), plugin.Name)
	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("failed to remove plugin directory: %w", err)
	}

	// Remove from installed list (installed list key is plugin.Name, e.g. "redis" not "lynx-redis")
	m.removeInstalledPlugin(plugin.Name)

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
	// Clone to temp dir first; only replace target on success to avoid losing existing dir on clone failure
	tmpDir, err := os.MkdirTemp("", "lynx-plugin-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cloneCmd := exec.Command("git", "clone", repo, tmpDir)
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Checkout tag/branch only when not "latest" (latest = use default branch)
	if version != "" && version != "latest" {
		checkoutCmd := exec.Command("git", "checkout", version)
		checkoutCmd.Dir = tmpDir
		checkoutCmd.Stdout = os.Stdout
		checkoutCmd.Stderr = os.Stderr
		if err := checkoutCmd.Run(); err != nil {
			return fmt.Errorf("git checkout %s: %w", version, err)
		}
	}

	// Success: replace target with cloned content
	_ = os.RemoveAll(dir)
	if err := os.Rename(tmpDir, dir); err != nil {
		// Cross-filesystem: copy then remove temp
		if err := copyDir(tmpDir, dir); err != nil {
			return fmt.Errorf("move clone to target: %w", err)
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s, d := filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(s)
		if err != nil {
			return err
		}
		if err := os.WriteFile(d, data, 0644); err != nil {
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
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (m *PluginManager) installFromURL(url, version string, force bool) error {
	fmt.Printf("Installing plugin from: %s\n", url)

	cloneURL := toCloneURL(url)
	importPath := importPathFromCloneURL(cloneURL)
	parts := strings.Split(strings.TrimSuffix(cloneURL, ".git"), "/")
	repoName := parts[len(parts)-1]
	name := repoName
	if strings.HasPrefix(name, "lynx-") {
		name = strings.TrimPrefix(name, "lynx-")
	}

	plugin := &PluginMetadata{
		Name:       name,
		Type:       TypeOther,
		Repository: cloneURL,
		ImportPath: importPath,
		Official:   false,
	}
	InvalidatePluginCache()
	m.registry.AddCustomPlugin(plugin)
	return m.InstallPlugin(name, version, force)
}

// toCloneURL normalizes a repo reference to a clone URL (HTTPS or git@)
func toCloneURL(url string) string {
	s := strings.TrimSpace(url)
	s = strings.TrimSuffix(s, ".git")
	if strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "git@") {
		return s + ".git"
	}
	if strings.HasPrefix(s, "github.com/") {
		return "https://" + s + ".git"
	}
	return s + ".git"
}

// importPathFromCloneURL extracts Go import path from clone URL (e.g. https://github.com/user/repo.git -> github.com/user/repo)
func importPathFromCloneURL(cloneURL string) string {
	s := strings.TrimSuffix(cloneURL, ".git")
	if idx := strings.Index(s, "github.com"); idx >= 0 {
		s = s[idx:]
		// git@github.com:user/repo -> github.com/user/repo
		s = strings.Replace(s, "github.com:", "github.com/", 1)
		return s
	}
	return s
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

	// Generate plugin-specific config template
	config := m.generatePluginConfig(plugin)

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// generatePluginConfig creates plugin-specific configuration templates
func (m *PluginManager) generatePluginConfig(plugin *PluginMetadata) map[string]interface{} {
	baseConfig := map[string]interface{}{
		"lynx": map[string]interface{}{
			plugin.Name: m.getPluginSpecificConfig(plugin),
		},
	}
	return baseConfig
}

// getPluginSpecificConfig returns plugin-specific configuration based on plugin type and name
func (m *PluginManager) getPluginSpecificConfig(plugin *PluginMetadata) map[string]interface{} {
	switch plugin.Type {
	case TypeService:
		return m.getServicePluginConfig(plugin.Name)
	case TypeMQ:
		return m.getMQPluginConfig(plugin.Name)
	case TypeSQL:
		return m.getSQLPluginConfig(plugin.Name)
	case TypeNoSQL:
		return m.getNoSQLPluginConfig(plugin.Name)
	case TypeTracer:
		return m.getTracerPluginConfig(plugin.Name)
	case TypeDTX:
		return m.getDTXPluginConfig(plugin.Name)
	default:
		return m.getGenericPluginConfig(plugin.Name)
	}
}

// getServicePluginConfig returns service plugin configuration templates
func (m *PluginManager) getServicePluginConfig(name string) map[string]interface{} {
	switch name {
	case "http":
		return map[string]interface{}{
			"network": "tcp",
			"addr":    ":8080",
			"timeout": "30s",
			"monitoring": map[string]interface{}{
				"enable_metrics":         true,
				"metrics_path":           "/metrics",
				"health_path":            "/health",
				"enable_request_logging": true,
			},
			"security": map[string]interface{}{
				"max_request_size": 10485760,
				"rate_limit": map[string]interface{}{
					"enabled":         true,
					"rate_per_second": 100,
					"burst_limit":     200,
				},
			},
		}
	case "grpc":
		return map[string]interface{}{
			"network": "tcp",
			"addr":    ":9090",
			"timeout": "30s",
			"client": map[string]interface{}{
				"default_timeout":    "10s",
				"default_keep_alive": "30s",
				"max_retries":        3,
				"connection_pooling": true,
				"subscribe_services": []map[string]interface{}{
					{
						"name":     "example-service",
						"timeout":  "5s",
						"required": false,
					},
				},
			},
		}
	default:
		return map[string]interface{}{
			"addr":    ":8080",
			"timeout": "30s",
			"# Note": "Configure service-specific settings here",
		}
	}
}

// getMQPluginConfig returns message queue plugin configuration templates
func (m *PluginManager) getMQPluginConfig(name string) map[string]interface{} {
	switch name {
	case "kafka":
		return map[string]interface{}{
			"brokers": []string{"localhost:9092"},
			"consumer": map[string]interface{}{
				"group_id":           "lynx-consumer-group",
				"auto_offset_reset":  "earliest",
				"enable_auto_commit": true,
			},
			"producer": map[string]interface{}{
				"acks":       "all",
				"retries":    3,
				"batch_size": 16384,
				"linger_ms":  1,
			},
		}
	case "rabbitmq":
		return map[string]interface{}{
			"url":           "amqp://guest:guest@localhost:5672/",
			"exchange":      "lynx-exchange",
			"exchange_type": "topic",
			"queue":         "lynx-queue",
			"routing_key":   "lynx.#",
			"durable":       true,
			"auto_delete":   false,
		}
	default:
		return map[string]interface{}{
			"brokers": []string{"localhost:9092"},
			"# Note":  "Configure message queue specific settings here",
		}
	}
}

// getSQLPluginConfig returns SQL database plugin configuration templates
func (m *PluginManager) getSQLPluginConfig(name string) map[string]interface{} {
	switch name {
	case "mysql":
		return map[string]interface{}{
			"dsn":               "user:password@tcp(localhost:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local",
			"max_open_conns":    25,
			"max_idle_conns":    10,
			"conn_max_lifetime": "5m",
			"enable_metrics":    true,
		}
	case "postgresql", "pgsql":
		return map[string]interface{}{
			"dsn":               "host=localhost user=username password=password dbname=mydb port=5432 sslmode=disable TimeZone=Asia/Shanghai",
			"max_open_conns":    25,
			"max_idle_conns":    10,
			"conn_max_lifetime": "5m",
			"enable_metrics":    true,
		}
	default:
		return map[string]interface{}{
			"dsn":            "user:password@tcp(localhost:3306)/dbname",
			"max_open_conns": 25,
			"max_idle_conns": 10,
			"# Note":         "Configure database specific connection settings here",
		}
	}
}

// getNoSQLPluginConfig returns NoSQL database plugin configuration templates
func (m *PluginManager) getNoSQLPluginConfig(name string) map[string]interface{} {
	switch name {
	case "redis":
		return map[string]interface{}{
			"addr":                "localhost:6379",
			"password":            "",
			"db":                  0,
			"pool_size":           10,
			"min_idle_conns":      5,
			"max_retries":         3,
			"enable_metrics":      true,
			"enable_health_check": true,
		}
	case "mongodb":
		return map[string]interface{}{
			"uri":            "mongodb://localhost:27017",
			"database":       "lynx_db",
			"max_pool_size":  100,
			"min_pool_size":  5,
			"max_idle_time":  "30s",
			"enable_metrics": true,
		}
	case "elasticsearch":
		return map[string]interface{}{
			"addresses":       []string{"http://localhost:9200"},
			"username":        "",
			"password":        "",
			"index_prefix":    "lynx",
			"enable_sniffing": false,
			"enable_metrics":  true,
		}
	default:
		return map[string]interface{}{
			"addr":     "localhost:6379",
			"database": "lynx_db",
			"# Note":   "Configure NoSQL database specific settings here",
		}
	}
}

// getTracerPluginConfig returns tracer plugin configuration templates
func (m *PluginManager) getTracerPluginConfig(name string) map[string]interface{} {
	return map[string]interface{}{
		"service_name":    "lynx-service",
		"service_version": "v1.0.0",
		"endpoint":        "http://localhost:14268/api/traces",
		"sampler": map[string]interface{}{
			"type":  "const",
			"param": 1,
		},
		"reporter": map[string]interface{}{
			"log_spans":             true,
			"buffer_flush_interval": "1s",
		},
	}
}

// getDTXPluginConfig returns distributed transaction plugin configuration templates
func (m *PluginManager) getDTXPluginConfig(name string) map[string]interface{} {
	switch name {
	case "seata":
		return map[string]interface{}{
			"application_id":                "lynx-app",
			"tx_service_group":              "lynx_tx_group",
			"enable_auto_data_source_proxy": true,
			"data_source_proxy_mode":        "AT",
			"registry": map[string]interface{}{
				"type": "nacos",
				"nacos": map[string]interface{}{
					"application": "seata-server",
					"server_addr": "127.0.0.1:8848",
					"group":       "SEATA_GROUP",
					"namespace":   "",
					"cluster":     "default",
				},
			},
		}
	case "dtm":
		return map[string]interface{}{
			"server":         "http://localhost:36789/api/dtmsvr",
			"timeout":        "30s",
			"retry_interval": "15s",
			"max_retry":      3,
		}
	default:
		return map[string]interface{}{
			"server":  "http://localhost:8080",
			"timeout": "30s",
			"# Note":  "Configure distributed transaction specific settings here",
		}
	}
}

// getGenericPluginConfig returns generic plugin configuration template
func (m *PluginManager) getGenericPluginConfig(name string) map[string]interface{} {
	switch name {
	case "polaris":
		return map[string]interface{}{
			"namespace":              "default",
			"server_addresses":       []string{"127.0.0.1:8091"},
			"enable_retry":           true,
			"max_retry_times":        3,
			"retry_interval":         "2s",
			"health_check_interval":  "5s",
		}
	case "swagger":
		return map[string]interface{}{
			"title":       "Lynx API Documentation",
			"description": "API documentation for Lynx service",
			"version":     "1.0.0",
			"host":        "localhost:8080",
			"base_path":   "/api/v1",
			"schemes":     []string{"http", "https"},
		}
	default:
		return map[string]interface{}{
			"enabled": true,
			"# Note":  fmt.Sprintf("Configure %s plugin specific settings here", name),
			"# Docs":  fmt.Sprintf("Refer to the plugin documentation for available configuration options"),
		}
	}
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
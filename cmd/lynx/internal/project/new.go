package project

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"gopkg.in/yaml.v3"

	"github.com/go-lynx/lynx/cmd/lynx/internal/base"
	"github.com/go-lynx/lynx/cmd/lynx/internal/plugin"
)

// Project represents a project template, containing project name and path information.
type Project struct {
	Name string // Project name
	Path string // Project path
}

type projectCreateResult struct {
	pluginFailures []string
	modTidyFailed  bool
}

const defaultLayoutRepo = "https://github.com/go-lynx/lynx-layout.git"

var builtInLayoutPlugins = map[string]struct{}{
	"grpc":       {},
	"http":       {},
	"mysql":      {},
	"redis":      {},
	"redis-lock": {},
	"tracer":     {},
}

// New creates a new project from a remote repository.
// preSelectedPlugins: when nil, run interactive plugin selection; when non-nil, use the slice (may be empty) and skip prompt.
func (p *Project) New(ctx context.Context, dir string, layout string, branch string, force bool, module string, postTidy bool, preSelectedPlugins *[]*plugin.PluginMetadata) error {
	// Calculate the complete path where the project will be created
	to := filepath.Join(dir, p.Name)

	// Check if target path already exists
	if _, err := os.Stat(to); !os.IsNotExist(err) {
		// If exists, notify user that path already exists
		base.Warnf("%s", base.T("already_exists", p.Name))
		// --force will silently overwrite, otherwise interactive confirmation
		if !force {
			prompt := &survey.Confirm{
				Message: base.T("override_confirm"),
				Help:    base.T("override_help"),
			}
			var override bool
			if e := survey.AskOne(prompt, &override); e != nil {
				return e
			}
			if !override {
				return fmt.Errorf("directory %s already exists and overwrite was declined", p.Name)
			}
		}
		if force {
			base.Warnf("--force: removing existing directory %s", to)
		}
		if e := os.RemoveAll(to); e != nil {
			return e
		}
	}

	// Notify user to start creating project and display project name and layout repository information
	base.Infof("%s", base.T("creating_service", p.Name, layout))
	// Create a new repository instance
	repo := base.NewRepo(layout, branch)
	// Copy remote repository content to target path, excluding .git and .github directories
	// If --module is provided, replace the module in template go.mod; otherwise don't replace template module
	if err := repo.CopyToV2(ctx, to, module, []string{".git", ".github"}, nil); err != nil {
		return err
	}
	if err := sanitizeProjectModules(to); err != nil {
		return err
	}
	// Rename the user directory under cmd directory to project name (layout must provide cmd/user)
	cmdUser := filepath.Join(to, "cmd", "user")
	if _, err := os.Stat(cmdUser); os.IsNotExist(err) {
		return fmt.Errorf("template layout must contain cmd/user directory (not found in %s); check repo branch or layout structure", to)
	}
	e := os.Rename(cmdUser, filepath.Join(to, "cmd", p.Name))
	if e != nil {
		return e
	}
	// Print project directory structure
	base.Tree(to, dir)

	// Resolve plugin list: interactive when preSelectedPlugins==nil, otherwise use pre-selected (or empty)
	result := projectCreateResult{}
	var selectedPlugins []*plugin.PluginMetadata
	if preSelectedPlugins != nil {
		selectedPlugins = *preSelectedPlugins
	} else {
		var err error
		selectedPlugins, err = selectPlugins()
		if err != nil {
			base.Warnf("Plugin selection failed: %v\n", err)
		}
	}
	if len(selectedPlugins) > 0 {
		// Add selected plugins to go.mod
		pluginResult, err := addPluginsToProject(ctx, to, selectedPlugins)
		result.pluginFailures = append(result.pluginFailures, pluginResult.pluginFailures...)
		if err != nil {
			base.Warnf("Failed to add plugin dependencies: %v\n", err)
		}
	}

	// Optional: Execute go mod tidy
	if postTidy {
		cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
		cmd.Dir = to
		if out, err := cmd.CombinedOutput(); err != nil {
			result.modTidyFailed = true
			base.Warnf("%s", base.T("mod_tidy_failed", err, string(out)))
		} else {
			base.Infof("%s", base.T("mod_tidy_ok"))
		}
	}

	// Notify user that project creation was successful
	printProjectSummary(p.Name, result)
	// Prompt user to use the following commands to start the project
	base.Infof("%s", base.T("start_cmds_header"))
	base.Infof("%s\n", color.WhiteString("$ cd %s", p.Name))
	if !postTidy {
		base.Infof("%s\n", color.WhiteString("$ go mod tidy"))
	}
	base.Infof("%s\n", color.WhiteString("$ go generate ./..."))
	base.Infof("%s\n", color.WhiteString("$ go build -o ./bin/ ./... "))
	base.Infof("%s\n", color.WhiteString("$ ./bin/%s -conf ./configs", p.Name))
	// Thank user for using Lynx and provide tutorial link
	base.Infof("%s", base.T("thanks"))
	base.Infof("%s", base.T("tutorial"))
	return nil
}

func printProjectSummary(name string, result projectCreateResult) {
	if len(result.pluginFailures) == 0 && !result.modTidyFailed {
		base.Infof("%s", base.T("project_success", color.GreenString(name)))
		return
	}

	base.Warnf("%s", base.T("project_partial", color.YellowString(name)))
	for _, failure := range result.pluginFailures {
		base.Warnf("%s\n", base.T("project_followup", failure))
	}
	if result.modTidyFailed {
		base.Warnf("%s\n", base.T("project_followup", "go mod tidy failed; inspect dependency integrity or module proxy state"))
	}
}

// selectPlugins prompts the user to select plugins interactively.
// Loads plugin list from GitHub (with cache). Returns nil, nil when no plugins available or user cancels.
func selectPlugins() ([]*plugin.PluginMetadata, error) {
	registry := plugin.NewPluginRegistry()
	// Load from GitHub; use empty projectRoot for "lynx new" (no project yet), still uses in-memory cache
	if err := registry.LoadFromGitHub(""); err != nil {
		base.Warnf("Load plugin list failed: %v (skip plugin selection)\n", err)
		return nil, nil
	}
	allPlugins := filterSelectablePlugins(registry.GetAllPlugins())
	if len(allPlugins) == 0 {
		return nil, nil
	}

	// Sort plugins by type and name for better display
	sort.Slice(allPlugins, func(i, j int) bool {
		if allPlugins[i].Type != allPlugins[j].Type {
			return allPlugins[i].Type < allPlugins[j].Type
		}
		return allPlugins[i].Name < allPlugins[j].Name
	})

	// Create options for multi-select
	options := make([]string, 0, len(allPlugins))
	pluginMap := make(map[string]*plugin.PluginMetadata)

	for _, p := range allPlugins {
		// Format: "Type: Name - Description"
		displayName := fmt.Sprintf("%s: %s - %s", formatPluginType(p.Type), p.Name, p.Description)
		options = append(options, displayName)
		pluginMap[displayName] = p
	}

	// Multi-select prompt
	prompt := &survey.MultiSelect{
		Message: base.T("select_plugins"),
		Help:    base.T("select_plugins_help"),
		Options: options,
	}

	var selectedOptions []string
	if err := survey.AskOne(prompt, &selectedOptions); err != nil {
		// User cancelled or interrupted - treat as no selection
		base.Infof("%s\n", base.T("no_plugins_selected"))
		return nil, nil
	}

	// Convert selected options to plugin metadata
	selectedPlugins := make([]*plugin.PluginMetadata, 0, len(selectedOptions))
	for _, opt := range selectedOptions {
		if p, ok := pluginMap[opt]; ok {
			selectedPlugins = append(selectedPlugins, p)
		}
	}

	return selectedPlugins, nil
}

// formatPluginType formats plugin type for display with i18n support
func formatPluginType(t plugin.PluginType) string {
	typeMap := map[plugin.PluginType]map[string]string{
		plugin.TypeService: {
			"zh": "服务",
			"en": "Service",
		},
		plugin.TypeMQ: {
			"zh": "消息队列",
			"en": "Message Queue",
		},
		plugin.TypeSQL: {
			"zh": "SQL数据库",
			"en": "SQL Database",
		},
		plugin.TypeNoSQL: {
			"zh": "NoSQL数据库",
			"en": "NoSQL Database",
		},
		plugin.TypeTracer: {
			"zh": "链路追踪",
			"en": "Tracing",
		},
		plugin.TypeDTX: {
			"zh": "分布式事务",
			"en": "Distributed Transaction",
		},
		plugin.TypeConfig: {
			"zh": "配置中心",
			"en": "Configuration",
		},
		plugin.TypeOther: {
			"zh": "其他",
			"en": "Other",
		},
	}

	lang := base.Lang()
	if typeNames, ok := typeMap[t]; ok {
		if name, ok := typeNames[lang]; ok {
			return name
		}
		// Fallback to Chinese if language not found
		if name, ok := typeNames["zh"]; ok {
			return name
		}
	}

	return string(t)
}

// ResolvePluginNames parses comma-separated plugin names and returns plugin metadata slice.
// Loads registry from GitHub. Unknown names are skipped with a warning.
func ResolvePluginNames(commaSeparated string) ([]*plugin.PluginMetadata, error) {
	commaSeparated = strings.TrimSpace(commaSeparated)
	if commaSeparated == "" {
		return nil, nil
	}
	registry := plugin.NewPluginRegistry()
	if err := registry.LoadFromGitHub(""); err != nil {
		return nil, err
	}
	names := strings.Split(commaSeparated, ",")
	var out []*plugin.PluginMetadata
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		p, err := registry.GetPlugin(name)
		if err != nil {
			base.Warnf("Plugin %q not found, skip: %v\n", name, err)
			continue
		}
		if shouldSkipBuiltInPlugin(p.Name) {
			base.Warnf("Plugin %q is already built into the default layout, skip explicit integration\n", name)
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func normalizeRepoURL(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	s = strings.TrimSuffix(s, "/")
	return s
}

func usingDefaultLayout() bool {
	current := repoURL
	if current == "" {
		current = defaultLayoutRepo
	}
	return normalizeRepoURL(current) == normalizeRepoURL(defaultLayoutRepo)
}

func shouldSkipBuiltInPlugin(name string) bool {
	if !usingDefaultLayout() {
		return false
	}
	_, ok := builtInLayoutPlugins[strings.TrimSpace(strings.ToLower(name))]
	return ok
}

func filterSelectablePlugins(all []*plugin.PluginMetadata) []*plugin.PluginMetadata {
	if !usingDefaultLayout() {
		return all
	}
	filtered := make([]*plugin.PluginMetadata, 0, len(all))
	for _, p := range all {
		if p == nil || shouldSkipBuiltInPlugin(p.Name) {
			continue
		}
		filtered = append(filtered, p)
	}
	return filtered
}

// getPluginModulePath returns the correct Go module path for a plugin
func getPluginModulePath(p *plugin.PluginMetadata) string {
	// Map plugin names to their actual module paths
	// The registry uses internal paths, but we need the actual published module paths
	modulePathMap := map[string]string{
		"http":          "github.com/go-lynx/lynx-http",
		"grpc":          "github.com/go-lynx/lynx-grpc",
		"openim":        "github.com/go-lynx/lynx-openim",
		"kafka":         "github.com/go-lynx/lynx-kafka",
		"rabbitmq":      "github.com/go-lynx/lynx-rabbitmq",
		"rocketmq":      "github.com/go-lynx/lynx-rocketmq",
		"pulsar":        "github.com/go-lynx/lynx-pulsar",
		"mysql":         "github.com/go-lynx/lynx-mysql",
		"postgresql":    "github.com/go-lynx/lynx-pgsql",
		"mssql":         "github.com/go-lynx/lynx-mssql",
		"redis":         "github.com/go-lynx/lynx-redis",
		"mongodb":       "github.com/go-lynx/lynx-mongodb",
		"elasticsearch": "github.com/go-lynx/lynx-elasticsearch",
		"seata":         "github.com/go-lynx/lynx-seata",
		"dtm":           "github.com/go-lynx/lynx-dtm",
		"apollo":        "github.com/go-lynx/lynx-apollo",
		"nacos":         "github.com/go-lynx/lynx-nacos",
		"polaris":       "github.com/go-lynx/lynx-polaris",
		"tracer":        "github.com/go-lynx/lynx-tracer",
		"swagger":       "github.com/go-lynx/lynx-swagger",
		"sentinel":      "github.com/go-lynx/lynx-sentinel",
		"snowflake":     "github.com/go-lynx/lynx-eon-id",
		"etcd":          "github.com/go-lynx/lynx-etcd",
	}

	// Check if we have a mapped path
	if mappedPath, ok := modulePathMap[p.Name]; ok {
		return mappedPath
	}

	// Fallback to ImportPath or Repository
	if p.ImportPath != "" {
		return p.ImportPath
	}
	if p.Repository != "" {
		return p.Repository
	}

	// Last resort: construct from name
	return fmt.Sprintf("github.com/go-lynx/lynx-%s", p.Name)
}

// addPluginsToProject adds selected plugins to the project's go.mod
func addPluginsToProject(ctx context.Context, projectDir string, plugins []*plugin.PluginMetadata) (projectCreateResult, error) {
	result := projectCreateResult{}
	if len(plugins) == 0 {
		return result, nil
	}

	base.Infof("%s\n", base.T("adding_plugins"))

	selected := make([]*plugin.PluginMetadata, 0, len(plugins))
	for _, p := range plugins {
		if p == nil {
			continue
		}

		// Get the correct module path for the plugin
		importPath := getPluginModulePath(p)

		if importPath == "" {
			base.Warnf("%s\n", base.T("plugin_add_failed", p.Name, "no import path"))
			result.pluginFailures = append(result.pluginFailures, fmt.Sprintf("plugin %s: no import path", p.Name))
			continue
		}
		selected = append(selected, p)

		// Add version if specified
		packagePath := importPath
		if p.Version != "" && p.Version != "latest" {
			// Use the version as-is (go get handles v prefix)
			packagePath = fmt.Sprintf("%s@%s", importPath, p.Version)
		} else {
			packagePath = fmt.Sprintf("%s@latest", importPath)
		}

		// Use go get to add the dependency
		cmd := exec.CommandContext(ctx, "go", "get", packagePath)
		cmd.Dir = projectDir
		if out, err := cmd.CombinedOutput(); err != nil {
			base.Warnf("%s\n", base.T("plugin_add_failed", p.Name, err))
			result.pluginFailures = append(result.pluginFailures, fmt.Sprintf("plugin %s: %v", p.Name, err))
			if len(out) > 0 {
				base.Warnf("Output: %s\n", string(out))
			}
		} else {
			base.Infof("%s\n", base.T("plugin_added", p.Name))
		}
	}

	if err := writePluginImportsFile(projectDir, selected); err != nil {
		return result, err
	}
	if err := mergePluginBootstrapConfig(projectDir, selected); err != nil {
		return result, err
	}

	return result, nil
}

func sanitizeProjectModules(projectDir string) error {
	for _, rel := range []string{"go.mod", filepath.Join("api", "go.mod")} {
		filename := filepath.Join(projectDir, rel)
		if _, err := os.Stat(filename); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := base.SanitizeGeneratedGoMod(filename); err != nil {
			return err
		}
	}
	return nil
}

func writePluginImportsFile(projectDir string, plugins []*plugin.PluginMetadata) error {
	if len(plugins) == 0 {
		return nil
	}

	mainPackageDir, err := findMainPackageDir(projectDir)
	if err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(plugins))
	imports := make([]string, 0, len(plugins))
	for _, p := range plugins {
		importPath := getPluginModulePath(p)
		if importPath == "" {
			continue
		}
		if _, ok := seen[importPath]; ok {
			continue
		}
		seen[importPath] = struct{}{}
		imports = append(imports, importPath)
	}
	if len(imports) == 0 {
		return nil
	}

	sort.Strings(imports)

	var b strings.Builder
	b.WriteString("// Code generated by lynx new. DO NOT EDIT.\n\n")
	b.WriteString("package main\n\n")
	b.WriteString("import (\n")
	for _, importPath := range imports {
		b.WriteString(fmt.Sprintf("\t_ %q\n", importPath))
	}
	b.WriteString(")\n")

	return os.WriteFile(filepath.Join(mainPackageDir, "plugins_gen.go"), []byte(b.String()), 0644)
}

func findMainPackageDir(projectDir string) (string, error) {
	cmdDir := filepath.Join(projectDir, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return "", fmt.Errorf("read cmd directory %s: %w", cmdDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		mainGo := filepath.Join(cmdDir, entry.Name(), "main.go")
		if _, err := os.Stat(mainGo); err == nil {
			return filepath.Join(cmdDir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("no cmd/<name>/main.go found under %s", cmdDir)
}

func mergePluginBootstrapConfig(projectDir string, plugins []*plugin.PluginMetadata) error {
	if len(plugins) == 0 {
		return nil
	}

	configPath := filepath.Join(projectDir, "configs", "bootstrap.local.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read bootstrap config %s: %w", configPath, err)
	}

	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse bootstrap config %s: %w", configPath, err)
	}
	if cfg == nil {
		cfg = make(map[string]any)
	}

	lynxSection := ensureStringMap(cfg["lynx"])
	cfg["lynx"] = lynxSection

	for _, p := range plugins {
		if p == nil {
			continue
		}
		key := strings.TrimSpace(p.Name)
		if key == "" {
			continue
		}
		if _, exists := lynxSection[key]; exists {
			continue
		}
		defaultConfig := plugin.DefaultConfigForPlugin(p)
		if len(defaultConfig) == 0 {
			continue
		}
		lynxSection[key] = defaultConfig
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal bootstrap config %s: %w", configPath, err)
	}
	return os.WriteFile(configPath, out, 0644)
}

func ensureStringMap(v any) map[string]any {
	switch typed := v.(type) {
	case map[string]any:
		return typed
	default:
		return make(map[string]any)
	}
}

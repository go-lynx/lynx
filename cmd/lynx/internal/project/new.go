package project

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"

	"github.com/go-lynx/lynx/cmd/lynx/internal/base"
	"github.com/go-lynx/lynx/cmd/lynx/internal/plugin"
)

// Project represents a project template, containing project name and path information.
type Project struct {
	Name string // Project name
	Path string // Project path
}

// New creates a new project from a remote repository.
// ctx: Context for controlling the lifecycle of the operation.
// dir: Target directory for project creation.
// layout: Remote repository address for project layout.
// branch: Remote repository branch to use.
// force: Whether to force overwrite existing project directory.
// module: If provided, replaces the module in template go.mod.
// postTidy: Whether to execute go mod tidy command.
// Returns: Returns corresponding error information if an error occurs during operation; otherwise returns nil.
func (p *Project) New(ctx context.Context, dir string, layout string, branch string, force bool, module string, postTidy bool) error {
	// Calculate the complete path where the project will be created
	to := filepath.Join(dir, p.Name)

	// Check if target path already exists
	if _, err := os.Stat(to); !os.IsNotExist(err) {
		// If exists, notify user that path already exists
		base.Warnf("%s", fmt.Sprintf(base.T("already_exists"), p.Name))
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
				return err
			}
		}
		if e := os.RemoveAll(to); e != nil {
			return e
		}
	}

	// Notify user to start creating project and display project name and layout repository information
	base.Infof("%s", fmt.Sprintf(base.T("creating_service"), p.Name, layout))
	// Create a new repository instance
	repo := base.NewRepo(layout, branch)
	// Copy remote repository content to target path, excluding .git and .github directories
	// If --module is provided, replace the module in template go.mod; otherwise don't replace template module
	if err := repo.CopyToV2(ctx, to, module, []string{".git", ".github"}, nil); err != nil {
		return err
	}
	// Rename the user directory under cmd directory to project name
	e := os.Rename(
		filepath.Join(to, "cmd", "user"),
		filepath.Join(to, "cmd", p.Name),
	)
	if e != nil {
		return e
	}
	// Print project directory structure
	base.Tree(to, dir)

	// Ask user to select plugins
	selectedPlugins, err := selectPlugins()
	if err != nil {
		base.Warnf("Plugin selection failed: %v\n", err)
	} else if len(selectedPlugins) > 0 {
		// Add selected plugins to go.mod
		if err := addPluginsToProject(ctx, to, selectedPlugins); err != nil {
			base.Warnf("Failed to add plugin dependencies: %v\n", err)
		}
	}

	// Optional: Execute go mod tidy
	if postTidy {
		cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
		cmd.Dir = to
		if out, err := cmd.CombinedOutput(); err != nil {
			base.Warnf("%s", fmt.Sprintf(base.T("mod_tidy_failed"), err, string(out)))
		} else {
			base.Infof("%s", base.T("mod_tidy_ok"))
		}
	}

	// Notify user that project creation was successful
	base.Infof("%s", fmt.Sprintf(base.T("project_success"), color.GreenString(p.Name)))
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

// selectPlugins prompts the user to select plugins interactively
func selectPlugins() ([]*plugin.PluginMetadata, error) {
	// Create plugin registry to get available plugins
	registry := plugin.NewPluginRegistry()
	allPlugins := registry.GetAllPlugins()

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
func addPluginsToProject(ctx context.Context, projectDir string, plugins []*plugin.PluginMetadata) error {
	if len(plugins) == 0 {
		return nil
	}

	base.Infof("%s\n", base.T("adding_plugins"))

	for _, p := range plugins {
		// Get the correct module path for the plugin
		importPath := getPluginModulePath(p)

		if importPath == "" {
			base.Warnf("%s\n", fmt.Sprintf(base.T("plugin_add_failed"), p.Name, "no import path"))
			continue
		}

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
			base.Warnf("%s\n", fmt.Sprintf(base.T("plugin_add_failed"), p.Name, err))
			if len(out) > 0 {
				base.Warnf("Output: %s\n", string(out))
			}
		} else {
			base.Infof("%s\n", fmt.Sprintf(base.T("plugin_added"), p.Name))
		}
	}

	return nil
}

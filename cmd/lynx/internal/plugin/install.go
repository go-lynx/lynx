package plugin

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	installVersion string
	installForce   bool
)

// cmdInstall represents the install command
var cmdInstall = &cobra.Command{
	Use:   "install [plugin-name]",
	Short: "Install a plugin",
	Long: `Install a plugin from the official registry or a custom repository.
You can specify a version or use the latest version by default.`,
	Example: `  # Install plugin (uses default branch; run from project root)
  lynx plugin install redis
  
  # Install specific tag/branch
  lynx plugin install redis --version v2.0.0
  
  # Force reinstall
  lynx plugin install redis --force
  
  # Install from GitHub repository
  lynx plugin install github.com/user/custom-plugin
  
  # Install from local path
  lynx plugin install ./my-plugin`,
	Args: cobra.ExactArgs(1),
	RunE: runInstall,
}

func init() {
	// No -v shorthand: -v means --verbose elsewhere; keep --version long-form only
	// to avoid an ambiguous shorthand across subcommands.
	cmdInstall.Flags().StringVar(&installVersion, "version", "latest", "Plugin version to install")
	cmdInstall.Flags().BoolVarP(&installForce, "force", "f", false, "Force reinstall even if already installed")
}

func runInstall(cmd *cobra.Command, args []string) error {
	pluginName := args[0]

	// Create plugin manager
	manager, err := NewPluginManager()
	if err != nil {
		return fmt.Errorf("failed to initialize plugin manager: %w", err)
	}

	// Show installation progress
	fmt.Printf("🔍 Looking for plugin: %s\n", color.CyanString(pluginName))

	// Resolve metadata up-front (best effort) so we can print actionable steps later.
	var meta *PluginMetadata
	if strings.Contains(pluginName, "/") || strings.HasPrefix(pluginName, ".") {
		fmt.Printf("📦 Installing from: %s\n", pluginName)
	} else if m, infoErr := manager.GetPluginInfo(pluginName); infoErr == nil {
		meta = m
		fmt.Printf("📋 Found: %s (%s)\n", meta.Name, meta.Description)
		if meta.Official {
			fmt.Printf("✓ Official plugin by %s\n", meta.Author)
		}

		// Show dependencies if any
		if len(meta.Dependencies) > 0 {
			fmt.Println("📌 Dependencies:")
			for _, dep := range meta.Dependencies {
				status := "optional"
				if dep.Required {
					status = "required"
				}
				fmt.Printf("   - %s %s (%s)\n", dep.Name, dep.Version, status)
			}
		}
	}

	// Install the plugin
	if err := manager.InstallPlugin(pluginName, installVersion, installForce); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	printInstallNextSteps(pluginName, meta)
	return nil
}

// printInstallNextSteps prints concrete, copy-pasteable next steps, using the
// plugin's metadata (import path, config file) when available.
func printInstallNextSteps(pluginName string, meta *PluginMetadata) {
	fmt.Printf("\n%s plugin %s installed\n", color.GreenString("✓"), color.CyanString(pluginName))
	fmt.Println("\n📚 Next steps:")

	// 1) Blank import that registers the plugin with the framework.
	if meta != nil && meta.ImportPath != "" {
		fmt.Println("  1. Register it with a blank import in your service bootstrap (e.g. cmd/<svc>/main.go):")
		fmt.Printf("       import _ %q\n", meta.ImportPath)
	} else {
		fmt.Println("  1. Add a blank import for the plugin package in your service bootstrap (cmd/<svc>/main.go)")
	}

	// 2) Where to configure it.
	configFile := "configs/config.yaml"
	if meta != nil && meta.ConfigFile != "" {
		configFile = meta.ConfigFile
	}
	fmt.Printf("  2. Add the plugin's configuration section in %s\n", color.YellowString(configFile))

	// 3) Verify.
	fmt.Printf("  3. Verify the setup: %s\n", color.CyanString("lynx doctor"))
}

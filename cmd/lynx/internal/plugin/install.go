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
	Example: `  # Install latest version of a plugin
  lynx plugin install redis
  
  # Install specific version
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
	cmdInstall.Flags().StringVarP(&installVersion, "version", "v", "latest", "Plugin version to install")
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

	// Check if it's a URL or path
	if strings.Contains(pluginName, "/") || strings.HasPrefix(pluginName, ".") {
		fmt.Printf("📦 Installing from: %s\n", pluginName)
	} else {
		// Try to get plugin info first
		if plugin, err := manager.GetPluginInfo(pluginName); err == nil {
			fmt.Printf("📋 Found: %s (%s)\n", plugin.Name, plugin.Description)
			if plugin.Official {
				fmt.Printf("✓ Official plugin by %s\n", plugin.Author)
			}
			
			// Show dependencies if any
			if len(plugin.Dependencies) > 0 {
				fmt.Println("📌 Dependencies:")
				for _, dep := range plugin.Dependencies {
					status := "optional"
					if dep.Required {
						status = "required"
					}
					fmt.Printf("   - %s %s (%s)\n", dep.Name, dep.Version, status)
				}
			}
		}
	}

	// Install the plugin
	if err := manager.InstallPlugin(pluginName, installVersion, installForce); err != nil {
		return fmt.Errorf("❌ Installation failed: %w", err)
	}

	// Show post-installation instructions
	fmt.Println("\n📚 Next steps:")
	fmt.Printf("1. Configure the plugin in the generated config file\n")
	fmt.Printf("2. Import and initialize the plugin in your code\n")
	fmt.Printf("3. Run 'lynx doctor' to verify the installation\n")

	return nil
}
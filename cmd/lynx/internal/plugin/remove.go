package plugin

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	removeKeepConfig bool
	removeForce      bool
)

// cmdRemove represents the remove command
var cmdRemove = &cobra.Command{
	Use:   "remove [plugin-name]",
	Short: "Remove an installed plugin",
	Long: `Remove an installed plugin from your project.
By default, it will ask for confirmation before removing.`,
	Example: `  # Remove a plugin
  lynx plugin remove redis
  
  # Keep configuration file
  lynx plugin remove redis --keep-config
  
  # Force remove without confirmation
  lynx plugin remove redis --force`,
	Aliases: []string{"uninstall", "rm"},
	Args:    cobra.ExactArgs(1),
	RunE:    runRemove,
}

func init() {
	cmdRemove.Flags().BoolVar(&removeKeepConfig, "keep-config", false, "Keep plugin configuration file")
	cmdRemove.Flags().BoolVarP(&removeForce, "force", "f", false, "Force remove without confirmation")
}

func runRemove(cmd *cobra.Command, args []string) error {
	pluginName := args[0]

	// Create plugin manager
	manager, err := NewPluginManager()
	if err != nil {
		return fmt.Errorf("failed to initialize plugin manager: %w", err)
	}

	// Get plugin info
	plugin, err := manager.GetPluginInfo(pluginName)
	if err != nil {
		return fmt.Errorf("plugin not found: %s", pluginName)
	}

	if plugin.Status != StatusInstalled {
		return fmt.Errorf("plugin %s is not installed", pluginName)
	}

	// Show plugin info
	fmt.Printf("📦 Plugin: %s\n", color.CyanString(plugin.Name))
	fmt.Printf("   Type: %s\n", plugin.Type)
	fmt.Printf("   Version: %s\n", plugin.InstalledVer)
	
	// Confirmation
	if !removeForce {
		fmt.Printf("\n⚠️  %s\n", color.YellowString("This will remove the plugin and its files."))
		if !removeKeepConfig {
			fmt.Printf("   Configuration file will also be removed.\n")
		} else {
			fmt.Printf("   Configuration file will be kept.\n")
		}
		
		fmt.Printf("\nAre you sure you want to remove %s? (y/N): ", pluginName)
		
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("❌ Removal cancelled")
			return nil
		}
	}

	// Remove the plugin
	fmt.Printf("\n🗑️  Removing plugin: %s\n", pluginName)
	if err := manager.RemovePlugin(pluginName, removeKeepConfig); err != nil {
		return fmt.Errorf("❌ Removal failed: %w", err)
	}

	// Show success message
	if removeKeepConfig {
		fmt.Printf("\n✅ Plugin %s removed successfully (configuration kept)\n", color.GreenString(pluginName))
		fmt.Printf("   Configuration file can be manually removed from conf/%s.yaml\n", pluginName)
	} else {
		fmt.Printf("\n✅ Plugin %s removed successfully\n", color.GreenString(pluginName))
	}

	return nil
}
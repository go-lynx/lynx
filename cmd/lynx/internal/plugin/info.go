package plugin

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var infoFormat string

var cmdInfo = &cobra.Command{
	Use:   "info [plugin-name]",
	Short: "Show detailed information about a plugin",
	Long:  `Display detailed information about a specific plugin, including its description, version, dependencies, and installation status.`,
	Example: `  # Show plugin information
  lynx plugin info redis
  
  # Output in JSON format
  lynx plugin info redis --format json
  
  # Output in YAML format
  lynx plugin info redis --format yaml`,
	Aliases: []string{"show", "describe"},
	Args:    cobra.ExactArgs(1),
	RunE:    runInfo,
}

func init() {
	cmdInfo.Flags().StringVarP(&infoFormat, "format", "f", "text", "Output format (text/json/yaml)")
}

func runInfo(cmd *cobra.Command, args []string) error {
	pluginName := args[0]

	// info is a discovery command; use the read-only manager so it works outside
	// a Go project directory too (installed status will show "Not installed" in that case).
	manager, err := NewReadOnlyPluginManager()
	if err != nil {
		return fmt.Errorf("failed to initialize plugin manager: %w", err)
	}

	plugin, err := manager.GetPluginInfo(pluginName)
	if err != nil {
		return fmt.Errorf("plugin not found: %s", pluginName)
	}

	switch infoFormat {
	case "json":
		return exportJSON(cmd.OutOrStdout(), plugin)
	case "yaml":
		return exportYAML(cmd.OutOrStdout(), plugin)
	default:
		return displayPluginInfo(plugin)
	}
}

func displayPluginInfo(plugin *PluginMetadata) error {
	fmt.Printf("\n%s Plugin Information\n", color.CyanString("📦"))
	fmt.Println(strings.Repeat("=", 50))

	fmt.Printf("\n%s\n", color.YellowString("Basic Information:"))
	fmt.Printf("  Name:        %s", color.CyanString(plugin.Name))
	if plugin.Official {
		fmt.Printf(" %s\n", color.YellowString("✓ Official"))
	} else {
		fmt.Println()
	}
	fmt.Printf("  Type:        %s\n", formatPluginType(plugin.Type))
	fmt.Printf("  Version:     %s\n", plugin.Version)
	fmt.Printf("  Author:      %s\n", plugin.Author)
	fmt.Printf("  License:     %s\n", plugin.License)

	fmt.Printf("\n%s\n", color.YellowString("Status:"))
	switch plugin.Status {
	case StatusInstalled:
		fmt.Printf("  Installation: %s\n", color.GreenString("Installed"))
		if plugin.InstalledVer != "" && plugin.InstalledVer != "unknown" {
			fmt.Printf("  Installed Version: %s\n", plugin.InstalledVer)
			if plugin.InstalledVer != plugin.Version {
				fmt.Printf("  %s\n", color.YellowString("⚠ Update available"))
			}
		}
		if plugin.Enabled {
			fmt.Printf("  State: %s\n", color.GreenString("Enabled"))
		} else {
			fmt.Printf("  State: %s\n", color.RedString("Disabled"))
		}
		if plugin.ConfigFile != "" {
			fmt.Printf("  Config File: %s\n", plugin.ConfigFile)
		}
	case StatusNotInstalled:
		fmt.Printf("  Installation: %s\n", color.YellowString("Not Installed"))
		fmt.Printf("  %s\n", color.HiBlackString("  Use 'lynx plugin install %s' to install", plugin.Name))
	default:
		fmt.Printf("  Installation: %s\n", plugin.Status)
	}

	fmt.Printf("\n%s\n", color.YellowString("Description:"))
	fmt.Printf("  %s\n", plugin.Description)

	if plugin.Repository != "" {
		fmt.Printf("\n%s\n", color.YellowString("Repository:"))
		fmt.Printf("  %s\n", plugin.Repository)
	}

	if plugin.ImportPath != "" && plugin.ImportPath != plugin.Repository {
		fmt.Printf("\n%s\n", color.YellowString("Import Path:"))
		fmt.Printf("  %s\n", plugin.ImportPath)
	}

	if len(plugin.Dependencies) > 0 {
		fmt.Printf("\n%s\n", color.YellowString("Dependencies:"))
		for _, dep := range plugin.Dependencies {
			status := color.HiBlackString("(optional)")
			if dep.Required {
				status = color.RedString("(required)")
			}
			fmt.Printf("  • %s %s %s\n", dep.Name, dep.Version, status)
		}
	}

	if len(plugin.Tags) > 0 {
		fmt.Printf("\n%s\n", color.YellowString("Tags:"))
		fmt.Printf("  %s\n", strings.Join(plugin.Tags, ", "))
	}

	if plugin.Compatible != "" {
		fmt.Printf("\n%s\n", color.YellowString("Compatibility:"))
		fmt.Printf("  Lynx Version: %s\n", plugin.Compatible)
	}

	if len(plugin.ExtraInfo) > 0 {
		fmt.Printf("\n%s\n", color.YellowString("Additional Information:"))
		for key, value := range plugin.ExtraInfo {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}

	fmt.Printf("\n%s\n", color.YellowString("Available Commands:"))
	if plugin.Status == StatusInstalled {
		fmt.Printf("  • Update:  lynx plugin install %s --force\n", plugin.Name)
		fmt.Printf("  • Remove:  lynx plugin remove %s\n", plugin.Name)
		fmt.Printf("  • Config:  Edit conf/%s.yaml\n", plugin.Name)
	} else {
		fmt.Printf("  • Install: lynx plugin install %s\n", plugin.Name)
		fmt.Printf("  • Install specific version: lynx plugin install %s --version %s\n", plugin.Name, plugin.Version)
	}

	fmt.Println()
	return nil
}

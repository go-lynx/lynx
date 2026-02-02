package plugin

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	listAll     bool
	listType    string
	listFormat  string
	listNoCache bool
)

// cmdList represents the list command
var cmdList = &cobra.Command{
	Use:   "list",
	Short: "List available or installed plugins",
	Long: `List plugins that are available for installation or already installed in your project.
By default, it shows only installed plugins. Use --all to see all available plugins.`,
	Example: `  # List installed plugins
  lynx plugin list
  
  # List all available plugins
  lynx plugin list --all
  
  # List plugins by type
  lynx plugin list --type service
  lynx plugin list --type mq
  
  # List in different formats
  lynx plugin list --format json
  lynx plugin list --format yaml`,
	RunE: runList,
}

func init() {
	cmdList.Flags().BoolVarP(&listAll, "all", "a", false, "Show all available plugins")
	cmdList.Flags().StringVarP(&listType, "type", "t", "", "Filter by plugin type (service/mq/sql/nosql/tracer/dtx/other)")
	cmdList.Flags().StringVarP(&listFormat, "format", "f", "table", "Output format (table/json/yaml)")
	cmdList.Flags().BoolVar(&listNoCache, "no-cache", false, "Force refresh plugin list from GitHub (ignore cache)")
}

func runList(cmd *cobra.Command, args []string) error {
	if listNoCache {
		InvalidatePluginCache()
		SetForceRefreshPluginList(true)
	}
	manager, err := NewPluginManager()
	if err != nil {
		return fmt.Errorf("failed to initialize plugin manager: %w", err)
	}

	// Get plugins
	plugins, err := manager.ListPlugins(listAll, listType)
	if err != nil {
		return fmt.Errorf("failed to list plugins: %w", err)
	}

	// Sort plugins by type and name
	sort.Slice(plugins, func(i, j int) bool {
		if plugins[i].Type != plugins[j].Type {
			return plugins[i].Type < plugins[j].Type
		}
		return plugins[i].Name < plugins[j].Name
	})

	// When listing all available plugins, fetch latest version from Go proxy for each
	if listAll && len(plugins) > 0 {
		EnrichPluginsLatestVersion(plugins)
	}

	// Output based on format
	switch listFormat {
	case "json":
		return outputJSON(plugins)
	case "yaml":
		return outputYAML(plugins)
	default:
		return outputTable(plugins, listAll)
	}
}

func outputTable(plugins []*PluginMetadata, showAll bool) error {
	if len(plugins) == 0 {
		if showAll {
			fmt.Println("No plugins available.")
		} else {
			fmt.Println("No plugins installed. Use 'lynx plugin list --all' to see available plugins.")
		}
		return nil
	}

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	
	// Print header
	if showAll {
		fmt.Fprintf(w, "NAME\tTYPE\tVERSION\tSTATUS\tDESCRIPTION\n")
		fmt.Fprintf(w, "----\t----\t-------\t------\t-----------\n")
	} else {
		fmt.Fprintf(w, "NAME\tTYPE\tVERSION\tENABLED\tDESCRIPTION\n")
		fmt.Fprintf(w, "----\t----\t-------\t-------\t-----------\n")
	}

	// Print plugins
	for _, plugin := range plugins {
		name := plugin.Name
		if plugin.Official {
			name = color.CyanString(name) + " " + color.YellowString("✓")
		}

		typeStr := colorizeType(plugin.Type)
		
		// Truncate description if too long
		desc := plugin.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		if showAll {
			status := colorizeStatus(plugin.Status)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				name, typeStr, plugin.Version, status, desc)
		} else {
			enabled := "Yes"
			if !plugin.Enabled {
				enabled = color.RedString("No")
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				name, typeStr, plugin.InstalledVer, enabled, desc)
		}
	}

	w.Flush()

	// Print summary
	fmt.Println()
	if showAll {
		installedCount := 0
		for _, p := range plugins {
			if p.Status == StatusInstalled {
				installedCount++
			}
		}
		fmt.Printf("Total: %d plugins (%d installed)\n", len(plugins), installedCount)
		fmt.Println("\n✓ = Official plugin")
	} else {
		fmt.Printf("Total: %d installed plugins\n", len(plugins))
	}

	return nil
}

func outputJSON(plugins []*PluginMetadata) error {
	return exportJSON(os.Stdout, plugins)
}

func outputYAML(plugins []*PluginMetadata) error {
	return exportYAML(os.Stdout, plugins)
}

func colorizeType(t PluginType) string {
	switch t {
	case TypeService:
		return color.BlueString(string(t))
	case TypeMQ:
		return color.MagentaString(string(t))
	case TypeSQL:
		return color.GreenString(string(t))
	case TypeNoSQL:
		return color.YellowString(string(t))
	case TypeTracer:
		return color.CyanString(string(t))
	case TypeDTX:
		return color.RedString(string(t))
	default:
		return string(t)
	}
}

func colorizeStatus(s PluginStatus) string {
	switch s {
	case StatusInstalled:
		return color.GreenString("Installed")
	case StatusNotInstalled:
		return color.YellowString("Available")
	case StatusUpdatable:
		return color.BlueString("Updatable")
	default:
		return string(s)
	}
}

// Helper function to format plugin type display
func formatPluginType(pluginType PluginType) string {
	switch pluginType {
	case TypeService:
		return "Service"
	case TypeMQ:
		return "Message Queue"
	case TypeSQL:
		return "SQL Database"
	case TypeNoSQL:
		return "NoSQL Database"
	case TypeTracer:
		return "Tracing"
	case TypeDTX:
		return "Distributed Transaction"
	case TypeConfig:
		return "Configuration"
	case TypeOther:
		return "Other"
	default:
		return strings.Title(string(pluginType))
	}
}
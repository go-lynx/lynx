package plugin

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var searchFormat string

// cmdSearch represents the search command
var cmdSearch = &cobra.Command{
	Use:   "search [keyword]",
	Short: "Search for plugins",
	Long:  `Search for plugins by name, description, or tags.`,
	Example: `  # Search for redis related plugins
  lynx plugin search redis
  
  # Search for database plugins
  lynx plugin search database
  
  # Search for message queue plugins
  lynx plugin search mq`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	cmdSearch.Flags().StringVarP(&searchFormat, "format", "f", "table", "Output format (table/json/yaml)")
}

func runSearch(cmd *cobra.Command, args []string) error {
	keyword := args[0]

	// Create plugin manager
	manager, err := NewPluginManager()
	if err != nil {
		return fmt.Errorf("failed to initialize plugin manager: %w", err)
	}

	// Search plugins
	plugins := manager.SearchPlugins(keyword)

	if len(plugins) == 0 {
		fmt.Printf("No plugins found matching '%s'\n", keyword)
		fmt.Println("\nTry searching with different keywords or use 'lynx plugin list --all' to see all available plugins.")
		return nil
	}

	// Output based on format
	switch searchFormat {
	case "json":
		return exportJSON(cmd.OutOrStdout(), plugins)
	case "yaml":
		return exportYAML(cmd.OutOrStdout(), plugins)
	default:
		return displaySearchResults(plugins, keyword)
	}
}

func displaySearchResults(plugins []*PluginMetadata, keyword string) error {
	fmt.Printf("\n🔍 Search results for '%s' (%d found):\n\n", color.CyanString(keyword), len(plugins))

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	
	// Print header
	fmt.Fprintf(w, "NAME\tTYPE\tVERSION\tSTATUS\tDESCRIPTION\n")
	fmt.Fprintf(w, "----\t----\t-------\t------\t-----------\n")

	// Print plugins
	for _, plugin := range plugins {
		name := plugin.Name
		// Highlight matching keyword
		if strings.Contains(strings.ToLower(name), strings.ToLower(keyword)) {
			name = color.YellowString(name)
		}
		if plugin.Official {
			name = name + " " + color.CyanString("✓")
		}

		typeStr := colorizeType(plugin.Type)
		status := colorizeStatus(plugin.Status)
		
		// Truncate and highlight description
		desc := plugin.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		// Highlight keyword in description
		if strings.Contains(strings.ToLower(desc), strings.ToLower(keyword)) {
			desc = strings.ReplaceAll(desc, keyword, color.YellowString(keyword))
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			name, typeStr, plugin.Version, status, desc)
	}

	w.Flush()

	// Print legend
	fmt.Println("\n✓ = Official plugin")
	fmt.Printf("\nUse 'lynx plugin info <name>' for more details about a specific plugin.\n")
	fmt.Printf("Use 'lynx plugin install <name>' to install a plugin.\n")

	return nil
}
package plugin

import (
	"github.com/spf13/cobra"
)

// CmdPlugin represents the plugin management command
var CmdPlugin = &cobra.Command{
	Use:   "plugin",
	Short: "Manage Lynx plugins",
	Long: `The plugin command manages Lynx plugins for your project.
Plugin list is loaded from GitHub (go-lynx organization). Run from a project root (directory with go.mod).
Commands: list, search, install, remove, info.`,
	Example: `  # List all available plugins
  lynx plugin list --all
  
  # Search for plugins
  lynx plugin search redis
  
  # Install a plugin
  lynx plugin install redis
  
  # Remove a plugin
  lynx plugin remove redis
  
  # Get plugin information
  lynx plugin info redis`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand is provided, show help
		return cmd.Help()
	},
}

func init() {
	// Add subcommands
	CmdPlugin.AddCommand(cmdList)
	CmdPlugin.AddCommand(cmdInstall)
	CmdPlugin.AddCommand(cmdRemove)
	CmdPlugin.AddCommand(cmdInfo)
	CmdPlugin.AddCommand(cmdSearch)
}
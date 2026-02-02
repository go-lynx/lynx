package git

import (
	"github.com/spf13/cobra"
)

// CmdGit represents the git-related command group
var CmdGit = &cobra.Command{
	Use:   "git",
	Short: "Git operations for Lynx organization",
	Long:  `Git-related commands for the go-lynx organization on GitHub.`,
	Example: `  # Clone all public repositories from go-lynx
  lynx git clone-all`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	CmdGit.AddCommand(cmdCloneAll)
}

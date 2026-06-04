package main

import (
	"fmt"
	"os"

	"github.com/go-lynx/lynx/cmd/lynx/internal/ca"
	"github.com/go-lynx/lynx/cmd/lynx/internal/doctor"
	"github.com/go-lynx/lynx/cmd/lynx/internal/git"
	"github.com/go-lynx/lynx/cmd/lynx/internal/plugin"
	"github.com/go-lynx/lynx/cmd/lynx/internal/project"
	"github.com/go-lynx/lynx/cmd/lynx/internal/run"
	"github.com/spf13/cobra"
)

// rootCmd is the top-level lynx command; subcommands are registered in init.
var rootCmd = &cobra.Command{
	Use:     "lynx",
	Short:   "Lynx: The Plug-and-Play Go Microservices Framework",
	Long:    `Lynx: The Plug-and-Play Go Microservices Framework`,
	Version: release,
	// On a RunE error, print only our own clean message (see main); do not let
	// cobra dump the full usage text or print the error a second time.
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Flags are translated into env vars so subcommands and spawned
		// executors (which don't share the cobra flag set) can read them.
		verbose, _ := cmd.Flags().GetBool("verbose")
		quiet, _ := cmd.Flags().GetBool("quiet")
		logLevel, _ := cmd.Flags().GetString("log-level")
		lang, _ := cmd.Flags().GetString("lang")

		// Language environment
		if lang != "" {
			_ = os.Setenv("LYNX_LANG", lang)
		}

		// Log level priority: --log-level > --quiet/--verbose > default
		if logLevel != "" {
			_ = os.Setenv("LYNX_LOG_LEVEL", logLevel)
		} else if quiet {
			_ = os.Setenv("LYNX_LOG_LEVEL", "error")
			_ = os.Setenv("LYNX_QUIET", "1")
		} else if verbose {
			_ = os.Setenv("LYNX_LOG_LEVEL", "debug")
			_ = os.Setenv("LYNX_VERBOSE", "1")
		} else {
			if os.Getenv("LYNX_LOG_LEVEL") == "" {
				_ = os.Setenv("LYNX_LOG_LEVEL", "info")
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(project.CmdNew)
	rootCmd.AddCommand(doctor.CmdDoctor)
	rootCmd.AddCommand(git.CmdGit)
	rootCmd.AddCommand(plugin.CmdPlugin)
	rootCmd.AddCommand(run.CmdRun)
	rootCmd.AddCommand(ca.CmdGenCA)
	rootCmd.PersistentFlags().Bool("verbose", false, "enable verbose logs")
	rootCmd.PersistentFlags().Bool("quiet", false, "suppress non-error logs")
	rootCmd.PersistentFlags().String("log-level", "info", "log level: error|warn|info|debug (overrides --quiet/--verbose)")
	// Empty default so base.Lang() remains the single source of truth for the
	// default language (avoids a flag-default vs i18n-default mismatch).
	rootCmd.PersistentFlags().String("lang", "", "language for messages: zh|en (default en)")
}

func main() {
	// On error print a single clean line to stderr (no log timestamp, no usage
	// dump, per SilenceUsage/SilenceErrors above) and exit non-zero.
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "lynx: "+err.Error())
		os.Exit(1)
	}
}

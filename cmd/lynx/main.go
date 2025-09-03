package main

import (
	"log"
	"os"

	"github.com/go-lynx/lynx/cmd/lynx/internal/doctor"
	"github.com/go-lynx/lynx/cmd/lynx/internal/project"
	"github.com/spf13/cobra"
)

// rootCmd is the root command of Lynx CLI tool, defining basic information and version of the tool.
var rootCmd = &cobra.Command{
	// Use defines the usage of the command
	Use: "lynx",
	// Short is the short description of the command
	Short: "Lynx: The Plug-and-Play Go Microservices Framework",
	// Long is the detailed description of the command
	Long: `Lynx: The Plug-and-Play Go Microservices Framework`,
	// Version defines the version of the CLI tool, release variable needs to be defined elsewhere
	Version: release,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Persist log level to environment variables for internal subcommands and executors to read
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
			// Default info
			if os.Getenv("LYNX_LOG_LEVEL") == "" {
				_ = os.Setenv("LYNX_LOG_LEVEL", "info")
			}
		}
	},
}

// init function is the package initialization function, automatically executed when the package is loaded.
func init() {
	// Add subcommands to root command
	rootCmd.AddCommand(project.CmdNew)
	rootCmd.AddCommand(doctor.CmdDoctor)
	// Global log level flags
	rootCmd.PersistentFlags().Bool("verbose", false, "enable verbose logs")
	rootCmd.PersistentFlags().Bool("quiet", false, "suppress non-error logs")
	rootCmd.PersistentFlags().String("log-level", "info", "log level: error|warn|info|debug (overrides --quiet/--verbose)")
	rootCmd.PersistentFlags().String("lang", "zh", "language for messages: zh|en")
}

// main function is the entry point of the program, responsible for executing the root command.
func main() {
	// Execute root command, if error occurs during execution, log error and terminate program
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

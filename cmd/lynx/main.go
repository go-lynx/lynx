package main

import (
	"github.com/go-lynx/lynx/cmd/lynx/internal/project"
	"log"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "lynx",
	Short:   "Lynx: The Plug-and-Play Go Microservices Framework",
	Long:    `Lynx: The Plug-and-Play Go Microservices Framework`,
	Version: release,
}

func init() {
	rootCmd.AddCommand(project.CmdNew)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

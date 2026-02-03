package run

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	watchMode   bool
	buildArgs   string
	runArgs     string
	verbose     bool
	env         []string
	port        string
	skipBuild   bool
)

// CmdRun represents the run command
var CmdRun = &cobra.Command{
	Use:   "run [path]",
	Short: "Build and run the Lynx project",
	Long: `The run command builds and executes your Lynx project with optional hot reload.
It automatically detects the project root, builds the binary, and manages the process lifecycle.`,
	Example: `  # Run the project in current directory
  lynx run
  
  # Run with hot reload (watch for file changes)
  lynx run --watch
  
  # Run a specific project directory
  lynx run ./my-service
  
  # Run with custom build arguments
  lynx run --build-args="-ldflags=-s -w"
  
  # Run with custom runtime arguments
  lynx run --run-args="--config=./config.yaml"
  
  # Skip build and run existing binary
  lynx run --skip-build`,
	RunE: runCommand,
}

func init() {
	CmdRun.Flags().BoolVarP(&watchMode, "watch", "w", false, "Enable hot reload (watch for file changes)")
	CmdRun.Flags().StringVar(&buildArgs, "build-args", "", "Additional arguments for go build")
	CmdRun.Flags().StringVar(&runArgs, "run-args", "", "Arguments to pass to the running application")
	CmdRun.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	CmdRun.Flags().StringArrayVarP(&env, "env", "e", []string{}, "Environment variables (KEY=VALUE)")
	CmdRun.Flags().StringVarP(&port, "port", "p", "", "Override the application port")
	CmdRun.Flags().BoolVar(&skipBuild, "skip-build", false, "Skip build and run existing binary")
}

func runCommand(cmd *cobra.Command, args []string) error {
	// Determine project path
	projectPath := "."
	if len(args) > 0 {
		projectPath = args[0]
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to resolve project path: %w", err)
	}

	// Check if it's a valid Go project
	if err := validateProject(absPath); err != nil {
		return err
	}

	// Print startup message
	fmt.Printf("\n🚀 %s\n", color.CyanString("Starting Lynx project..."))
	fmt.Printf("📁 Project: %s\n", color.YellowString(absPath))

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		fmt.Printf("\n⏹  %s\n", color.YellowString("Shutting down..."))
		cancel()
	}()

	// Create runner
	runner := NewRunner(RunnerConfig{
		ProjectPath: absPath,
		WatchMode:   watchMode,
		BuildArgs:   buildArgs,
		RunArgs:     runArgs,
		Verbose:     verbose,
		Environment: env,
		Port:        port,
		SkipBuild:   skipBuild,
	})

	// Start the runner
	if err := runner.Start(ctx); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	return nil
}

func validateProject(path string) error {
	// Check for go.mod
	goModPath := filepath.Join(path, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return fmt.Errorf("no go.mod found in %s - not a Go project", path)
	}

	// Check for root main.go
	mainPath := filepath.Join(path, "main.go")
	if _, err := os.Stat(mainPath); err == nil {
		return nil
	}

	// Check for cmd/<subdir>/main.go (Lynx layout)
	cmdPath := filepath.Join(path, "cmd")
	cmdInfo, err := os.Stat(cmdPath)
	if err != nil || !cmdInfo.IsDir() {
		return fmt.Errorf("no main.go or cmd/ directory found in %s", path)
	}
	entries, err := os.ReadDir(cmdPath)
	if err != nil {
		return fmt.Errorf("cannot read cmd/ in %s: %w", path, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			mainGo := filepath.Join(cmdPath, e.Name(), "main.go")
			if _, err := os.Stat(mainGo); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("no main.go in project root or under cmd/ in %s", path)
}
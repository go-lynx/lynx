package run

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
)

// RunnerConfig holds configuration for the runner
type RunnerConfig struct {
	ProjectPath string
	WatchMode   bool
	BuildArgs   string
	RunArgs     string
	Verbose     bool
	Environment []string
	Port        string
	SkipBuild   bool
}

// Runner manages the build and run process
type Runner struct {
	config      RunnerConfig
	binaryPath  string
	process     *exec.Cmd
	processLock sync.Mutex
	watcher     *FileWatcher
}

// NewRunner creates a new runner instance
func NewRunner(config RunnerConfig) *Runner {
	return &Runner{
		config: config,
	}
}

// Start begins the build and run process
func (r *Runner) Start(ctx context.Context) error {
	// Determine binary path
	projectName := filepath.Base(r.config.ProjectPath)
	r.binaryPath = filepath.Join(r.config.ProjectPath, "bin", projectName)

	// Initial build and run
	if !r.config.SkipBuild {
		if err := r.build(ctx); err != nil {
			return fmt.Errorf("build failed: %w", err)
		}
	}

	// Start the application
	if err := r.run(ctx); err != nil {
		return fmt.Errorf("run failed: %w", err)
	}

	// If watch mode is enabled, start file watcher
	if r.config.WatchMode {
		r.watcher = NewFileWatcher(r.config.ProjectPath)
		go r.watchFiles(ctx)
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Stop the process
	r.stop()

	return nil
}

// build compiles the project
func (r *Runner) build(ctx context.Context) error {
	startTime := time.Now()
	
	fmt.Printf("🔨 %s\n", color.BlueString("Building project..."))

	// Create bin directory if not exists
	binDir := filepath.Dir(r.binaryPath)
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Prepare build command
	args := []string{"build", "-o", r.binaryPath}
	
	// Add custom build arguments (parseBuildArgs respects quoted strings, e.g. -ldflags="-s -w")
	if r.config.BuildArgs != "" {
		customArgs := parseBuildArgs(r.config.BuildArgs)
		args = append(args, customArgs...)
	}

	// Find main package
	mainPkg := r.findMainPackage()
	args = append(args, mainPkg)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = r.config.ProjectPath
	cmd.Env = append(os.Environ(), r.config.Environment...)

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("❌ Build failed:\n%s\n", color.RedString(string(output)))
		return err
	}

	elapsed := time.Since(startTime)
	fmt.Printf("✅ %s (%.2fs)\n\n", color.GreenString("Build successful"), elapsed.Seconds())
	
	if r.config.Verbose && len(output) > 0 {
		fmt.Printf("Build output:\n%s\n", string(output))
	}

	return nil
}

// findMainPackage locates the main package in the project
func (r *Runner) findMainPackage() string {
	// Check for main.go in root
	mainPath := filepath.Join(r.config.ProjectPath, "main.go")
	if _, err := os.Stat(mainPath); err == nil {
		return "."
	}

	// Check for cmd directory
	cmdPath := filepath.Join(r.config.ProjectPath, "cmd")
	if info, err := os.Stat(cmdPath); err == nil && info.IsDir() {
		// Find first directory with main.go
		entries, _ := os.ReadDir(cmdPath)
		for _, entry := range entries {
			if entry.IsDir() {
				mainFile := filepath.Join(cmdPath, entry.Name(), "main.go")
				if _, err := os.Stat(mainFile); err == nil {
					return "./cmd/" + entry.Name()
				}
			}
		}
	}

	// Default to current directory
	return "."
}

// run starts the compiled binary
func (r *Runner) run(ctx context.Context) error {
	r.processLock.Lock()
	defer r.processLock.Unlock()

	fmt.Printf("▶️  %s\n", color.GreenString("Starting application..."))

	// Prepare run command
	args := []string{}
	if r.config.RunArgs != "" {
		args = strings.Fields(r.config.RunArgs)
	}

	r.process = exec.CommandContext(ctx, r.binaryPath, args...)
	r.process.Dir = r.config.ProjectPath
	
	// Setup environment
	env := append(os.Environ(), r.config.Environment...)
	if r.config.Port != "" {
		env = append(env, fmt.Sprintf("PORT=%s", r.config.Port))
	}
	r.process.Env = env

	// Setup output pipes
	stdout, err := r.process.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := r.process.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := r.process.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Stream output
	go r.streamOutput(stdout, false)
	go r.streamOutput(stderr, true)

	fmt.Printf("📡 %s (PID: %d)\n", color.CyanString("Application running"), r.process.Process.Pid)
	if r.config.WatchMode {
		fmt.Printf("👁  %s\n", color.YellowString("Watching for file changes..."))
	}
	fmt.Println(strings.Repeat("-", 60))

	// Wait for process in background
	go func() {
		if err := r.process.Wait(); err != nil {
			// Process exited with error
			if ctx.Err() == nil {
				// Not a context cancellation
				fmt.Printf("\n❌ %s: %v\n", color.RedString("Application crashed"), err)
			}
		}
	}()

	return nil
}

// streamOutput streams process output to console
func (r *Runner) streamOutput(reader io.Reader, isError bool) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if isError {
			fmt.Fprintf(os.Stderr, "%s\n", line)
		} else {
			fmt.Println(line)
		}
	}
}

// stop terminates the running process
func (r *Runner) stop() {
	r.processLock.Lock()
	defer r.processLock.Unlock()

	if r.process != nil && r.process.Process != nil {
		// Try graceful shutdown first
		r.process.Process.Signal(syscall.SIGTERM)
		
		// Wait a bit for graceful shutdown
		done := make(chan bool, 1)
		go func() {
			r.process.Wait()
			done <- true
		}()

		select {
		case <-done:
			fmt.Printf("✅ %s\n", color.GreenString("Application stopped gracefully"))
		case <-time.After(5 * time.Second):
			// Force kill if not stopped
			r.process.Process.Kill()
			fmt.Printf("⚠️  %s\n", color.YellowString("Application force killed"))
		}
	}
}

// restart rebuilds and restarts the application
func (r *Runner) restart(ctx context.Context) {
	fmt.Printf("\n🔄 %s\n", color.CyanString("Restarting application..."))
	
	// Stop current process
	r.stop()

	// Rebuild
	if err := r.build(ctx); err != nil {
		fmt.Printf("❌ Rebuild failed: %v\n", err)
		return
	}

	// Run again
	if err := r.run(ctx); err != nil {
		fmt.Printf("❌ Restart failed: %v\n", err)
		return
	}
}

// parseBuildArgs splits s into arguments, respecting double-quoted strings
// (e.g. `-ldflags="-s -w"` stays one argument so go build receives it correctly).
func parseBuildArgs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var args []string
	for len(s) > 0 {
		s = strings.TrimLeft(s, " \t")
		if len(s) == 0 {
			break
		}
		if s[0] == '"' {
			end := strings.Index(s[1:], `"`)
			if end == -1 {
				args = append(args, s[1:])
				break
			}
			args = append(args, s[1:end+1])
			s = s[end+2:]
			continue
		}
		i := 0
		for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '"' {
			i++
		}
		args = append(args, s[:i])
		s = s[i:]
	}
	return args
}

// watchFiles monitors file changes and triggers restart
func (r *Runner) watchFiles(ctx context.Context) {
	debouncer := NewDebouncer(1 * time.Second)
	
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-r.watcher.Events():
			if event.Type == FileModified || event.Type == FileCreated {
				if r.config.Verbose {
					fmt.Printf("📝 File changed: %s\n", color.YellowString(event.Path))
				}
				
				debouncer.Trigger(func() {
					r.restart(ctx)
				})
			}
		}
	}
}
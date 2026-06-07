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

// RunnerConfig configures a Runner.
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

// Runner builds the project, runs the resulting binary, and (in watch mode)
// rebuilds and restarts it on file changes.
type Runner struct {
	config      RunnerConfig
	binaryPath  string
	process     *exec.Cmd
	processLock sync.Mutex
	processDone chan struct{} // closed by the crash-monitor goroutine in run()
	watcher     *FileWatcher
}

func NewRunner(config RunnerConfig) *Runner {
	return &Runner{
		config: config,
	}
}

// Start builds (unless skipped) and runs the project, then blocks until ctx is
// cancelled, after which it stops the process. In watch mode a file watcher runs
// in the background and drives rebuild/restart.
func (r *Runner) Start(ctx context.Context) error {
	projectName := filepath.Base(r.config.ProjectPath)
	r.binaryPath = filepath.Join(r.config.ProjectPath, "bin", projectName)

	if !r.config.SkipBuild {
		if err := r.build(ctx); err != nil {
			return fmt.Errorf("build failed: %w", err)
		}
	} else {
		// Verify the binary exists when skipping the build step.
		if _, err := os.Stat(r.binaryPath); os.IsNotExist(err) {
			return fmt.Errorf("--skip-build specified but binary not found: %s", r.binaryPath)
		}
	}

	if err := r.run(ctx); err != nil {
		return fmt.Errorf("run failed: %w", err)
	}

	if r.config.WatchMode {
		r.watcher = NewFileWatcher(r.config.ProjectPath)
		go r.watchFiles(ctx)
	}

	<-ctx.Done()

	r.stop()

	if r.config.WatchMode && r.watcher != nil {
		r.watcher.Stop()
	}

	return nil
}

// build compiles the project's main package to binaryPath.
func (r *Runner) build(ctx context.Context) error {
	startTime := time.Now()

	fmt.Printf("🔨 %s\n", color.BlueString("Building project..."))

	binDir := filepath.Dir(r.binaryPath)
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	args := []string{"build", "-o", r.binaryPath}

	// parseBuildArgs keeps quoted values intact, e.g. -ldflags="-s -w".
	if r.config.BuildArgs != "" {
		customArgs := parseBuildArgs(r.config.BuildArgs)
		args = append(args, customArgs...)
	}

	mainPkg := r.findMainPackage()
	args = append(args, mainPkg)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = r.config.ProjectPath
	cmd.Env = append(os.Environ(), r.config.Environment...)

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

// findMainPackage returns the build target: root if main.go is there, else the
// first cmd/<name> containing main.go, falling back to ".".
func (r *Runner) findMainPackage() string {
	mainPath := filepath.Join(r.config.ProjectPath, "main.go")
	if _, err := os.Stat(mainPath); err == nil {
		return "."
	}

	cmdPath := filepath.Join(r.config.ProjectPath, "cmd")
	if info, err := os.Stat(cmdPath); err == nil && info.IsDir() {
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

	return "."
}

// run launches the built binary, streaming its output, and reaps it in the
// background. A non-zero exit is reported as a crash unless ctx was cancelled.
func (r *Runner) run(ctx context.Context) error {
	r.processLock.Lock()
	defer r.processLock.Unlock()

	fmt.Printf("▶️  %s\n", color.GreenString("Starting application..."))

	args := []string{}
	if r.config.RunArgs != "" {
		// parseBuildArgs handles quoted tokens so arguments with spaces work correctly.
		args = parseBuildArgs(r.config.RunArgs)
	}

	r.process = exec.CommandContext(ctx, r.binaryPath, args...)
	r.process.Dir = r.config.ProjectPath

	env := append(os.Environ(), r.config.Environment...)
	if r.config.Port != "" {
		env = append(env, fmt.Sprintf("PORT=%s", r.config.Port))
	}
	r.process.Env = env

	stdout, err := r.process.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := r.process.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := r.process.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	go r.streamOutput(stdout, false)
	go r.streamOutput(stderr, true)

	fmt.Printf("📡 %s (PID: %d)\n", color.CyanString("Application running"), r.process.Process.Pid)
	if r.config.WatchMode {
		fmt.Printf("👁  %s\n", color.YellowString("Watching for file changes..."))
	}
	fmt.Println(strings.Repeat("-", 60))

	// processDone is closed once this process exits; stop() listens on it
	// instead of calling Wait() again (calling Wait twice is not allowed).
	r.processDone = make(chan struct{})
	go func() {
		defer close(r.processDone)
		if err := r.process.Wait(); err != nil {
			if ctx.Err() == nil {
				fmt.Printf("\n❌ %s: %v\n", color.RedString("Application crashed"), err)
			}
		}
	}()

	return nil
}

// streamOutput copies a process pipe line by line to stdout or stderr.
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

// stop sends SIGTERM and waits up to 5s for the process to exit, then SIGKILLs.
// It uses the processDone channel (closed by the run() goroutine) instead of
// calling Wait() again; calling Wait twice on the same Cmd is not allowed.
func (r *Runner) stop() {
	r.processLock.Lock()
	proc := r.process
	done := r.processDone
	r.processLock.Unlock()

	if proc == nil || proc.Process == nil {
		return
	}

	_ = proc.Process.Signal(syscall.SIGTERM)

	if done == nil {
		// No monitor goroutine (process was never started via run()); fall back.
		time.Sleep(5 * time.Second)
		_ = proc.Process.Kill()
		return
	}

	select {
	case <-done:
		fmt.Printf("✅ %s\n", color.GreenString("Application stopped gracefully"))
	case <-time.After(5 * time.Second):
		_ = proc.Process.Kill()
		fmt.Printf("⚠️  %s\n", color.YellowString("Application force killed"))
	}
}

// restart stops the process, rebuilds, and runs again.
func (r *Runner) restart(ctx context.Context) {
	fmt.Printf("\n🔄 %s\n", color.CyanString("Restarting application..."))

	r.stop()

	if err := r.build(ctx); err != nil {
		fmt.Printf("❌ Rebuild failed: %v\n", err)
		return
	}

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

// watchFiles restarts the app on relevant file changes, debounced to coalesce
// bursts of edits into a single rebuild.
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

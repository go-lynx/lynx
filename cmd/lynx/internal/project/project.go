package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/go-lynx/lynx/cmd/lynx/internal/base"
	"github.com/spf13/cobra"
)

// CmdNew represents the `new` command for creating a Lynx service template project.
var CmdNew = &cobra.Command{
	Use:   "new",
	Short: "Create a lynx service template",
	Long:  "Create a lynx service project using the repository template.",
	RunE:  run, // function called when executing the command
}

var (
	// repoURL stores the layout repository URL.
	repoURL string
	// branch stores the repository's branch name.
	branch string
	// ref stores the repository reference (takes precedence over branch).
	ref string
	// timeout stores the timeout for project creation.
	timeout string
	// module stores the Go module path for the generated project.
	module string
	// force indicates whether to overwrite an existing directory without prompt.
	force bool
	// postTidy indicates whether to run 'go mod tidy' after creation.
	postTidy bool
	// concurrency stores the concurrency limit.
	concurrency int
)

// init initializes defaults and command-line flags.
func init() {
	// Get the repository URL from the LYNX_LAYOUT_REPO environment variable; use the default if empty.
	if repoURL = os.Getenv("LYNX_LAYOUT_REPO"); repoURL == "" {
		repoURL = "https://github.com/go-lynx/lynx-layout.git"
	}
	timeout = "60s" // default timeout is 60 seconds
	// Add the --repo-url flag to specify the layout repository URL.
	CmdNew.Flags().StringVarP(&repoURL, "repo-url", "r", repoURL, "layout repo")
	// Add the --branch flag to specify the repository branch.
	CmdNew.Flags().StringVarP(&branch, "branch", "b", branch, "repo branch")
	// Add the --ref flag to specify commit/tag/branch (takes precedence over --branch).
	CmdNew.Flags().StringVar(&ref, "ref", ref, "repo ref (commit/tag/branch), takes precedence over --branch")
	// Add the --timeout flag to specify the timeout.
	CmdNew.Flags().StringVarP(&timeout, "timeout", "t", timeout, "time out")
	// Add the --module flag to specify the Go module path for the new project (e.g., github.com/acme/foo).
	CmdNew.Flags().StringVarP(&module, "module", "m", module, "go module path for the new project")
	// Add the --force flag to overwrite an existing directory without prompt.
	CmdNew.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing directory without prompt")
	// Add the --post-tidy flag to automatically run 'go mod tidy' after creation (disabled by default).
	CmdNew.Flags().BoolVar(&postTidy, "post-tidy", false, "run 'go mod tidy' in the new project after creation")
	// Compute the default concurrency limit (min(4, NumCPU*2)).
	defaultConc := runtime.NumCPU() * 2
	if defaultConc > 4 {
		defaultConc = 4
	}
	if defaultConc < 1 {
		defaultConc = 1
	}
	concurrency = defaultConc
	CmdNew.Flags().IntVarP(&concurrency, "concurrency", "c", concurrency, "max concurrent project creations")
}

// run executes the `new` command and creates Lynx service projects.
func run(_ *cobra.Command, args []string) error {
	// Get the current working directory.
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Parse the timeout string into a time.Duration.
	t, err := time.ParseDuration(timeout)
	if err != nil {
		return err
	}

	// Create a context with the specified timeout.
	ctx, cancel := context.WithTimeout(context.Background(), t)
	defer cancel() // ensure the context is canceled when the function returns

	var names []string
	if len(args) == 0 {
		// Prompt for project names if not provided via command-line arguments.
		prompt := &survey.Input{
			Message: base.T("project_names"),
			Help:    base.T("project_names_help"),
		}
		var input string
		errAsk := survey.AskOne(prompt, &input)
		if errAsk != nil || input == "" {
			base.Errorf("%s", base.T("no_project_names"))
			return fmt.Errorf("no project names provided")
		}
		// Split the input project names by spaces.
		names = strings.Split(input, " ")
	} else {
		names = args
	}

	// Check and remove duplicate project names.
	names = checkDuplicates(names)
	if len(names) < 1 {
		base.Errorf("%s", base.T("no_project_names"))
		return fmt.Errorf("no valid project names after de-duplication")
	}

	// Create multiple projects concurrently.
	done := make(chan error, len(names))
	var wg sync.WaitGroup
	// Use the runtime concurrency limit from --concurrency, or fallback to CPU*2.
	maxConc := concurrency
	if maxConc <= 0 {
		maxConc = runtime.NumCPU() * 2
	}
	if maxConc < 1 {
		maxConc = 1
	}
	if len(names) < maxConc {
		maxConc = len(names)
	}
	sem := make(chan struct{}, maxConc)
	// Determine the effective reference: --ref first, then --branch.
	effectiveRef := ref
	if strings.TrimSpace(effectiveRef) == "" {
		effectiveRef = branch
	}
	for _, name := range names {
		// Process the project name and working directory parameters.
		projectName, workingDir := processProjectParams(name, wd)
		p := &Project{Name: projectName}
		wg.Add(1)
		sem <- struct{}{}
		go func(p *Project, workingDir string) {
			// Call Project.New to create the project and send the result to the 'done' channel.
			defer func() { <-sem; wg.Done() }()
			done <- p.New(ctx, workingDir, repoURL, effectiveRef, force, module, postTidy)
		}(p, workingDir)
	}

	wg.Wait()   // Wait for all goroutines to complete
	close(done) // Close the 'done' channel

	// Read errors from the 'done' channel and print them
	fail := 0
	for err := range done {
		if err != nil {
			fail++
			base.Errorf("%s", fmt.Sprintf(base.T("failed_create"), err.Error()))
			// Print actionable suggestions.
			printSuggestions(err.Error())
		}
	}
	// Check whether the context was canceled due to timeout.
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		base.Errorf("%s", base.T("timeout"))
		return context.DeadlineExceeded
	}
	if fail > 0 {
		return fmt.Errorf("%d project(s) failed", fail)
	}
	return nil
}

// Print actionable suggestions based on error keywords.
func printSuggestions(msg string) {
	low := strings.ToLower(msg)
	say := func(key string, args ...any) { fmt.Fprintln(os.Stderr, fmt.Sprintf(base.T(key), args...)) }
	switch {
	case strings.Contains(low, "could not resolve host") || strings.Contains(low, "couldn't resolve host") || strings.Contains(low, "name or service not known"):
		say("suggestion_dns")
	case strings.Contains(low, "timed out") || strings.Contains(low, "timeout") || strings.Contains(low, "i/o timeout"):
		say("suggestion_timeout")
	case strings.Contains(low, "authentication") || strings.Contains(low, "permission denied") || strings.Contains(low, "auth"):
		say("suggestion_auth")
	case strings.Contains(low, "safe.directory"):
		say("suggestion_safe")
	case strings.Contains(low, "not found") && strings.Contains(low, "origin/"):
		say("suggestion_remote")
	}
}

// processProjectParams handles the project name parameter and returns the processed project name and working directory.
func processProjectParams(projectName string, workingDir string) (projectNameResult, workingDirResult string) {
	_projectDir := projectName
	// Expand the home directory only when the project name starts with "~/".
	if strings.HasPrefix(projectName, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			_projectDir = filepath.Join(homeDir, projectName[2:])
		}
	}

	// If the project name is a relative path, convert it to an absolute path.
	if !filepath.IsAbs(_projectDir) {
		joined := filepath.Join(workingDir, _projectDir)
		if absPath, err := filepath.Abs(joined); err == nil {
			_projectDir = absPath
		} else {
			// Fallback: use the joined path.
			_projectDir = joined
		}
	}

	// Return the processed project name (last path segment) and working directory (directory part).
	return filepath.Base(_projectDir), filepath.Dir(_projectDir)
}

// checkDuplicates removes duplicate project names and validates their format.
func checkDuplicates(names []string) []string {
	encountered := map[string]bool{}
	var result []string

	// Define the valid character pattern for project names.
	pattern := `^[A-Za-z0-9_-]+$`
	regex := regexp.MustCompile(pattern)

	for _, name := range names {
		// If the name matches the pattern and hasn't been seen, add it to the results.
		if regex.MatchString(name) && !encountered[name] {
			encountered[name] = true
			result = append(result, name)
		}
	}
	return result
}

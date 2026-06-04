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
	"github.com/go-lynx/lynx/cmd/lynx/internal/plugin"
	"github.com/spf13/cobra"
)

// CmdNew represents the `new` command for creating a Lynx service template project.
var CmdNew = &cobra.Command{
	Use:   "new [name...]",
	Short: "Create a lynx service template",
	Long: `Create one or more Lynx service projects from the layout template.

Pass one or more project names as arguments, or run without arguments to be
prompted interactively. Names may be plain (myservice) or path-like (apps/api).
The template is cloned from the layout repo (override with --repo-url / LYNX_LAYOUT_REPO).`,
	Example: `  # Interactive: prompts for name(s) and plugins
  lynx new

  # Create a single service
  lynx new myservice

  # Set the Go module path and run 'go mod tidy' afterwards
  lynx new myservice --module github.com/acme/myservice --post-tidy

  # Preselect plugins (skips the interactive picker)
  lynx new myservice --plugins http,grpc,redis

  # Create several services at once, no plugins
  lynx new svc-a svc-b svc-c --skip-plugins

  # Pin the template to a tag/commit and overwrite an existing dir
  lynx new myservice --ref v1.6.2 --force`,
	Args: cobra.ArbitraryArgs,
	RunE: run,
}

var (
	repoURL string
	branch  string
	// ref takes precedence over branch when both are set.
	ref          string
	timeout      string
	module       string
	force        bool
	postTidy     bool
	concurrency  int
	pluginsComma string
	skipPlugins  bool
)

func init() {
	// Default layout repo, overridable via env or the --repo-url flag.
	if repoURL = os.Getenv("LYNX_LAYOUT_REPO"); repoURL == "" {
		repoURL = "https://github.com/go-lynx/lynx-layout.git"
	}
	timeout = "60s"
	CmdNew.Flags().StringVarP(&repoURL, "repo-url", "r", repoURL, "layout repo")
	CmdNew.Flags().StringVarP(&branch, "branch", "b", branch, "repo branch")
	CmdNew.Flags().StringVar(&ref, "ref", ref, "repo ref (commit/tag/branch), takes precedence over --branch")
	CmdNew.Flags().StringVarP(&timeout, "timeout", "t", timeout, "time out")
	CmdNew.Flags().StringVarP(&module, "module", "m", module, "go module path for the new project")
	CmdNew.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing directory without prompt")
	CmdNew.Flags().BoolVar(&postTidy, "post-tidy", false, "run 'go mod tidy' in the new project after creation")
	// Default concurrency: min(4, NumCPU*2), clamped to at least 1.
	defaultConc := runtime.NumCPU() * 2
	if defaultConc > 4 {
		defaultConc = 4
	}
	if defaultConc < 1 {
		defaultConc = 1
	}
	concurrency = defaultConc
	CmdNew.Flags().IntVarP(&concurrency, "concurrency", "c", concurrency, "max concurrent project creations")
	CmdNew.Flags().StringVarP(&pluginsComma, "plugins", "p", "", "comma-separated plugin names to add (e.g. http,grpc,redis); skips interactive selection")
	CmdNew.Flags().BoolVar(&skipPlugins, "skip-plugins", false, "skip plugin selection and do not add any plugins")
}

// run creates one or more service projects, optionally in parallel.
func run(_ *cobra.Command, args []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	t, err := time.ParseDuration(timeout)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), t)
	defer cancel()

	var names []string
	if len(args) == 0 {
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
		// Split the input project names on any run of whitespace (handles tabs
		// and multiple spaces; avoids empty entries from strings.Split).
		names = strings.Fields(input)
	} else {
		names = args
	}

	names = checkDuplicates(names)
	if len(names) < 1 {
		base.Errorf("%s", base.T("no_project_names"))
		return fmt.Errorf("no valid project names after de-duplication")
	}

	// Resolve plugin selection once: avoid interactive prompt in concurrent goroutines.
	// When --skip-plugins: use empty; when --plugins: parse and resolve; when single project: pass nil (interactive in New); else prompt once and apply to all.
	var preSelectedPlugins *[]*plugin.PluginMetadata
	if skipPlugins {
		empty := []*plugin.PluginMetadata(nil)
		preSelectedPlugins = &empty
	} else if strings.TrimSpace(pluginsComma) != "" {
		resolved, err := ResolvePluginNames(pluginsComma)
		if err != nil {
			base.Warnf("Resolve plugins failed: %v (continue without plugins)\n", err)
			empty := []*plugin.PluginMetadata(nil)
			preSelectedPlugins = &empty
		} else {
			preSelectedPlugins = &resolved
		}
	} else if len(names) > 1 {
		// Multiple projects: prompt once so we don't run multiple interactive prompts concurrently
		selected, err := selectPlugins()
		if err != nil {
			base.Warnf("Plugin selection failed: %v\n", err)
		}
		preSelectedPlugins = &selected
	}
	// When len(names)==1 and no --plugins/--skip-plugins: preSelectedPlugins stays nil → interactive inside New()

	done := make(chan error, len(names))
	var wg sync.WaitGroup
	// Bound parallelism by --concurrency (fallback CPU*2), never above len(names).
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
	// --ref wins over --branch.
	effectiveRef := ref
	if strings.TrimSpace(effectiveRef) == "" {
		effectiveRef = branch
	}
	for _, name := range names {
		projectName, workingDir := processProjectParams(name, wd)
		p := &Project{Name: projectName}
		wg.Add(1)
		sem <- struct{}{}
		go func(p *Project, workingDir string, plugins *[]*plugin.PluginMetadata) {
			defer func() { <-sem; wg.Done() }()
			done <- p.New(ctx, workingDir, repoURL, effectiveRef, force, module, postTidy, plugins)
		}(p, workingDir, preSelectedPlugins)
	}

	wg.Wait()
	close(done)

	fail := 0
	for err := range done {
		if err != nil {
			fail++
			base.Errorf("%s", fmt.Sprintf(base.T("failed_create"), err.Error()))
			printSuggestions(err.Error())
		}
	}
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
	say := func(key string, args ...any) { fmt.Fprintln(os.Stderr, base.T(key, args...)) }
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

// processProjectParams resolves a name (plain, path-like, or "~/"-prefixed) into
// an absolute target, returning its final segment as the project name and its
// parent as the working directory.
func processProjectParams(projectName string, workingDir string) (projectNameResult, workingDirResult string) {
	_projectDir := projectName
	if strings.HasPrefix(projectName, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			_projectDir = filepath.Join(homeDir, projectName[2:])
		}
	}

	if !filepath.IsAbs(_projectDir) {
		joined := filepath.Join(workingDir, _projectDir)
		if absPath, err := filepath.Abs(joined); err == nil {
			_projectDir = absPath
		} else {
			_projectDir = joined
		}
	}

	return filepath.Base(_projectDir), filepath.Dir(_projectDir)
}

// checkDuplicates removes duplicate project names and validates their format.
// Path-like names (containing "/") are allowed and kept as-is; simple names must match [A-Za-z0-9_-]+.
func checkDuplicates(names []string) []string {
	encountered := map[string]bool{}
	var result []string

	// Valid character pattern for simple project names (no path).
	pattern := `^[A-Za-z0-9_-]+$`
	regex := regexp.MustCompile(pattern)

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if encountered[name] {
			continue
		}
		// Allow path-like names (e.g. "foo/bar/svc"); only validate simple names with regex.
		if strings.Contains(name, "/") {
			encountered[name] = true
			result = append(result, name)
			continue
		}
		if regex.MatchString(name) {
			encountered[name] = true
			result = append(result, name)
		}
	}
	return result
}

package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

// CmdNew represents the new command.
var CmdNew = &cobra.Command{
	Use:   "new",
	Short: "Create a lynx service template",
	Long:  "Create a lynx service project using the repository template.",
	Run:   run,
}

var (
	repoURL string
	branch  string
	timeout string
)

func init() {
	if repoURL = os.Getenv("LYNX_LAYOUT_REPO"); repoURL == "" {
		repoURL = "https://github.com/go-lynx/lynx-layout.git"
	}
	timeout = "60s"
	CmdNew.Flags().StringVarP(&repoURL, "repo-url", "r", repoURL, "layout repo")
	CmdNew.Flags().StringVarP(&branch, "branch", "b", branch, "repo branch")
	CmdNew.Flags().StringVarP(&timeout, "timeout", "t", timeout, "time out")
}

func run(_ *cobra.Command, args []string) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	t, err := time.ParseDuration(timeout)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), t)
	defer cancel()

	var names []string
	if len(args) == 0 {
		prompt := &survey.Input{
			Message: "What are the project names ?",
			Help:    "Enter project names separated by space.",
		}
		var input string
		err = survey.AskOne(prompt, &input)
		if err != nil || input == "" {
			fmt.Printf("\n❌ No project names found,Please provide the correct project name")
			return
		}
		names = strings.Split(input, " ")
	} else {
		names = args
	}

	names = checkDuplicates(names)
	if len(names) < 1 {
		fmt.Printf("\n❌ No project names found,Please provide the correct project name")
		return
	}

	// creation of multiple projects
	done := make(chan error, len(names))
	var wg sync.WaitGroup
	for _, name := range names {
		wg.Add(1)
		projectName, workingDir := processProjectParams(name, wd)
		p := &Project{Name: projectName}
		go func() {
			done <- p.New(ctx, workingDir, repoURL, branch)
			wg.Done()
		}()
	}

	wg.Wait()
	close(done)

	// Read errors from the done channel until it's closed
	for err := range done {
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "\033[31mERROR: Failed to create project(%s)\033[m\n", err.Error())
		}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		_, _ = fmt.Fprint(os.Stderr, "\033[31mERROR: project creation timed out\033[m\n")
		return
	}
}

func processProjectParams(projectName string, workingDir string) (projectNameResult, workingDirResult string) {
	_projectDir := projectName
	_workingDir := workingDir
	// Process ProjectName with system variable
	if strings.HasPrefix(projectName, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// cannot get user home return fallback place dir
			return _projectDir, _workingDir
		}
		_projectDir = filepath.Join(homeDir, projectName[2:])
	}

	// check path is relative
	if !filepath.IsAbs(projectName) {
		absPath, err := filepath.Abs(projectName)
		if err != nil {
			return _projectDir, _workingDir
		}
		_projectDir = absPath
	}

	return filepath.Base(_projectDir), filepath.Dir(_projectDir)
}

func checkDuplicates(names []string) []string {
	encountered := map[string]bool{}
	var result []string

	pattern := `^[A-Za-z0-9_-]+$`
	regex := regexp.MustCompile(pattern)

	for _, name := range names {
		if regex.MatchString(name) && !encountered[name] {
			encountered[name] = true
			result = append(result, name)
		}
	}
	return result
}

package project

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"

	"github.com/go-lynx/lynx/cmd/lynx/internal/base"
)

// Project represents a project template, containing project name and path information.
type Project struct {
	Name string // Project name
	Path string // Project path
}

// New creates a new project from a remote repository.
// ctx: Context for controlling the lifecycle of the operation.
// dir: Target directory for project creation.
// layout: Remote repository address for project layout.
// branch: Remote repository branch to use.
// force: Whether to force overwrite existing project directory.
// module: If provided, replaces the module in template go.mod.
// postTidy: Whether to execute go mod tidy command.
// Returns: Returns corresponding error information if an error occurs during operation; otherwise returns nil.
func (p *Project) New(ctx context.Context, dir string, layout string, branch string, force bool, module string, postTidy bool) error {
	// Calculate the complete path where the project will be created
	to := filepath.Join(dir, p.Name)

	// Check if target path already exists
	if _, err := os.Stat(to); !os.IsNotExist(err) {
		// If exists, notify user that path already exists
		base.Warnf("%s", fmt.Sprintf(base.T("already_exists"), p.Name))
		// --force will silently overwrite, otherwise interactive confirmation
		if !force {
			prompt := &survey.Confirm{
				Message: base.T("override_confirm"),
				Help:    base.T("override_help"),
			}
			var override bool
			if e := survey.AskOne(prompt, &override); e != nil {
				return e
			}
			if !override {
				return err
			}
		}
		if e := os.RemoveAll(to); e != nil {
			return e
		}
	}

	// Notify user to start creating project and display project name and layout repository information
	base.Infof("%s", fmt.Sprintf(base.T("creating_service"), p.Name, layout))
	// Create a new repository instance
	repo := base.NewRepo(layout, branch)
	// Copy remote repository content to target path, excluding .git and .github directories
	// If --module is provided, replace the module in template go.mod; otherwise don't replace template module
	if err := repo.CopyToV2(ctx, to, module, []string{".git", ".github"}, nil); err != nil {
		return err
	}
	// Rename the user directory under cmd directory to project name
	e := os.Rename(
		filepath.Join(to, "cmd", "user"),
		filepath.Join(to, "cmd", p.Name),
	)
	if e != nil {
		return e
	}
	// Print project directory structure
	base.Tree(to, dir)

	// Optional: Execute go mod tidy
	if postTidy {
		cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
		cmd.Dir = to
		if out, err := cmd.CombinedOutput(); err != nil {
			base.Warnf("%s", fmt.Sprintf(base.T("mod_tidy_failed"), err, string(out)))
		} else {
			base.Infof("%s", base.T("mod_tidy_ok"))
		}
	}

	// Notify user that project creation was successful
	base.Infof("%s", fmt.Sprintf(base.T("project_success"), color.GreenString(p.Name)))
	// Prompt user to use the following commands to start the project
	base.Infof("%s", base.T("start_cmds_header"))
	base.Infof("%s\n", color.WhiteString("$ cd %s", p.Name))
	if !postTidy {
		base.Infof("%s\n", color.WhiteString("$ go mod tidy"))
	}
	base.Infof("%s\n", color.WhiteString("$ go generate ./..."))
	base.Infof("%s\n", color.WhiteString("$ go build -o ./bin/ ./... "))
	base.Infof("%s\n", color.WhiteString("$ ./bin/%s -conf ./configs", p.Name))
	// Thank user for using Lynx and provide tutorial link
	base.Infof("%s", base.T("thanks"))
	base.Infof("%s", base.T("tutorial"))
	return nil
}

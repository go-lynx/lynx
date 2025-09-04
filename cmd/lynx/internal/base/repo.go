package base

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// unExpandVarPath defines a list of path prefixes that don't need variable expansion.
var unExpandVarPath = []string{"~", ".", ".."}

// Repo represents a Git repository manager for managing repository cloning, pulling, and copying operations.
type Repo struct {
	url    string // Remote URL of the repository
	home   string // Local cache path of the repository
	branch string // Repository branch to operate on
}

// repoDir generates a relatively unique directory name based on the repository URL.
// Parameter url is the remote URL of the repository.
// Returns the generated directory name.
func repoDir(url string) string {
	// Parse repository VCS URL
	vcsURL, err := ParseVCSUrl(url)
	if err != nil {
		// Return original URL when parsing fails
		return url
	}
	// Check if hostname contains port number
	host, _, err := net.SplitHostPort(vcsURL.Host)
	if err != nil {
		// Use original hostname when no port number is included
		host = vcsURL.Host
	}
	// Remove path prefixes that don't need variable expansion from hostname
	for _, p := range unExpandVarPath {
		host = strings.TrimLeft(host, p)
	}
	// Get the second-to-last directory name in URL path
	dir := path.Base(path.Dir(vcsURL.Path))
	// Combine hostname and directory name as the final directory name
	url = fmt.Sprintf("%s/%s", host, dir)
	return url
}

// NewRepo creates a new repository manager instance.
// Parameter url is the remote URL of the repository, branch is the repository branch to operate on.
// Returns a pointer to the newly created Repo instance.
func NewRepo(url string, branch string) *Repo {
	return &Repo{
		url: url,
		// Calculate local cache path of the repository
		home:   lynxHomeWithDir("repo/" + repoDir(url)),
		branch: branch,
	}
}

// Path returns the local cache path of the repository.
// Returns the local cache path as a string.
func (r *Repo) Path() string {
	// Find the index of the last '/' in URL
	start := strings.LastIndex(r.url, "/")
	// Find the index of the last '.git' in URL
	end := strings.LastIndex(r.url, ".git")
	if end == -1 {
		// If no '.git', take the length of URL
		end = len(r.url)
	}
	var branch string
	if r.branch == "" {
		// If branch name is empty, default to '@main'
		branch = "@main"
	} else {
		// Otherwise, add '@' prefix
		branch = "@" + r.branch
	}
	// Combine local cache path
	return path.Join(r.home, r.url[start+1:end]+branch)
}

// Pull pulls the latest code from remote repository to local cache path.
// Parameter ctx is the context for controlling the lifecycle of the operation.
// Returns errors that may occur during the operation.
func (r *Repo) Pull(ctx context.Context) error {
	// Ensure git is available
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH: %w", err)
	}
	// Ensure directory exists
	if _, err := os.Stat(r.Path()); os.IsNotExist(err) {
		return fmt.Errorf("repo path does not exist: %s", r.Path())
	}
	// Fetch all remotes and tags (with retry), and prune expired references
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"fetch", "--all", "--tags", "--prune"}, 2); err != nil {
		return fmt.Errorf("git fetch failed (check network/proxy or git auth): %w", err)
	}
	// Target ref: prioritize r.branch (actually ref), if empty then maintain current HEAD
	ref := strings.TrimSpace(r.branch)
	if ref == "" {
		// No specific ref, keep current branch and sync with remote (skip if in detached HEAD)
		if bOut, bErr := RunCMD(ctx, r.Path(), "git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, 0); bErr == nil {
			cur := strings.TrimSpace(bOut)
			if cur != "HEAD" && cur != "" {
				if _, err := RunCMD(ctx, r.Path(), "git", []string{"reset", "--hard", fmt.Sprintf("origin/%s", cur)}, 1); err != nil {
					return fmt.Errorf("git reset --hard origin/%s failed: %w", cur, err)
				}
			}
		}
		return nil
	}
	// Has ref: prioritize as remote branch, otherwise as tag/commit (detached HEAD)
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"show-ref", "--verify", fmt.Sprintf("refs/remotes/origin/%s", ref)}, 0); err == nil {
		// Remote branch exists: switch to that branch and force sync with remote
		if _, err := RunCMD(ctx, r.Path(), "git", []string{"checkout", "-B", ref, fmt.Sprintf("origin/%s", ref)}, 0); err != nil {
			return fmt.Errorf("git checkout branch %s failed: %w", ref, err)
		}
		if _, err := RunCMD(ctx, r.Path(), "git", []string{"reset", "--hard", fmt.Sprintf("origin/%s", ref)}, 1); err != nil {
			return fmt.Errorf("git reset --hard origin/%s failed: %w", ref, err)
		}
		return nil
	}
	// Treat as tag/commit: detached HEAD checkout
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"checkout", "--detach", ref}, 0); err != nil {
		return fmt.Errorf("git checkout %s failed (tag/commit?): %w", ref, err)
	}
	return nil
}

// Clone clones remote repository to local cache path. If local cache path already exists, try to pull latest code.
// Parameter ctx is the context for controlling the lifecycle of the operation.
// Returns errors that may occur during the operation.
func (r *Repo) Clone(ctx context.Context) error {
	// Ensure git is available
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH: %w", err)
	}
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(r.Path()), 0o755); err != nil {
		return fmt.Errorf("prepare repo parent dir failed: %w", err)
	}
	// Check if local cache path already exists
	if _, err := os.Stat(r.Path()); !os.IsNotExist(err) {
		// If exists, try to pull latest code
		return r.Pull(ctx)
	}
	var err error
	// Prioritize shallow clone of default branch, then switch to ref (more general)
	_, err = RunCMD(ctx, filepath.Dir(r.Path()), "git", []string{"clone", "--depth", "1", r.url, r.Path()}, 2)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	// Get tags and all remote references
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"fetch", "--all", "--tags", "--prune"}, 2); err != nil {
		return fmt.Errorf("git fetch after clone failed: %w", err)
	}
	// If ref is specified, try to switch
	ref := strings.TrimSpace(r.branch)
	if ref == "" {
		return nil
	}
	// Prioritize as remote branch
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"show-ref", "--verify", fmt.Sprintf("refs/remotes/origin/%s", ref)}, 0); err == nil {
		if _, err := RunCMD(ctx, r.Path(), "git", []string{"checkout", "-B", ref, fmt.Sprintf("origin/%s", ref)}, 0); err != nil {
			return fmt.Errorf("git checkout branch %s failed: %w", ref, err)
		}
		return nil
	}
	// As tag/commit detached HEAD
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"checkout", "--detach", ref}, 0); err != nil {
		return fmt.Errorf("git checkout %s failed (tag/commit?): %w", ref, err)
	}
	return nil
}

// CopyTo copies cloned repository content to specified project path.
// Parameters: ctx is the context for controlling the lifecycle of the operation; to is the target project path; modPath is the module path; ignores is the list of files or directories to ignore.
// Returns errors that may occur during the operation.
func (r *Repo) CopyTo(ctx context.Context, to string, modPath string, ignores []string) error {
	// First clone repository to local cache path
	if err := r.Clone(ctx); err != nil {
		return err
	}
	// Get module path from go.mod file under local cache path
	mod, err := ModulePath(filepath.Join(r.Path(), "go.mod"))
	if err != nil {
		return err
	}
	// Copy directory content
	return copyDir(r.Path(), to, []string{mod, modPath}, ignores)
}

// CopyToV2 copies cloned repository content to specified project path, supporting more replacement rules.
// Parameters: ctx is the context for controlling the lifecycle of the operation; to is the target project path; modPath is the module path; ignores is the list of files or directories to ignore; replaces is the list of content to replace.
// Returns errors that may occur during the operation.
func (r *Repo) CopyToV2(ctx context.Context, to string, modPath string, ignores, replaces []string) error {
	// First clone repository to local cache path
	if err := r.Clone(ctx); err != nil {
		return err
	}
	// Get module path from go.mod file under local cache path
	mod, err := ModulePath(filepath.Join(r.Path(), "go.mod"))
	if err != nil {
		return err
	}
	// Only add module path replacement to list when modPath is provided; if empty, don't replace template module
	if strings.TrimSpace(modPath) != "" {
		replaces = append([]string{mod, modPath}, replaces...)
	}
	// Copy directory content
	return copyDir(r.Path(), to, replaces, ignores)
}

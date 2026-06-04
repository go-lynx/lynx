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

// Repo manages a cached clone of a template Git repository: cloning, pulling
// the requested ref, and copying the tree into a generated project.
type Repo struct {
	url    string // Remote URL of the repository
	home   string // Local cache path of the repository
	branch string // Repository branch to operate on
}

// repoDir derives a cache subdirectory ("host/org") from a repository URL so
// that distinct templates don't collide on disk. The raw URL is returned if it
// can't be parsed.
func repoDir(url string) string {
	vcsURL, err := ParseVCSUrl(url)
	if err != nil {
		return url
	}
	host, _, err := net.SplitHostPort(vcsURL.Host)
	if err != nil {
		host = vcsURL.Host
	}
	for _, p := range unExpandVarPath {
		host = strings.TrimLeft(host, p)
	}
	dir := path.Base(path.Dir(vcsURL.Path))
	url = fmt.Sprintf("%s/%s", host, dir)
	return url
}

// NewRepo returns a Repo for the given URL and ref (branch, tag, or commit),
// backed by a cache directory under ~/.lynx/repo.
func NewRepo(url string, branch string) (*Repo, error) {
	home, err := lynxHomeWithDir("repo/" + repoDir(url))
	if err != nil {
		return nil, err
	}
	return &Repo{
		url:    url,
		home:   home,
		branch: branch,
	}, nil
}

// Path returns the on-disk checkout path, named "<repo>@<ref>" so different
// refs of the same template are cached side by side. Empty ref maps to "@main".
func (r *Repo) Path() string {
	start := strings.LastIndex(r.url, "/")
	end := strings.LastIndex(r.url, ".git")
	if end == -1 {
		end = len(r.url)
	}
	var branch string
	if r.branch == "" {
		branch = "@main"
	} else {
		branch = "@" + r.branch
	}
	return path.Join(r.home, r.url[start+1:end]+branch)
}

// Pull updates the cached checkout to match the configured ref. With no ref it
// fast-forwards the current branch to its upstream; with a ref it resolves it as
// a remote branch first, then falls back to a detached tag/commit checkout.
func (r *Repo) Pull(ctx context.Context) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH: %w", err)
	}
	if _, err := os.Stat(r.Path()); os.IsNotExist(err) {
		return fmt.Errorf("repo path does not exist: %s", r.Path())
	}
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"fetch", "--all", "--tags", "--prune"}, 2); err != nil {
		return fmt.Errorf("git fetch failed (check network/proxy or git auth): %w", err)
	}
	ref := strings.TrimSpace(r.branch)
	if ref == "" {
		// No ref: sync the current branch to its upstream, but skip detached HEAD.
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
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"show-ref", "--verify", fmt.Sprintf("refs/remotes/origin/%s", ref)}, 0); err == nil {
		// Remote branch: check it out and force-sync to the remote tip.
		if _, err := RunCMD(ctx, r.Path(), "git", []string{"checkout", "-B", ref, fmt.Sprintf("origin/%s", ref)}, 0); err != nil {
			return fmt.Errorf("git checkout branch %s failed: %w", ref, err)
		}
		if _, err := RunCMD(ctx, r.Path(), "git", []string{"reset", "--hard", fmt.Sprintf("origin/%s", ref)}, 1); err != nil {
			return fmt.Errorf("git reset --hard origin/%s failed: %w", ref, err)
		}
		return nil
	}
	// Not a branch: treat ref as a tag/commit via detached checkout.
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"checkout", "--detach", ref}, 0); err != nil {
		return fmt.Errorf("git checkout %s failed (tag/commit?): %w", ref, err)
	}
	return nil
}

// Clone populates the cache for this repo, delegating to Pull if the checkout
// already exists. A fresh clone is shallow, then a full fetch makes all
// branches and tags available before resolving the requested ref.
func (r *Repo) Clone(ctx context.Context) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(r.Path()), 0o755); err != nil {
		return fmt.Errorf("prepare repo parent dir failed: %w", err)
	}
	if _, err := os.Stat(r.Path()); !os.IsNotExist(err) {
		return r.Pull(ctx)
	}
	var err error
	_, err = RunCMD(ctx, filepath.Dir(r.Path()), "git", []string{"clone", "--depth", "1", r.url, r.Path()}, 2)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"fetch", "--all", "--tags", "--prune"}, 2); err != nil {
		return fmt.Errorf("git fetch after clone failed: %w", err)
	}
	ref := strings.TrimSpace(r.branch)
	if ref == "" {
		return nil
	}
	// Resolve ref as a remote branch first, then as a tag/commit (see Pull).
	if _, showRefErr := RunCMD(ctx, r.Path(), "git", []string{"show-ref", "--verify", fmt.Sprintf("refs/remotes/origin/%s", ref)}, 0); showRefErr == nil {
		if _, checkoutErr := RunCMD(ctx, r.Path(), "git", []string{"checkout", "-B", ref, fmt.Sprintf("origin/%s", ref)}, 0); checkoutErr != nil {
			return fmt.Errorf("git checkout branch %s failed: %w", ref, checkoutErr)
		}
		return nil
	}
	if _, err := RunCMD(ctx, r.Path(), "git", []string{"checkout", "--detach", ref}, 0); err != nil {
		return fmt.Errorf("git checkout %s failed (tag/commit?): %w", ref, err)
	}
	return nil
}

// CopyTo clones (or refreshes) the template and copies it to to, rewriting the
// template's module path (read from its go.mod) to modPath.
func (r *Repo) CopyTo(ctx context.Context, to string, modPath string, ignores []string) error {
	if err := r.Clone(ctx); err != nil {
		return err
	}
	mod, err := ModulePath(filepath.Join(r.Path(), "go.mod"))
	if err != nil {
		return err
	}
	return copyDir(r.Path(), to, []string{mod, modPath}, ignores)
}

// CopyToV2 is like CopyTo but accepts extra replacements applied on top of the
// module-path rewrite. The module rewrite is skipped when modPath is empty.
func (r *Repo) CopyToV2(ctx context.Context, to string, modPath string, ignores, replaces []string) error {
	if err := r.Clone(ctx); err != nil {
		return err
	}
	mod, err := ModulePath(filepath.Join(r.Path(), "go.mod"))
	if err != nil {
		return err
	}
	if strings.TrimSpace(modPath) != "" {
		replaces = append([]string{mod, modPath}, replaces...)
	}
	return copyDir(r.Path(), to, replaces, ignores)
}

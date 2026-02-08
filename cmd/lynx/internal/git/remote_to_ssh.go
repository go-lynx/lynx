package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	remoteToSSHDir string
)

var cmdRemoteToSSH = &cobra.Command{
	Use:   "remote-to-ssh",
	Short: "Change origin from HTTPS to SSH for all Git repos under a directory",
	Long: `Scan subdirectories under the given path; for each directory containing .git:
  If origin is currently an HTTPS URL, convert it to the corresponding SSH URL and run git remote set-url.
Example: https://github.com/go-lynx/lynx.git -> git@github.com:go-lynx/lynx.git`,
	Example: `  # Change remote to SSH for all repos in current directory
  lynx git remote-to-ssh

  # Specify directory to scan
  lynx git remote-to-ssh --dir ./go-lynx-repos`,
	RunE: runRemoteToSSH,
}

func init() {
	cmdRemoteToSSH.Flags().StringVarP(&remoteToSSHDir, "dir", "d", ".", "Directory to scan (containing repo subdirectories)")
}

// httpsURLToSSH converts GitHub/GitLab etc. HTTPS clone URLs to SSH.
// Example: https://github.com/owner/repo.git -> git@github.com:owner/repo.git
func httpsURLToSSH(httpsURL string) (sshURL string, ok bool) {
	s := strings.TrimSpace(httpsURL)
	if s == "" {
		return "", false
	}
	if strings.HasPrefix(s, "git@") && strings.Contains(s, ":") {
		return s, true // already SSH
	}
	// https://github.com/owner/repo or https://github.com/owner/repo.git
	if !strings.HasPrefix(s, "https://") {
		return "", false
	}
	s = strings.TrimPrefix(s, "https://")
	// Strip optional user:pass@
	if idx := strings.Index(s, "@"); idx != -1 {
		s = s[idx+1:]
	}
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return "", false
	}
	host, path := parts[0], parts[1]
	path = strings.TrimSuffix(path, ".git")
	if path == "" {
		return "", false
	}
	return fmt.Sprintf("git@%s:%s.git", host, path), true
}

func runRemoteToSSH(cmd *cobra.Command, args []string) error {
	absDir, err := filepath.Abs(remoteToSSHDir)
	if err != nil {
		return fmt.Errorf("resolve directory: %w", err)
	}
	fi, err := os.Stat(absDir)
	if err != nil {
		return fmt.Errorf("stat directory: %w", err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("not a directory: %s", absDir)
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}

	var updated, skipped, failed []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		repoDir := filepath.Join(absDir, e.Name())
		gitDir := filepath.Join(repoDir, ".git")
		if fi, err := os.Stat(gitDir); err != nil || !fi.IsDir() {
			continue
		}
		origin, err := getOriginURL(repoDir)
		if err != nil {
			color.Red("  %s: get origin failed: %v\n", e.Name(), err)
			failed = append(failed, e.Name())
			continue
		}
		sshURL, ok := httpsURLToSSH(origin)
		if !ok {
			color.Yellow("  %s: origin is already SSH or not HTTPS, skip: %s\n", e.Name(), origin)
			skipped = append(skipped, e.Name())
			continue
		}
		if sshURL == origin {
			skipped = append(skipped, e.Name())
			continue
		}
		if err := setOriginURL(repoDir, sshURL); err != nil {
			color.Red("  %s: set origin failed: %v\n", e.Name(), err)
			failed = append(failed, e.Name())
			continue
		}
		color.Green("  %s: %s -> %s\n", e.Name(), origin, sshURL)
		updated = append(updated, e.Name())
	}

	color.Cyan("remote-to-ssh done: updated %d, skipped %d, failed %d\n",
		len(updated), len(skipped), len(failed))
	if len(failed) > 0 {
		return fmt.Errorf("failed: %s", strings.Join(failed, ", "))
	}
	return nil
}

func getOriginURL(repoDir string) (string, error) {
	c := exec.Command("git", "remote", "get-url", "origin")
	c.Dir = repoDir
	out, err := c.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func setOriginURL(repoDir, url string) error {
	c := exec.Command("git", "remote", "set-url", "origin", url)
	c.Dir = repoDir
	var stderr bytes.Buffer
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.Bytes())
	}
	return nil
}

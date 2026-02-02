package git

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const (
	githubOrgReposURL = "https://api.github.com/orgs/go-lynx/repos"
	perPage           = 100
	httpTimeout       = 30 * time.Second
)

type repoInfo struct {
	Name     string `json:"name"`
	CloneURL string `json:"clone_url"`
	Private  bool   `json:"private"`
}

var (
	cloneAllDir string
)

var cmdCloneAll = &cobra.Command{
	Use:   "clone-all",
	Short: "Clone all public repositories from go-lynx organization",
	Long: `Fetch the list of all public repositories from https://github.com/orgs/go-lynx/repositories
and clone each of them into the current directory (or the directory specified by --dir).`,
	Example: `  # Clone all repos into current directory
  lynx git clone-all

  # Clone all repos into a specific directory
  lynx git clone-all --dir ./go-lynx-repos`,
	RunE: runCloneAll,
}

func init() {
	cmdCloneAll.Flags().StringVarP(&cloneAllDir, "dir", "d", ".", "Directory to clone repositories into")
}

func runCloneAll(cmd *cobra.Command, args []string) error {
	repos, err := fetchOrgRepos()
	if err != nil {
		return fmt.Errorf("fetch repos: %w", err)
	}

	if len(repos) == 0 {
		color.Yellow("No public repositories found in go-lynx organization.")
		return nil
	}

	absDir, err := filepath.Abs(cloneAllDir)
	if err != nil {
		return fmt.Errorf("resolve directory: %w", err)
	}
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", absDir, err)
	}

	color.Cyan("Cloning %d repositories from go-lynx into %s\n", len(repos), absDir)

	var failed []string
	for i, r := range repos {
		dest := filepath.Join(absDir, r.Name)
		if fi, err := os.Stat(dest); err == nil {
			if fi.IsDir() {
				color.Yellow("[%d/%d] %s already exists, skip\n", i+1, len(repos), r.Name)
				continue
			}
			color.Red("[%d/%d] %s exists but is not a directory, skip\n", i+1, len(repos), r.Name)
			failed = append(failed, r.Name)
			continue
		}
		if r.CloneURL == "" {
			color.Red("[%d/%d] %s has no clone_url, skip\n", i+1, len(repos), r.Name)
			failed = append(failed, r.Name)
			continue
		}
		color.Cyan("[%d/%d] Cloning %s ...\n", i+1, len(repos), r.Name)
		if err := cloneRepo(r.CloneURL, dest); err != nil {
			color.Red("  failed: %v\n", err)
			failed = append(failed, r.Name)
			continue
		}
		color.Green("  done\n")
	}

	if len(failed) > 0 {
		color.Red("\nFailed to clone: %s\n", strings.Join(failed, ", "))
		return fmt.Errorf("%d repo(s) failed to clone", len(failed))
	}
	color.Green("\nAll %d repositories cloned successfully.\n", len(repos))
	return nil
}

func fetchOrgRepos() ([]repoInfo, error) {
	var all []repoInfo
	page := 1
	for {
		url := fmt.Sprintf("%s?per_page=%d&page=%d&type=public", githubOrgReposURL, perPage, page)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		client := &http.Client{Timeout: httpTimeout}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("github api: %s %s", resp.Status, string(body))
		}

		var pageRepos []repoInfo
		if err := json.Unmarshal(body, &pageRepos); err != nil {
			return nil, err
		}
		for _, r := range pageRepos {
			if !r.Private {
				all = append(all, r)
			}
		}
		if len(pageRepos) < perPage {
			break
		}
		page++
	}
	return all, nil
}

func cloneRepo(cloneURL, dest string) error {
	cmd := exec.Command("git", "clone", cloneURL, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

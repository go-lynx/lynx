package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	githubOrgReposURL = "https://api.github.com/orgs/go-lynx/repos"
	pluginCacheDir    = ".lynx"
	pluginCacheFile   = "plugin-registry-cache.json"
	cacheTTL          = 1 * time.Hour
	httpTimeout       = 30 * time.Second
	perPage           = 100
)

// repoInfo matches GitHub API repo response (only needed fields)
type githubRepoInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CloneURL    string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private     bool   `json:"private"`
}

// pluginCacheEntry is the cached plugin list with timestamp
type pluginCacheEntry struct {
	Plugins   []*PluginMetadata `json:"plugins"`
	CachedAt  time.Time         `json:"cached_at"`
}

var (
	githubPluginCache     []*PluginMetadata
	githubPluginCacheMu   sync.RWMutex
	forceRefreshPluginList bool // when true, skip cache and always fetch from GitHub (set by list --no-cache)
)

// repoNameToType maps go-lynx repo name (without "lynx-" prefix) to PluginType
var repoNameToType = map[string]PluginType{
	"http": TypeService, "grpc": TypeService,
	"kafka": TypeMQ, "rabbitmq": TypeMQ, "rocketmq": TypeMQ, "pulsar": TypeMQ,
	"mysql": TypeSQL, "pgsql": TypeSQL, "mssql": TypeSQL, "sql-sdk": TypeSQL,
	"redis": TypeNoSQL, "redis-lock": TypeNoSQL, "mongodb": TypeNoSQL,
	"elasticsearch": TypeNoSQL, "etcd": TypeOther, "etcd-lock": TypeOther,
	"tracer": TypeTracer,
	"seata": TypeDTX, "dtm": TypeDTX,
	"nacos": TypeConfig, "apollo": TypeConfig, "polaris": TypeOther,
	"swagger": TypeOther, "sentinel": TypeOther, "eon-id": TypeOther,
}

// excludedRepos are org repos that are not installable plugins
var excludedRepos = map[string]bool{
	"lynx": true, "lynx-layout": true, "lynx.github.cn": true, ".github": true,
}

// SetForceRefreshPluginList sets whether to skip cache on next FetchPluginsFromGitHub (used by list --no-cache)
func SetForceRefreshPluginList(force bool) {
	forceRefreshPluginList = force
}

// FetchPluginsFromGitHub fetches the list of public plugin repos from go-lynx org and returns PluginMetadata slice.
// Results are cached in memory and optionally on disk (under projectRoot/.lynx/) with TTL.
func FetchPluginsFromGitHub(projectRoot string) ([]*PluginMetadata, error) {
	if !forceRefreshPluginList {
		githubPluginCacheMu.RLock()
		if len(githubPluginCache) > 0 {
			cached := githubPluginCache
			githubPluginCacheMu.RUnlock()
			return append([]*PluginMetadata(nil), cached...), nil
		}
		githubPluginCacheMu.RUnlock()

		if projectRoot != "" {
			cachePath := filepath.Join(projectRoot, pluginCacheDir, pluginCacheFile)
			if data, err := os.ReadFile(cachePath); err == nil {
				var entry pluginCacheEntry
				if json.Unmarshal(data, &entry) == nil && time.Since(entry.CachedAt) < cacheTTL && len(entry.Plugins) > 0 {
					githubPluginCacheMu.Lock()
					githubPluginCache = entry.Plugins
					githubPluginCacheMu.Unlock()
					return append([]*PluginMetadata(nil), entry.Plugins...), nil
				}
			}
		}
	}
	forceRefreshPluginList = false

	plugins, err := fetchFromGitHubAPI()
	if err != nil {
		return nil, err
	}

	githubPluginCacheMu.Lock()
	githubPluginCache = plugins
	githubPluginCacheMu.Unlock()

	// Persist cache
	if projectRoot != "" && len(plugins) > 0 {
		cachePath := filepath.Join(projectRoot, pluginCacheDir, pluginCacheFile)
		_ = os.MkdirAll(filepath.Dir(cachePath), 0755)
		entry := pluginCacheEntry{Plugins: plugins, CachedAt: time.Now()}
		if data, err := json.MarshalIndent(entry, "", "  "); err == nil {
			_ = os.WriteFile(cachePath, data, 0644)
		}
	}

	return append([]*PluginMetadata(nil), plugins...), nil
}

func fetchFromGitHubAPI() ([]*PluginMetadata, error) {
	client := &http.Client{Timeout: httpTimeout}
	var all []*PluginMetadata
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

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request github: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("github api %s: %s", resp.Status, string(body))
		}

		var pageRepos []githubRepoInfo
		if err := json.Unmarshal(body, &pageRepos); err != nil {
			return nil, err
		}

		for _, r := range pageRepos {
			if r.Private || excludedRepos[r.Name] {
				continue
			}
			if !strings.HasPrefix(r.Name, "lynx-") {
				continue
			}
			name := strings.TrimPrefix(r.Name, "lynx-")
			if name == "" {
				continue
			}
			pluginType := repoNameToType[name]
			if pluginType == "" {
				pluginType = TypeOther
			}
			desc := r.Description
			if desc == "" {
				desc = "Lynx plugin: " + r.Name
			}
			importPath := "github.com/go-lynx/" + r.Name
			if !strings.HasPrefix(r.CloneURL, "http") && !strings.HasPrefix(r.CloneURL, "git@") {
				r.CloneURL = "https://github.com/go-lynx/" + r.Name + ".git"
			}
			all = append(all, &PluginMetadata{
				Name:        name,
				Type:        pluginType,
				Version:     "latest",
				Description: desc,
				Repository:  r.CloneURL,
				ImportPath:  importPath,
				Author:      "go-lynx",
				License:     "Apache-2.0",
				Tags:        []string{name},
				Status:      StatusNotInstalled,
				Official:    true,
			})
		}

		if len(pageRepos) < perPage {
			break
		}
		page++
	}

	return all, nil
}

// InvalidatePluginCache clears the in-memory plugin cache (e.g. after install from URL).
func InvalidatePluginCache() {
	githubPluginCacheMu.Lock()
	githubPluginCache = nil
	githubPluginCacheMu.Unlock()
}

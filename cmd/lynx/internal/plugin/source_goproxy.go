package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	goproxyDefault   = "https://proxy.golang.org"
	goproxyTimeout   = 15 * time.Second
	goproxyMaxConcur = 5
)

// goproxyLatestResponse is the JSON response from GET $GOPROXY/<module>/@latest
type goproxyLatestResponse struct {
	Version string `json:"Version"`
}

// getGoproxyBase returns the first proxy from GOPROXY (e.g. https://proxy.golang.org).
func getGoproxyBase() string {
	v := os.Getenv("GOPROXY")
	v = strings.TrimSpace(v)
	if v == "" {
		return goproxyDefault
	}
	// GOPROXY can be "direct" or "https://proxy.golang.org,direct" etc.
	for _, s := range strings.Split(v, ",") {
		s = strings.TrimSpace(s)
		if s == "" || s == "off" || s == "direct" {
			continue
		}
		return s
	}
	return goproxyDefault
}

// FetchLatestVersion fetches the latest module version from the Go module proxy.
// importPath is the module path (e.g. github.com/go-lynx/lynx-http).
// Returns the version string (e.g. v1.5.2) or empty string on failure.
func FetchLatestVersion(importPath string) (string, error) {
	if importPath == "" {
		return "", fmt.Errorf("empty import path")
	}
	base := strings.TrimSuffix(getGoproxyBase(), "/")
	// URL path: /modulePath/@latest (no encoding needed for standard paths)
	u := base + "/" + importPath + "/@latest"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: goproxyTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("proxy %s: %s", resp.Status, string(body))
	}
	var out goproxyLatestResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Version, nil
}

// EnrichPluginsLatestVersion fetches the latest version for each plugin from the Go proxy
// and sets PluginMetadata.Version. Uses limited concurrency. Failed plugins keep their current Version.
func EnrichPluginsLatestVersion(plugins []*PluginMetadata) {
	if len(plugins) == 0 {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, goproxyMaxConcur)
	for _, p := range plugins {
		if p.ImportPath == "" {
			continue
		}
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ver, err := FetchLatestVersion(p.ImportPath)
			if err == nil && ver != "" {
				p.Version = ver
			}
		}()
	}
	wg.Wait()
}

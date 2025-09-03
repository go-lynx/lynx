package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// HealthCheck represents a diagnostic check
type HealthCheck interface {
	Name() string
	Category() string
	Check() CheckResult
	Fix() error
	CanAutoFix() bool
}

// BaseCheck provides common functionality for all checks
type BaseCheck struct {
	name     string
	category string
}

func (b *BaseCheck) Name() string {
	return b.name
}

func (b *BaseCheck) Category() string {
	return b.category
}

func (b *BaseCheck) CanAutoFix() bool {
	return false
}

func (b *BaseCheck) Fix() error {
	return fmt.Errorf("auto-fix not implemented for %s", b.name)
}

// Environment Checks

// GoVersionCheck checks if Go is installed and version is compatible
type GoVersionCheck struct {
	BaseCheck
}

func NewGoVersionCheck() *GoVersionCheck {
	return &GoVersionCheck{
		BaseCheck: BaseCheck{
			name:     "Go Version",
			category: "environment",
		},
	}
}

func (c *GoVersionCheck) Check() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:         c.Name(),
		Category:     c.Category(),
		FixAvailable: false,
		Duration:     time.Since(start),
		Details:      make(map[string]interface{}),
	}

	// Check if Go is installed
	goPath, err := exec.LookPath("go")
	if err != nil {
		result.Status = StatusError
		result.Message = "Go is not installed or not in PATH"
		return result
	}
	result.Details["go_path"] = goPath

	// Get Go version
	cmd := exec.Command("go", "version")
	output, err := cmd.Output()
	if err != nil {
		result.Status = StatusError
		result.Message = "Failed to get Go version"
		result.Details["error"] = err.Error()
		return result
	}

	versionStr := string(output)
	result.Details["version_output"] = strings.TrimSpace(versionStr)

	// Parse version
	re := regexp.MustCompile(`go(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(versionStr)
	if len(matches) < 3 {
		result.Status = StatusWarning
		result.Message = "Could not parse Go version"
		return result
	}

	major := matches[1]
	minor := matches[2]
	version := fmt.Sprintf("%s.%s", major, minor)
	result.Details["parsed_version"] = version

	// Check minimum version (Go 1.20+)
	if major == "1" && minor < "20" {
		result.Status = StatusWarning
		result.Message = fmt.Sprintf("Go version %s is below recommended 1.20+", version)
		result.FixAvailable = true
	} else {
		result.Status = StatusOK
		result.Message = fmt.Sprintf("Go %s installed", version)
	}

	result.Duration = time.Since(start)
	return result
}

// GoEnvCheck checks Go environment variables
type GoEnvCheck struct {
	BaseCheck
}

func NewGoEnvCheck() *GoEnvCheck {
	return &GoEnvCheck{
		BaseCheck: BaseCheck{
			name:     "Go Environment",
			category: "environment",
		},
	}
}

func (c *GoEnvCheck) Check() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:     c.Name(),
		Category: c.Category(),
		Details:  make(map[string]interface{}),
	}

	// Check GOPATH
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		cmd := exec.Command("go", "env", "GOPATH")
		output, _ := cmd.Output()
		gopath = strings.TrimSpace(string(output))
	}
	result.Details["GOPATH"] = gopath

	// Check GO111MODULE
	go111module := os.Getenv("GO111MODULE")
	if go111module == "" {
		cmd := exec.Command("go", "env", "GO111MODULE")
		output, _ := cmd.Output()
		go111module = strings.TrimSpace(string(output))
	}
	result.Details["GO111MODULE"] = go111module

	// Check GOPROXY
	goproxy := os.Getenv("GOPROXY")
	if goproxy == "" {
		cmd := exec.Command("go", "env", "GOPROXY")
		output, _ := cmd.Output()
		goproxy = strings.TrimSpace(string(output))
	}
	result.Details["GOPROXY"] = goproxy

	result.Status = StatusOK
	result.Message = "Go environment variables configured"
	result.Duration = time.Since(start)
	return result
}

// Tool Checks

// ProtocCheck checks if protoc is installed
type ProtocCheck struct {
	BaseCheck
}

func NewProtocCheck() *ProtocCheck {
	return &ProtocCheck{
		BaseCheck: BaseCheck{
			name:     "Protocol Buffers Compiler",
			category: "tools",
		},
	}
}

func (c *ProtocCheck) Check() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:         c.Name(),
		Category:     c.Category(),
		FixAvailable: true,
		Details:      make(map[string]interface{}),
	}

	// Check if protoc is installed
	protocPath, err := exec.LookPath("protoc")
	if err != nil {
		result.Status = StatusError
		result.Message = "protoc is not installed"
		result.Details["suggestion"] = "Run 'make init' to install required tools"
		result.Duration = time.Since(start)
		return result
	}
	result.Details["protoc_path"] = protocPath

	// Get protoc version
	cmd := exec.Command("protoc", "--version")
	output, err := cmd.Output()
	if err != nil {
		result.Status = StatusWarning
		result.Message = "protoc found but could not get version"
	} else {
		version := strings.TrimSpace(string(output))
		result.Details["version"] = version
		result.Status = StatusOK
		result.Message = fmt.Sprintf("protoc installed: %s", version)
	}

	result.Duration = time.Since(start)
	return result
}

func (c *ProtocCheck) CanAutoFix() bool {
	return true
}

func (c *ProtocCheck) Fix() error {
	// Try to install protoc using make init
	cmd := exec.Command("make", "init")
	return cmd.Run()
}

// WireCheck checks if Wire is installed
type WireCheck struct {
	BaseCheck
}

func NewWireCheck() *WireCheck {
	return &WireCheck{
		BaseCheck: BaseCheck{
			name:     "Wire Dependency Injection",
			category: "tools",
		},
	}
}

func (c *WireCheck) Check() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:         c.Name(),
		Category:     c.Category(),
		FixAvailable: true,
		Details:      make(map[string]interface{}),
	}

	// Check if wire is installed
	wirePath, err := exec.LookPath("wire")
	if err != nil {
		// Check in GOPATH/bin
		gopath := os.Getenv("GOPATH")
		if gopath != "" {
			wirePath = filepath.Join(gopath, "bin", "wire")
			if runtime.GOOS == "windows" {
				wirePath += ".exe"
			}
			if _, err := os.Stat(wirePath); err != nil {
				result.Status = StatusWarning
				result.Message = "Wire is not installed"
				result.Details["suggestion"] = "Run 'go install github.com/google/wire/cmd/wire@latest'"
				result.Duration = time.Since(start)
				return result
			}
		} else {
			result.Status = StatusWarning
			result.Message = "Wire is not installed"
			result.Details["suggestion"] = "Run 'go install github.com/google/wire/cmd/wire@latest'"
			result.Duration = time.Since(start)
			return result
		}
	}
	result.Details["wire_path"] = wirePath

	result.Status = StatusOK
	result.Message = "Wire installed"
	result.Duration = time.Since(start)
	return result
}

func (c *WireCheck) CanAutoFix() bool {
	return true
}

func (c *WireCheck) Fix() error {
	cmd := exec.Command("go", "install", "github.com/google/wire/cmd/wire@latest")
	return cmd.Run()
}

// Project Checks

// ProjectStructureCheck checks if the project has the expected structure
type ProjectStructureCheck struct {
	BaseCheck
}

func NewProjectStructureCheck() *ProjectStructureCheck {
	return &ProjectStructureCheck{
		BaseCheck: BaseCheck{
			name:     "Project Structure",
			category: "project",
		},
	}
}

func (c *ProjectStructureCheck) Check() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:     c.Name(),
		Category: c.Category(),
		Details:  make(map[string]interface{}),
	}

	// Expected directories
	expectedDirs := []string{
		"app",
		"boot",
		"plugins",
		"cmd",
		"docs",
		"examples",
	}

	missingDirs := []string{}
	foundDirs := []string{}

	for _, dir := range expectedDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			foundDirs = append(foundDirs, dir)
		} else {
			missingDirs = append(missingDirs, dir)
		}
	}

	result.Details["expected"] = expectedDirs
	result.Details["found"] = foundDirs
	result.Details["missing"] = missingDirs

	if len(missingDirs) == 0 {
		result.Status = StatusOK
		result.Message = "All expected directories found"
	} else if len(missingDirs) <= 2 {
		result.Status = StatusWarning
		result.Message = fmt.Sprintf("Missing %d directories: %s", len(missingDirs), strings.Join(missingDirs, ", "))
	} else {
		result.Status = StatusError
		result.Message = fmt.Sprintf("Missing %d directories, project structure may be incorrect", len(missingDirs))
	}

	result.Duration = time.Since(start)
	return result
}

// GoModCheck checks if go.mod exists and is valid
type GoModCheck struct {
	BaseCheck
}

func NewGoModCheck() *GoModCheck {
	return &GoModCheck{
		BaseCheck: BaseCheck{
			name:     "Go Modules",
			category: "project",
		},
	}
}

func (c *GoModCheck) Check() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:         c.Name(),
		Category:     c.Category(),
		FixAvailable: true,
		Details:      make(map[string]interface{}),
	}

	// Check if go.mod exists
	if _, err := os.Stat("go.mod"); err != nil {
		result.Status = StatusError
		result.Message = "go.mod not found"
		result.Details["suggestion"] = "Run 'go mod init' to create go.mod"
		result.Duration = time.Since(start)
		return result
	}

	// Read go.mod
	content, err := os.ReadFile("go.mod")
	if err != nil {
		result.Status = StatusError
		result.Message = "Failed to read go.mod"
		result.Details["error"] = err.Error()
		result.Duration = time.Since(start)
		return result
	}

	// Parse module name
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "module ") {
			moduleName := strings.TrimPrefix(line, "module ")
			moduleName = strings.TrimSpace(moduleName)
			result.Details["module"] = moduleName
			
			if moduleName == "github.com/go-lynx/lynx" || strings.HasPrefix(moduleName, "github.com/go-lynx/") {
				result.Status = StatusOK
				result.Message = fmt.Sprintf("Valid go.mod with module: %s", moduleName)
			} else {
				result.Status = StatusWarning
				result.Message = fmt.Sprintf("go.mod found with module: %s", moduleName)
			}
			break
		}
	}

	// Check go version
	for _, line := range lines {
		if strings.HasPrefix(line, "go ") {
			goVersion := strings.TrimPrefix(line, "go ")
			goVersion = strings.TrimSpace(goVersion)
			result.Details["go_version"] = goVersion
			break
		}
	}

	result.Duration = time.Since(start)
	return result
}

func (c *GoModCheck) CanAutoFix() bool {
	return true
}

func (c *GoModCheck) Fix() error {
	// Run go mod tidy
	cmd := exec.Command("go", "mod", "tidy")
	return cmd.Run()
}

// Config Checks

// ConfigFileCheck checks if configuration files are valid
type ConfigFileCheck struct {
	BaseCheck
}

func NewConfigFileCheck() *ConfigFileCheck {
	return &ConfigFileCheck{
		BaseCheck: BaseCheck{
			name:     "Configuration Files",
			category: "config",
		},
	}
}

func (c *ConfigFileCheck) Check() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:     c.Name(),
		Category: c.Category(),
		Details:  make(map[string]interface{}),
	}

	configFiles := []string{}
	invalidFiles := []string{}

	// Find all YAML config files
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		// Skip vendor and hidden directories
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor") {
			return filepath.SkipDir
		}
		
		if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
			if strings.Contains(path, "conf") || strings.Contains(path, "config") {
				configFiles = append(configFiles, path)
				
				// Try to parse the YAML file
				content, err := os.ReadFile(path)
				if err != nil {
					invalidFiles = append(invalidFiles, path)
				} else {
					var data interface{}
					if err := yaml.Unmarshal(content, &data); err != nil {
						invalidFiles = append(invalidFiles, path)
					}
				}
			}
		}
		return nil
	})

	if err != nil {
		result.Status = StatusWarning
		result.Message = "Failed to scan for config files"
		result.Details["error"] = err.Error()
	} else {
		result.Details["config_files"] = configFiles
		result.Details["invalid_files"] = invalidFiles
		
		if len(invalidFiles) > 0 {
			result.Status = StatusError
			result.Message = fmt.Sprintf("Found %d invalid config files", len(invalidFiles))
		} else if len(configFiles) == 0 {
			result.Status = StatusWarning
			result.Message = "No configuration files found"
		} else {
			result.Status = StatusOK
			result.Message = fmt.Sprintf("Found %d valid config files", len(configFiles))
		}
	}

	result.Duration = time.Since(start)
	return result
}

// MakefileCheck checks if Makefile exists and has expected targets
type MakefileCheck struct {
	BaseCheck
}

func NewMakefileCheck() *MakefileCheck {
	return &MakefileCheck{
		BaseCheck: BaseCheck{
			name:     "Makefile",
			category: "project",
		},
	}
}

func (c *MakefileCheck) Check() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:     c.Name(),
		Category: c.Category(),
		Details:  make(map[string]interface{}),
	}

	// Check if Makefile exists
	if _, err := os.Stat("Makefile"); err != nil {
		result.Status = StatusWarning
		result.Message = "Makefile not found"
		result.Duration = time.Since(start)
		return result
	}

	// Read Makefile
	content, err := os.ReadFile("Makefile")
	if err != nil {
		result.Status = StatusError
		result.Message = "Failed to read Makefile"
		result.Details["error"] = err.Error()
		result.Duration = time.Since(start)
		return result
	}

	// Check for expected targets
	expectedTargets := []string{"init", "config", "help", "release"}
	foundTargets := []string{}
	missingTargets := []string{}

	makefileContent := string(content)
	for _, target := range expectedTargets {
		if strings.Contains(makefileContent, target+":") {
			foundTargets = append(foundTargets, target)
		} else {
			missingTargets = append(missingTargets, target)
		}
	}

	result.Details["expected_targets"] = expectedTargets
	result.Details["found_targets"] = foundTargets
	result.Details["missing_targets"] = missingTargets

	if len(missingTargets) == 0 {
		result.Status = StatusOK
		result.Message = "Makefile has all expected targets"
	} else {
		result.Status = StatusWarning
		result.Message = fmt.Sprintf("Makefile missing targets: %s", strings.Join(missingTargets, ", "))
	}

	result.Duration = time.Since(start)
	return result
}

// GitCheck checks Git repository status
type GitCheck struct {
	BaseCheck
}

func NewGitCheck() *GitCheck {
	return &GitCheck{
		BaseCheck: BaseCheck{
			name:     "Git Repository",
			category: "environment",
		},
	}
}

func (c *GitCheck) Check() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:     c.Name(),
		Category: c.Category(),
		Details:  make(map[string]interface{}),
	}

	// Check if git is installed
	if _, err := exec.LookPath("git"); err != nil {
		result.Status = StatusWarning
		result.Message = "Git is not installed"
		result.Duration = time.Since(start)
		return result
	}

	// Check if current directory is a git repo
	if _, err := os.Stat(".git"); err != nil {
		result.Status = StatusWarning
		result.Message = "Not a Git repository"
		result.FixAvailable = true
		result.Duration = time.Since(start)
		return result
	}

	// Get git status
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		result.Status = StatusError
		result.Message = "Failed to get Git status"
		result.Details["error"] = err.Error()
	} else {
		modifiedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(modifiedFiles) > 0 && modifiedFiles[0] != "" {
			result.Details["modified_files"] = len(modifiedFiles)
			result.Status = StatusWarning
			result.Message = fmt.Sprintf("Git repository has %d uncommitted changes", len(modifiedFiles))
		} else {
			result.Status = StatusOK
			result.Message = "Git repository is clean"
		}
	}

	// Get current branch
	cmd = exec.Command("git", "branch", "--show-current")
	if output, err := cmd.Output(); err == nil {
		branch := strings.TrimSpace(string(output))
		result.Details["current_branch"] = branch
	}

	result.Duration = time.Since(start)
	return result
}
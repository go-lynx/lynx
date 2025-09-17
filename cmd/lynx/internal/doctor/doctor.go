package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// CmdDoctor represents the doctor command
var CmdDoctor = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose the development environment and project health",
	Long: `The doctor command performs comprehensive health checks on your development 
environment and Lynx project. It verifies Go installation, required tools, 
project structure, and configuration validity.`,
	Example: `  # Run all diagnostic checks
  lynx doctor
  
  # Output results in JSON format
  lynx doctor --format json
  
  # Run specific category of checks
  lynx doctor --category env
  lynx doctor --category tools
  lynx doctor --category project
  
  # Auto-fix issues if possible
  lynx doctor --fix`,
	RunE: runDoctor,
}

var (
	outputFormat string
	fixIssues    bool
	category     string
	verbose      bool
)

func init() {
	CmdDoctor.Flags().StringVarP(&outputFormat, "format", "f", "text", "Output format (text/json/markdown)")
	CmdDoctor.Flags().BoolVar(&fixIssues, "fix", false, "Attempt to automatically fix issues")
	CmdDoctor.Flags().StringVarP(&category, "category", "c", "all", "Check category (all/env/tools/project/config)")
	CmdDoctor.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed diagnostic information")
}

// DiagnosticReport represents the complete diagnostic report
type DiagnosticReport struct {
	Timestamp   time.Time     `json:"timestamp"`
	System      SystemInfo    `json:"system"`
	Checks      []CheckResult `json:"checks"`
	Summary     Summary       `json:"summary"`
	FixesApplied []string     `json:"fixes_applied,omitempty"`
}

// SystemInfo contains system information
type SystemInfo struct {
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	GoVersion    string `json:"go_version"`
	NumCPU       int    `json:"num_cpu"`
	LynxVersion  string `json:"lynx_version"`
}

// CheckResult represents a single check result
type CheckResult struct {
	Name        string                 `json:"name"`
	Category    string                 `json:"category"`
	Status      Status                 `json:"status"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	FixAvailable bool                  `json:"fix_available"`
	FixApplied   bool                  `json:"fix_applied,omitempty"`
	Duration    time.Duration          `json:"duration"`
}

// Status represents the check status
type Status string

const (
	StatusOK      Status = "OK"
	StatusWarning Status = "WARNING"
	StatusError   Status = "ERROR"
	StatusSkipped Status = "SKIPPED"
)

// Summary provides overall diagnostic summary
type Summary struct {
	TotalChecks int           `json:"total_checks"`
	Passed      int           `json:"passed"`
	Warnings    int           `json:"warnings"`
	Errors      int           `json:"errors"`
	Skipped     int           `json:"skipped"`
	Duration    time.Duration `json:"duration"`
	Health      string        `json:"health"` // Healthy, Degraded, Critical
}

// runDoctor executes the diagnostic checks
func runDoctor(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	
	// Initialize diagnostic runner
	runner := NewDiagnosticRunner()
	
	// Configure runner based on flags
	runner.SetVerbose(verbose)
	runner.SetAutoFix(fixIssues)
	
	// Select checks based on category
	var checks []HealthCheck
	switch category {
	case "all":
		checks = runner.GetAllChecks()
	case "env":
		checks = runner.GetEnvironmentChecks()
	case "tools":
		checks = runner.GetToolsChecks()
	case "project":
		checks = runner.GetProjectChecks()
	case "config":
		checks = runner.GetConfigChecks()
	default:
		return fmt.Errorf("invalid category: %s", category)
	}
	
	// Run diagnostics
	results := runner.RunChecks(checks)
	
	// Generate report
	report := DiagnosticReport{
		Timestamp: startTime,
		System:    getSystemInfo(),
		Checks:    results,
		Summary:   calculateSummary(results, time.Since(startTime)),
	}
	
	if fixIssues {
		report.FixesApplied = runner.GetAppliedFixes()
	}
	
	// Output report
	return outputReport(report, outputFormat)
}

// getSystemInfo collects system information
func getSystemInfo() SystemInfo {
	return SystemInfo{
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		GoVersion:   runtime.Version(),
		NumCPU:      runtime.NumCPU(),
		LynxVersion: getLynxVersion(),
	}
}

// getLynxVersion gets the Lynx CLI version dynamically
func getLynxVersion() string {
	// Try to get version from build info first
	if info, ok := debug.ReadBuildInfo(); ok {
		// Check if this is the main module
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
		
		// Look for version in build settings
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && len(setting.Value) >= 7 {
				return "dev-" + setting.Value[:7] // Short commit hash
			}
		}
	}
	
	// Try to get version from git if available
	if version := getVersionFromGit(); version != "" {
		return version
	}
	
	// Try to read version from version file
	if version := getVersionFromFile(); version != "" {
		return version
	}
	
	// Fallback to default version
	return "v2.0.0-unknown"
}

// getVersionFromGit attempts to get version from git
func getVersionFromGit() string {
	// Try to get the latest git tag
	if cmd := exec.Command("git", "describe", "--tags", "--abbrev=0"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			return strings.TrimSpace(string(output))
		}
	}
	
	// Try to get current commit hash
	if cmd := exec.Command("git", "rev-parse", "--short", "HEAD"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			return "dev-" + strings.TrimSpace(string(output))
		}
	}
	
	return ""
}

// getVersionFromFile attempts to read version from a version file
func getVersionFromFile() string {
	// Look for version files in common locations
	versionFiles := []string{
		"VERSION",
		"version.txt", 
		".version",
		"cmd/lynx/VERSION",
	}
	
	for _, versionFile := range versionFiles {
		if data, err := os.ReadFile(versionFile); err == nil {
			version := strings.TrimSpace(string(data))
			if version != "" {
				return version
			}
		}
		
		// Try relative to executable path
		if execPath, err := os.Executable(); err == nil {
			execDir := filepath.Dir(execPath)
			fullPath := filepath.Join(execDir, versionFile)
			if data, err := os.ReadFile(fullPath); err == nil {
				version := strings.TrimSpace(string(data))
				if version != "" {
					return version
				}
			}
		}
	}
	
	return ""
}

// calculateSummary calculates the diagnostic summary
func calculateSummary(results []CheckResult, duration time.Duration) Summary {
	summary := Summary{
		TotalChecks: len(results),
		Duration:    duration,
	}
	
	for _, result := range results {
		switch result.Status {
		case StatusOK:
			summary.Passed++
		case StatusWarning:
			summary.Warnings++
		case StatusError:
			summary.Errors++
		case StatusSkipped:
			summary.Skipped++
		}
	}
	
	// Determine overall health
	if summary.Errors > 0 {
		summary.Health = "Critical"
	} else if summary.Warnings > 0 {
		summary.Health = "Degraded"
	} else {
		summary.Health = "Healthy"
	}
	
	return summary
}

// outputReport outputs the diagnostic report in the specified format
func outputReport(report DiagnosticReport, format string) error {
	switch format {
	case "json":
		return outputJSON(report)
	case "markdown":
		return outputMarkdown(report)
	default:
		return outputText(report)
	}
}

// outputJSON outputs report in JSON format
func outputJSON(report DiagnosticReport) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// outputText outputs report in human-readable text format
func outputText(report DiagnosticReport) error {
	// Header
	fmt.Println("\n🔍 Lynx Doctor - Diagnostic Report")
	fmt.Println(strings.Repeat("=", 50))
	
	// System Info
	fmt.Printf("\n📊 System Information:\n")
	fmt.Printf("  • OS/Arch: %s/%s\n", report.System.OS, report.System.Arch)
	fmt.Printf("  • Go Version: %s\n", report.System.GoVersion)
	fmt.Printf("  • Lynx Version: %s\n", report.System.LynxVersion)
	fmt.Printf("  • CPUs: %d\n", report.System.NumCPU)
	
	// Checks Results
	fmt.Printf("\n🔎 Diagnostic Checks:\n")
	fmt.Println(strings.Repeat("-", 50))
	
	// Group checks by category
	checksByCategory := groupChecksByCategory(report.Checks)
	
	for category, checks := range checksByCategory {
		fmt.Printf("\n📁 %s:\n", strings.Title(category))
		for _, check := range checks {
			statusIcon := getStatusIcon(check.Status)
			fmt.Printf("  %s %s: %s\n", statusIcon, check.Name, check.Message)
			
			if verbose && len(check.Details) > 0 {
				for key, value := range check.Details {
					fmt.Printf("      %s: %v\n", key, value)
				}
			}
			
			if check.FixAvailable && !check.FixApplied {
				fmt.Printf("      💡 Fix available (use --fix to apply)\n")
			} else if check.FixApplied {
				fmt.Printf("      ✅ Fix applied\n")
			}
		}
	}
	
	// Summary
	fmt.Printf("\n📈 Summary:\n")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("  Total Checks: %d\n", report.Summary.TotalChecks)
	fmt.Printf("  ✅ Passed: %d\n", report.Summary.Passed)
	if report.Summary.Warnings > 0 {
		fmt.Printf("  ⚠️  Warnings: %d\n", report.Summary.Warnings)
	}
	if report.Summary.Errors > 0 {
		fmt.Printf("  ❌ Errors: %d\n", report.Summary.Errors)
	}
	if report.Summary.Skipped > 0 {
		fmt.Printf("  ⏭️  Skipped: %d\n", report.Summary.Skipped)
	}
	fmt.Printf("  ⏱️  Duration: %v\n", report.Summary.Duration)
	
	// Overall Health
	healthIcon := getHealthIcon(report.Summary.Health)
	fmt.Printf("\n%s Overall Health: %s\n", healthIcon, report.Summary.Health)
	
	// Applied Fixes
	if len(report.FixesApplied) > 0 {
		fmt.Printf("\n🔧 Fixes Applied:\n")
		for _, fix := range report.FixesApplied {
			fmt.Printf("  • %s\n", fix)
		}
	}
	
	// Recommendations
	if report.Summary.Health != "Healthy" {
		fmt.Printf("\n💡 Recommendations:\n")
		printRecommendations(report)
	}
	
	fmt.Println()
	return nil
}

// outputMarkdown outputs report in Markdown format
func outputMarkdown(report DiagnosticReport) error {
	fmt.Printf("# Lynx Doctor - Diagnostic Report\n\n")
	fmt.Printf("**Generated:** %s\n\n", report.Timestamp.Format(time.RFC3339))
	
	// System Info
	fmt.Printf("## System Information\n\n")
	fmt.Printf("| Property | Value |\n")
	fmt.Printf("|----------|-------|\n")
	fmt.Printf("| OS/Arch | %s/%s |\n", report.System.OS, report.System.Arch)
	fmt.Printf("| Go Version | %s |\n", report.System.GoVersion)
	fmt.Printf("| Lynx Version | %s |\n", report.System.LynxVersion)
	fmt.Printf("| CPUs | %d |\n\n", report.System.NumCPU)
	
	// Checks Results
	fmt.Printf("## Diagnostic Checks\n\n")
	
	checksByCategory := groupChecksByCategory(report.Checks)
	for category, checks := range checksByCategory {
		fmt.Printf("### %s\n\n", strings.Title(category))
		fmt.Printf("| Check | Status | Message |\n")
		fmt.Printf("|-------|--------|---------||\n")
		
		for _, check := range checks {
			status := string(check.Status)
			if check.Status == StatusOK {
				status = "✅ " + status
			} else if check.Status == StatusWarning {
				status = "⚠️ " + status
			} else if check.Status == StatusError {
				status = "❌ " + status
			}
			fmt.Printf("| %s | %s | %s |\n", check.Name, status, check.Message)
		}
		fmt.Println()
	}
	
	// Summary
	fmt.Printf("## Summary\n\n")
	fmt.Printf("- **Total Checks:** %d\n", report.Summary.TotalChecks)
	fmt.Printf("- **Passed:** %d\n", report.Summary.Passed)
	fmt.Printf("- **Warnings:** %d\n", report.Summary.Warnings)
	fmt.Printf("- **Errors:** %d\n", report.Summary.Errors)
	fmt.Printf("- **Duration:** %v\n", report.Summary.Duration)
	fmt.Printf("- **Overall Health:** %s\n\n", report.Summary.Health)
	
	return nil
}

// Helper functions

func getStatusIcon(status Status) string {
	switch status {
	case StatusOK:
		return "✅"
	case StatusWarning:
		return "⚠️"
	case StatusError:
		return "❌"
	case StatusSkipped:
		return "⏭️"
	default:
		return "❓"
	}
}

func getHealthIcon(health string) string {
	switch health {
	case "Healthy":
		return "💚"
	case "Degraded":
		return "💛"
	case "Critical":
		return "🔴"
	default:
		return "❓"
	}
}

func groupChecksByCategory(checks []CheckResult) map[string][]CheckResult {
	grouped := make(map[string][]CheckResult)
	for _, check := range checks {
		grouped[check.Category] = append(grouped[check.Category], check)
	}
	return grouped
}

func printRecommendations(report DiagnosticReport) {
	for _, check := range report.Checks {
		if check.Status == StatusError {
			fmt.Printf("  • Fix %s: %s\n", check.Name, getRecommendation(check))
		}
	}
	for _, check := range report.Checks {
		if check.Status == StatusWarning {
			fmt.Printf("  • Consider fixing %s: %s\n", check.Name, getRecommendation(check))
		}
	}
}

func getRecommendation(check CheckResult) string {
	// This would provide specific recommendations based on the check
	// For now, return a generic message
	if check.FixAvailable {
		return "Run 'lynx doctor --fix' to automatically resolve"
	}
	return "Manual intervention required"
}
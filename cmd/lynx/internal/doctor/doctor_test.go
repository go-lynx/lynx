package doctor

import (
	"testing"
	"time"
)

func TestCheckResult(t *testing.T) {
	tests := []struct {
		name     string
		result   CheckResult
		expected Status
	}{
		{
			name: "OK status",
			result: CheckResult{
				Name:     "Test Check",
				Category: "test",
				Status:   StatusOK,
				Message:  "Everything is fine",
			},
			expected: StatusOK,
		},
		{
			name: "Error status",
			result: CheckResult{
				Name:     "Test Check",
				Category: "test",
				Status:   StatusError,
				Message:  "Something went wrong",
			},
			expected: StatusError,
		},
		{
			name: "Warning status",
			result: CheckResult{
				Name:     "Test Check",
				Category: "test",
				Status:   StatusWarning,
				Message:  "Minor issue detected",
			},
			expected: StatusWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.Status != tt.expected {
				t.Errorf("Expected status %v, got %v", tt.expected, tt.result.Status)
			}
		})
	}
}

func TestCalculateSummary(t *testing.T) {
	results := []CheckResult{
		{Status: StatusOK},
		{Status: StatusOK},
		{Status: StatusWarning},
		{Status: StatusError},
		{Status: StatusSkipped},
	}

	summary := calculateSummary(results, 1*time.Second)

	if summary.TotalChecks != 5 {
		t.Errorf("Expected 5 total checks, got %d", summary.TotalChecks)
	}

	if summary.Passed != 2 {
		t.Errorf("Expected 2 passed checks, got %d", summary.Passed)
	}

	if summary.Warnings != 1 {
		t.Errorf("Expected 1 warning, got %d", summary.Warnings)
	}

	if summary.Errors != 1 {
		t.Errorf("Expected 1 error, got %d", summary.Errors)
	}

	if summary.Skipped != 1 {
		t.Errorf("Expected 1 skipped check, got %d", summary.Skipped)
	}

	if summary.Health != "Critical" {
		t.Errorf("Expected health status 'Critical', got %s", summary.Health)
	}
}

func TestGetStatusIcon(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusOK, "✅"},
		{StatusWarning, "⚠️"},
		{StatusError, "❌"},
		{StatusSkipped, "⏭️"},
		{Status("unknown"), "❓"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			icon := getStatusIcon(tt.status)
			if icon != tt.expected {
				t.Errorf("Expected icon %s, got %s", tt.expected, icon)
			}
		})
	}
}

func TestGetHealthIcon(t *testing.T) {
	tests := []struct {
		health   string
		expected string
	}{
		{"Healthy", "💚"},
		{"Degraded", "💛"},
		{"Critical", "🔴"},
		{"Unknown", "❓"},
	}

	for _, tt := range tests {
		t.Run(tt.health, func(t *testing.T) {
			icon := getHealthIcon(tt.health)
			if icon != tt.expected {
				t.Errorf("Expected icon %s, got %s", tt.expected, icon)
			}
		})
	}
}

func TestGroupChecksByCategory(t *testing.T) {
	checks := []CheckResult{
		{Category: "env", Name: "Check1"},
		{Category: "env", Name: "Check2"},
		{Category: "tools", Name: "Check3"},
		{Category: "project", Name: "Check4"},
	}

	grouped := groupChecksByCategory(checks)

	if len(grouped) != 3 {
		t.Errorf("Expected 3 categories, got %d", len(grouped))
	}

	if len(grouped["env"]) != 2 {
		t.Errorf("Expected 2 env checks, got %d", len(grouped["env"]))
	}

	if len(grouped["tools"]) != 1 {
		t.Errorf("Expected 1 tools check, got %d", len(grouped["tools"]))
	}

	if len(grouped["project"]) != 1 {
		t.Errorf("Expected 1 project check, got %d", len(grouped["project"]))
	}
}

func TestDiagnosticRunner(t *testing.T) {
	runner := NewDiagnosticRunner()

	// Test verbose setting
	runner.SetVerbose(true)
	if !runner.verbose {
		t.Error("Expected verbose to be true")
	}

	// Test auto-fix setting
	runner.SetAutoFix(true)
	if !runner.autoFix {
		t.Error("Expected autoFix to be true")
	}

	// Test getting checks
	envChecks := runner.GetEnvironmentChecks()
	if len(envChecks) == 0 {
		t.Error("Expected at least one environment check")
	}

	toolsChecks := runner.GetToolsChecks()
	if len(toolsChecks) == 0 {
		t.Error("Expected at least one tools check")
	}

	projectChecks := runner.GetProjectChecks()
	if len(projectChecks) == 0 {
		t.Error("Expected at least one project check")
	}

	configChecks := runner.GetConfigChecks()
	if len(configChecks) == 0 {
		t.Error("Expected at least one config check")
	}

	allChecks := runner.GetAllChecks()
	expectedTotal := len(envChecks) + len(toolsChecks) + len(projectChecks) + len(configChecks)
	if len(allChecks) != expectedTotal {
		t.Errorf("Expected %d total checks, got %d", expectedTotal, len(allChecks))
	}
}

func TestGoVersionCheck(t *testing.T) {
	check := NewGoVersionCheck()

	if check.Name() != "Go Version" {
		t.Errorf("Expected name 'Go Version', got %s", check.Name())
	}

	if check.Category() != "environment" {
		t.Errorf("Expected category 'environment', got %s", check.Category())
	}

	// Run the actual check (this will check the real Go installation)
	result := check.Check()

	// We expect Go to be installed in the test environment
	if result.Status == StatusError {
		t.Errorf("Go version check failed: %s", result.Message)
	}

	// Check that details are populated
	if result.Details == nil {
		t.Error("Expected details to be populated")
	}
}
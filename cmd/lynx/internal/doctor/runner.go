package doctor

import (
	"fmt"
	"sync"
	"time"
)

// DiagnosticRunner manages and executes diagnostic checks
type DiagnosticRunner struct {
	verbose      bool
	autoFix      bool
	appliedFixes []string
	mu           sync.Mutex
}

// NewDiagnosticRunner creates a new diagnostic runner
func NewDiagnosticRunner() *DiagnosticRunner {
	return &DiagnosticRunner{
		appliedFixes: make([]string, 0),
	}
}

// SetVerbose sets verbose mode
func (r *DiagnosticRunner) SetVerbose(verbose bool) {
	r.verbose = verbose
}

// SetAutoFix sets auto-fix mode
func (r *DiagnosticRunner) SetAutoFix(autoFix bool) {
	r.autoFix = autoFix
}

// GetAppliedFixes returns the list of applied fixes
func (r *DiagnosticRunner) GetAppliedFixes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.appliedFixes
}

// GetAllChecks returns all available checks
func (r *DiagnosticRunner) GetAllChecks() []HealthCheck {
	checks := []HealthCheck{}
	checks = append(checks, r.GetEnvironmentChecks()...)
	checks = append(checks, r.GetToolsChecks()...)
	checks = append(checks, r.GetProjectChecks()...)
	checks = append(checks, r.GetConfigChecks()...)
	return checks
}

// GetEnvironmentChecks returns environment-related checks
func (r *DiagnosticRunner) GetEnvironmentChecks() []HealthCheck {
	return []HealthCheck{
		NewGoVersionCheck(),
		NewGoEnvCheck(),
		NewGitCheck(),
	}
}

// GetToolsChecks returns tool-related checks
func (r *DiagnosticRunner) GetToolsChecks() []HealthCheck {
	return []HealthCheck{
		NewProtocCheck(),
		NewWireCheck(),
	}
}

// GetProjectChecks returns project-related checks
func (r *DiagnosticRunner) GetProjectChecks() []HealthCheck {
	return []HealthCheck{
		NewProjectStructureCheck(),
		NewGoModCheck(),
		NewMakefileCheck(),
	}
}

// GetConfigChecks returns configuration-related checks
func (r *DiagnosticRunner) GetConfigChecks() []HealthCheck {
	return []HealthCheck{
		NewConfigFileCheck(),
	}
}

// RunChecks executes the provided health checks
func (r *DiagnosticRunner) RunChecks(checks []HealthCheck) []CheckResult {
	results := make([]CheckResult, 0, len(checks))
	
	for _, check := range checks {
		if r.verbose {
			fmt.Printf("Running check: %s...\n", check.Name())
		}
		
		// Run the check
		result := check.Check()
		
		// Attempt auto-fix if enabled and available
		if r.autoFix && result.Status != StatusOK && result.FixAvailable && check.CanAutoFix() {
			if r.verbose {
				fmt.Printf("  Attempting to fix %s...\n", check.Name())
			}
			
			if err := check.Fix(); err != nil {
				if r.verbose {
					fmt.Printf("  Failed to fix %s: %v\n", check.Name(), err)
				}
				result.Details["fix_error"] = err.Error()
			} else {
				// Re-run the check after fix
				newResult := check.Check()
				if newResult.Status == StatusOK {
					result = newResult
					result.FixApplied = true
					r.mu.Lock()
					r.appliedFixes = append(r.appliedFixes, fmt.Sprintf("Fixed: %s", check.Name()))
					r.mu.Unlock()
					
					if r.verbose {
						fmt.Printf("  Successfully fixed %s\n", check.Name())
					}
				}
			}
		}
		
		results = append(results, result)
	}
	
	return results
}

// RunChecksParallel executes health checks in parallel
func (r *DiagnosticRunner) RunChecksParallel(checks []HealthCheck) []CheckResult {
	results := make([]CheckResult, len(checks))
	var wg sync.WaitGroup
	
	for i, check := range checks {
		wg.Add(1)
		go func(index int, hc HealthCheck) {
			defer wg.Done()
			
			if r.verbose {
				fmt.Printf("Running check: %s...\n", hc.Name())
			}
			
			// Run the check
			result := hc.Check()
			
			// Auto-fix is not safe in parallel mode for now
			// Could be improved with proper locking per resource
			
			results[index] = result
		}(i, check)
	}
	
	wg.Wait()
	return results
}

// RunCategoryChecks runs checks for a specific category
func (r *DiagnosticRunner) RunCategoryChecks(category string) ([]CheckResult, error) {
	var checks []HealthCheck
	
	switch category {
	case "environment":
		checks = r.GetEnvironmentChecks()
	case "tools":
		checks = r.GetToolsChecks()
	case "project":
		checks = r.GetProjectChecks()
	case "config":
		checks = r.GetConfigChecks()
	case "all":
		checks = r.GetAllChecks()
	default:
		return nil, fmt.Errorf("unknown category: %s", category)
	}
	
	return r.RunChecks(checks), nil
}

// QuickCheck performs a quick health check with essential checks only
func (r *DiagnosticRunner) QuickCheck() []CheckResult {
	essentialChecks := []HealthCheck{
		NewGoVersionCheck(),
		NewGoModCheck(),
		NewProjectStructureCheck(),
	}
	
	return r.RunChecks(essentialChecks)
}

// CheckWithTimeout runs a check with a timeout
func (r *DiagnosticRunner) CheckWithTimeout(check HealthCheck, timeout time.Duration) CheckResult {
	resultChan := make(chan CheckResult, 1)
	
	go func() {
		resultChan <- check.Check()
	}()
	
	select {
	case result := <-resultChan:
		return result
	case <-time.After(timeout):
		return CheckResult{
			Name:     check.Name(),
			Category: check.Category(),
			Status:   StatusError,
			Message:  "Check timed out",
			Duration: timeout,
		}
	}
}

// ValidateEnvironment performs a comprehensive environment validation
func (r *DiagnosticRunner) ValidateEnvironment() error {
	envChecks := r.GetEnvironmentChecks()
	results := r.RunChecks(envChecks)
	
	for _, result := range results {
		if result.Status == StatusError {
			return fmt.Errorf("environment validation failed: %s - %s", result.Name, result.Message)
		}
	}
	
	return nil
}

// ValidateProject performs a comprehensive project validation
func (r *DiagnosticRunner) ValidateProject() error {
	projectChecks := r.GetProjectChecks()
	results := r.RunChecks(projectChecks)
	
	for _, result := range results {
		if result.Status == StatusError {
			return fmt.Errorf("project validation failed: %s - %s", result.Name, result.Message)
		}
	}
	
	return nil
}
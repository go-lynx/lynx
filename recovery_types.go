package lynx

import "time"

// ErrorRecord represents a recorded error with enhanced context.
type ErrorRecord struct {
	Timestamp    time.Time
	ErrorType    string
	Category     ErrorCategory
	Message      string
	Component    string
	Severity     ErrorSeverity
	Context      map[string]any
	Recovered    bool
	RecoveryTime *time.Time
	StackTrace   string
	UserID       string
	RequestID    string
	Environment  string
	Version      string
}

// RecoveryRecord represents a recovery attempt.
type RecoveryRecord struct {
	Timestamp time.Time
	ErrorType string
	Component string
	Strategy  string
	Success   bool
	Duration  time.Duration
	Message   string
}

// ErrorSeverity represents error severity levels.
type ErrorSeverity int

const (
	ErrorSeverityLow ErrorSeverity = iota
	ErrorSeverityMedium
	ErrorSeverityHigh
	ErrorSeverityCritical
)

// ErrorCategory represents error categories for better classification.
type ErrorCategory string

const (
	ErrorCategoryNetwork    ErrorCategory = "network"
	ErrorCategoryDatabase   ErrorCategory = "database"
	ErrorCategoryConfig     ErrorCategory = "configuration"
	ErrorCategoryPlugin     ErrorCategory = "plugin"
	ErrorCategoryResource   ErrorCategory = "resource"
	ErrorCategorySecurity   ErrorCategory = "security"
	ErrorCategoryTimeout    ErrorCategory = "timeout"
	ErrorCategoryValidation ErrorCategory = "validation"
	ErrorCategorySystem     ErrorCategory = "system"
)

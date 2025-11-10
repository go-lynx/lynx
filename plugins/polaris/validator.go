package polaris

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-lynx/lynx/plugins/polaris/conf"
)

// ValidationError configuration validation error
type ValidationError struct {
	Field   string
	Message string
	Value   interface{}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
}

// ValidationResult validation result
type ValidationResult struct {
	IsValid bool
	Errors  []*ValidationError
}

// NewValidationResult creates validation result
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		IsValid: true,
		Errors:  make([]*ValidationError, 0),
	}
}

// AddError adds error
func (r *ValidationResult) AddError(field, message string, value interface{}) {
	r.IsValid = false
	r.Errors = append(r.Errors, &ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// Error returns error message
func (r *ValidationResult) Error() string {
	if r.IsValid {
		return ""
	}

	var messages []string
	for _, err := range r.Errors {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// Validator configuration validator
type Validator struct {
	config *conf.Polaris
}

// NewValidator creates new validator
func NewValidator(config *conf.Polaris) *Validator {
	return &Validator{
		config: config,
	}
}

// Validate validates configuration
func (v *Validator) Validate() *ValidationResult {
	result := NewValidationResult()

	// Validate basic fields
	v.validateBasicFields(result)

	// Validate numeric ranges
	v.validateNumericRanges(result)

	// Validate enum values
	v.validateEnumValues(result)

	// Validate time-related configurations
	v.validateTimeConfigs(result)

	// Validate dependencies
	v.validateDependencies(result)

	// Additional: validate security-related configurations
	v.validateSecurityConfigs(result)

	// Additional: validate network-related configurations
	v.validateNetworkConfigs(result)

	// Additional: validate performance-related configurations
	v.validatePerformanceConfigs(result)

	return result
}

// validateBasicFields validates basic fields
func (v *Validator) validateBasicFields(result *ValidationResult) {
	// Validate namespace
	if v.config.Namespace == "" {
		result.AddError("namespace", "namespace cannot be empty", v.config.Namespace)
	} else if len(v.config.Namespace) > 64 {
		result.AddError("namespace", "namespace length must not exceed 64 characters", v.config.Namespace)
	} else {
		// Validate namespace format (only letters, numbers, underscores, and hyphens allowed)
		namespaceRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
		if !namespaceRegex.MatchString(v.config.Namespace) {
			result.AddError("namespace", "namespace can only contain letters, numbers, underscores, and hyphens", v.config.Namespace)
		}
	}

	// Validate Token (if provided)
	if v.config.Token != "" && len(v.config.Token) > 1024 {
		result.AddError("token", "token length must not exceed 1024 characters", v.config.Token)
	}
	if v.config.Token != "" && len(v.config.Token) < 8 {
		result.AddError("token", "token must be at least 8 characters long", v.config.Token)
	}
}

// validateNumericRanges validates numeric ranges
func (v *Validator) validateNumericRanges(result *ValidationResult) {
	// Validate weight
	if v.config.Weight < conf.MinWeight || v.config.Weight > conf.MaxWeight {
		result.AddError("weight", fmt.Sprintf("weight must be between %d and %d", conf.MinWeight, conf.MaxWeight), v.config.Weight)
	}

	// Validate TTL
	if v.config.Ttl < conf.MinTTL || v.config.Ttl > conf.MaxTTL {
		result.AddError("ttl", fmt.Sprintf("ttl must be between %d and %d seconds", conf.MinTTL, conf.MaxTTL), v.config.Ttl)
	}
}

// validateEnumValues validates enum values
func (v *Validator) validateEnumValues(result *ValidationResult) {
	// No enum value fields in current configuration, skip validation
}

// validateTimeConfigs validates time-related configurations
func (v *Validator) validateTimeConfigs(result *ValidationResult) {
	// Validate timeout
	if v.config.Timeout != nil {
		timeout := time.Duration(v.config.Timeout.Seconds) * time.Second
		if timeout < time.Duration(conf.MinTimeoutSeconds)*time.Second || timeout > time.Duration(conf.MaxTimeoutSeconds)*time.Second {
			result.AddError("timeout", fmt.Sprintf("timeout must be between %d and %d seconds", conf.MinTimeoutSeconds, conf.MaxTimeoutSeconds), timeout)
		}
	}
}

// validateDependencies validates dependencies
func (v *Validator) validateDependencies(result *ValidationResult) {
	// Validate reasonableness of weight and TTL
	if v.config.Weight > 0 && v.config.Ttl < 5 {
		result.AddError("ttl", "TTL should be at least 5 seconds when weight is greater than 0", v.config.Ttl)
	}

	// Validate coordination between timeout and TTL
	if v.config.Timeout != nil && v.config.Ttl > 0 {
		timeout := time.Duration(v.config.Timeout.Seconds) * time.Second
		ttlDuration := time.Duration(v.config.Ttl) * time.Second

		if timeout >= ttlDuration {
			result.AddError("timeout", "Timeout should be less than TTL to ensure proper operation", timeout)
		}
	}

	// Validate coordination between namespace and service
	if v.config.Namespace == "default" && v.config.Token != "" {
		result.AddError("token", "Token should not be required for default namespace", v.config.Token)
	}
}

// validateSecurityConfigs validates security-related configurations
func (v *Validator) validateSecurityConfigs(result *ValidationResult) {
	// Validate token security
	if v.config.Token != "" {
		// Check token length
		if len(v.config.Token) < 8 {
			result.AddError("token", "token must be at least 8 characters long for security", v.config.Token)
		}

		// Check token complexity (must contain both letters and digits)
		hasLetter := false
		hasDigit := false
		for _, char := range v.config.Token {
			if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
				hasLetter = true
			}
			if char >= '0' && char <= '9' {
				hasDigit = true
			}
		}
		if !hasLetter || !hasDigit {
			result.AddError("token", "token must contain both letters and numbers for security", "***")
		}
	}

	// Validate namespace security
	if v.config.Namespace != "" {
		// Check for sensitive words
		sensitiveChars := []string{"admin", "root", "system", "internal"}
		namespaceLower := strings.ToLower(v.config.Namespace)
		for _, sensitive := range sensitiveChars {
			if strings.Contains(namespaceLower, sensitive) {
				result.AddError("namespace", fmt.Sprintf("namespace should not contain sensitive word: %s", sensitive), v.config.Namespace)
			}
		}
	}
}

// validateNetworkConfigs validates network-related configurations
func (v *Validator) validateNetworkConfigs(result *ValidationResult) {
	// Validate connection timeout configuration
	if v.config.Timeout != nil {
		timeout := v.config.Timeout.AsDuration()
		if timeout < 100*time.Millisecond {
			result.AddError("timeout", "timeout should be at least 100ms for network operations", timeout)
		}
		if timeout > 30*time.Second {
			result.AddError("timeout", "timeout should not exceed 30s for network operations", timeout)
		}
	}

	// Validate retry configuration
	if v.config.MaxRetryTimes < 0 {
		result.AddError("max_retry_times", "max_retry_times cannot be negative", v.config.MaxRetryTimes)
	}
	if v.config.MaxRetryTimes > 10 {
		result.AddError("max_retry_times", "max_retry_times should not exceed 10 to prevent excessive retries", v.config.MaxRetryTimes)
	}
}

// validatePerformanceConfigs validates performance-related configurations
func (v *Validator) validatePerformanceConfigs(result *ValidationResult) {
	// Validate weight configuration
	if v.config.Weight < 1 {
		result.AddError("weight", "weight should be at least 1 for load balancing", v.config.Weight)
	}
	if v.config.Weight > 1000 {
		result.AddError("weight", "weight should not exceed 1000 to prevent load balancing issues", v.config.Weight)
	}

	// Validate TTL configuration
	if v.config.Ttl < 1 {
		result.AddError("ttl", "TTL should be at least 1 second", v.config.Ttl)
	}
	if v.config.Ttl > 86400 {
		result.AddError("ttl", "TTL should not exceed 24 hours (86400 seconds)", v.config.Ttl)
	}
}

// contains checks if slice contains specified value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// ValidateConfig convenient configuration validation function
func ValidateConfig(config *conf.Polaris) error {
	validator := NewValidator(config)
	result := validator.Validate()

	if !result.IsValid {
		return fmt.Errorf("configuration validation failed: %s", result.Error())
	}

	return nil
}

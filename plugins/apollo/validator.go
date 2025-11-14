package apollo

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-lynx/lynx/plugins/apollo/conf"
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
	config *conf.Apollo
}

// NewValidator creates new validator
func NewValidator(config *conf.Apollo) *Validator {
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

	return result
}

// validateBasicFields validates basic fields
func (v *Validator) validateBasicFields(result *ValidationResult) {
	// Validate app_id (required)
	if v.config.AppId == "" {
		result.AddError("app_id", "app_id cannot be empty", v.config.AppId)
	} else if len(v.config.AppId) > 128 {
		result.AddError("app_id", "app_id length must not exceed 128 characters", v.config.AppId)
	} else {
		// Validate app_id format (only letters, numbers, underscores, and hyphens allowed)
		appIdRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
		if !appIdRegex.MatchString(v.config.AppId) {
			result.AddError("app_id", "app_id can only contain letters, numbers, underscores, and hyphens", v.config.AppId)
		}
	}

	// Validate meta_server (required)
	if v.config.MetaServer == "" {
		result.AddError("meta_server", "meta_server cannot be empty", v.config.MetaServer)
	} else {
		// Validate URL format
		_, err := url.Parse(v.config.MetaServer)
		if err != nil {
			result.AddError("meta_server", fmt.Sprintf("meta_server must be a valid URL: %v", err), v.config.MetaServer)
		}
	}

	// Validate cluster
	if v.config.Cluster != "" && len(v.config.Cluster) > 64 {
		result.AddError("cluster", "cluster length must not exceed 64 characters", v.config.Cluster)
	}

	// Validate namespace
	if v.config.Namespace != "" && len(v.config.Namespace) > 128 {
		result.AddError("namespace", "namespace length must not exceed 128 characters", v.config.Namespace)
	}

	// Validate token (if provided)
	if v.config.Token != "" && len(v.config.Token) > 1024 {
		result.AddError("token", "token length must not exceed 1024 characters", v.config.Token)
	}
	if v.config.Token != "" && len(v.config.Token) < 8 {
		result.AddError("token", "token must be at least 8 characters long", v.config.Token)
	}

	// Validate cache_dir
	if v.config.CacheDir != "" && len(v.config.CacheDir) > 512 {
		result.AddError("cache_dir", "cache_dir length must not exceed 512 characters", v.config.CacheDir)
	}
}

// validateNumericRanges validates numeric ranges
func (v *Validator) validateNumericRanges(result *ValidationResult) {
	// Validate max_retry_times
	if v.config.MaxRetryTimes < conf.MinRetryTimes || v.config.MaxRetryTimes > conf.MaxRetryTimes {
		result.AddError("max_retry_times", fmt.Sprintf("max_retry_times must be between %d and %d", conf.MinRetryTimes, conf.MaxRetryTimes), v.config.MaxRetryTimes)
	}

	// Validate circuit_breaker_threshold
	if v.config.CircuitBreakerThreshold < conf.MinCircuitBreakerThreshold || v.config.CircuitBreakerThreshold > conf.MaxCircuitBreakerThreshold {
		result.AddError("circuit_breaker_threshold", fmt.Sprintf("circuit_breaker_threshold must be between %.1f and %.1f", conf.MinCircuitBreakerThreshold, conf.MaxCircuitBreakerThreshold), v.config.CircuitBreakerThreshold)
	}
}

// validateEnumValues validates enum values
func (v *Validator) validateEnumValues(result *ValidationResult) {
	// Validate log_level
	if v.config.LogLevel != "" {
		valid := false
		for _, level := range conf.SupportedLogLevels {
			if v.config.LogLevel == level {
				valid = true
				break
			}
		}
		if !valid {
			result.AddError("log_level", fmt.Sprintf("log_level must be one of: %v", conf.SupportedLogLevels), v.config.LogLevel)
		}
	}

	// Validate merge_strategy in service_config
	if v.config.ServiceConfig != nil && v.config.ServiceConfig.MergeStrategy != "" {
		valid := false
		for _, strategy := range conf.SupportedMergeStrategies {
			if v.config.ServiceConfig.MergeStrategy == strategy {
				valid = true
				break
			}
		}
		if !valid {
			result.AddError("service_config.merge_strategy", fmt.Sprintf("merge_strategy must be one of: %v", conf.SupportedMergeStrategies), v.config.ServiceConfig.MergeStrategy)
		}
	}
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

	// Validate notification_timeout
	if v.config.NotificationTimeout != nil {
		timeout := time.Duration(v.config.NotificationTimeout.Seconds) * time.Second
		if timeout < time.Duration(conf.MinNotificationTimeoutSeconds)*time.Second || timeout > time.Duration(conf.MaxNotificationTimeoutSeconds)*time.Second {
			result.AddError("notification_timeout", fmt.Sprintf("notification_timeout must be between %d and %d seconds", conf.MinNotificationTimeoutSeconds, conf.MaxNotificationTimeoutSeconds), timeout)
		}
	}

	// Validate retry_interval
	if v.config.RetryInterval != nil {
		interval := time.Duration(v.config.RetryInterval.Seconds) * time.Second
		if interval < conf.MinRetryInterval || interval > conf.MaxRetryInterval {
			result.AddError("retry_interval", fmt.Sprintf("retry_interval must be between %v and %v", conf.MinRetryInterval, conf.MaxRetryInterval), interval)
		}
	}

	// Validate shutdown_timeout
	if v.config.ShutdownTimeout != nil {
		timeout := time.Duration(v.config.ShutdownTimeout.Seconds) * time.Second
		if timeout < conf.MinShutdownTimeout || timeout > conf.MaxShutdownTimeout {
			result.AddError("shutdown_timeout", fmt.Sprintf("shutdown_timeout must be between %v and %v", conf.MinShutdownTimeout, conf.MaxShutdownTimeout), timeout)
		}
	}
}

// validateDependencies validates dependencies
func (v *Validator) validateDependencies(result *ValidationResult) {
	// Validate coordination between timeout and notification_timeout
	if v.config.Timeout != nil && v.config.NotificationTimeout != nil {
		timeout := time.Duration(v.config.Timeout.Seconds) * time.Second
		notificationTimeout := time.Duration(v.config.NotificationTimeout.Seconds) * time.Second

		if timeout >= notificationTimeout {
			result.AddError("timeout", "timeout should be less than notification_timeout to ensure proper operation", timeout)
		}
	}

	// Validate cache_dir when enable_cache is true
	if v.config.EnableCache && v.config.CacheDir == "" {
		result.AddError("cache_dir", "cache_dir must be set when enable_cache is true", v.config.CacheDir)
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
	}

	// Validate meta_server security (should use HTTPS in production)
	if v.config.MetaServer != "" {
		parsedURL, err := url.Parse(v.config.MetaServer)
		if err == nil && parsedURL.Scheme == "http" {
			result.AddError("meta_server", "meta_server should use HTTPS in production environments", v.config.MetaServer)
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

// ValidateConfig convenient configuration validation function
func ValidateConfig(config *conf.Apollo) error {
	validator := NewValidator(config)
	result := validator.Validate()

	if !result.IsValid {
		return fmt.Errorf("configuration validation failed: %s", result.Error())
	}

	return nil
}


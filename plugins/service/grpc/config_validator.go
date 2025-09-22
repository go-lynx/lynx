// Package grpc provides configuration validation for gRPC clients
package grpc

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/go-lynx/lynx/plugins/service/grpc/conf"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s' (value: %v): %s", e.Field, e.Value, e.Message)
}

// ValidationResult contains the results of configuration validation
type ValidationResult struct {
	Valid    bool
	Errors   []*ValidationError
	Warnings []string
}

// AddError adds a validation error
func (vr *ValidationResult) AddError(field string, value interface{}, message string) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	})
}

// AddWarning adds a validation warning
func (vr *ValidationResult) AddWarning(message string) {
	vr.Warnings = append(vr.Warnings, message)
}

// HasErrors returns true if there are validation errors
func (vr *ValidationResult) HasErrors() bool {
	return len(vr.Errors) > 0
}

// HasWarnings returns true if there are validation warnings
func (vr *ValidationResult) HasWarnings() bool {
	return len(vr.Warnings) > 0
}

// ConfigValidator provides configuration validation functionality
type ConfigValidator struct {
	strictMode bool
}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator(strictMode bool) *ConfigValidator {
	return &ConfigValidator{
		strictMode: strictMode,
	}
}

// ValidateClientConfig validates the entire client configuration
func (cv *ConfigValidator) ValidateClientConfig(config *conf.GrpcClient) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if config == nil {
		result.AddError("config", nil, "client configuration cannot be nil")
		return result
	}

	// Validate default timeouts
	cv.validateTimeouts(config, result)

	// Validate connection pooling
	cv.validateConnectionPooling(config, result)

	// Validate retry configuration
	cv.validateRetryConfig(config, result)

	// Validate subscribe services
	cv.validateSubscribeServices(config, result)

	// Validate legacy services (if any)
	cv.validateLegacyServices(config, result)

	// Validate TLS configuration
	cv.validateTLSConfig(config, result)

	return result
}

// validateTimeouts validates timeout configurations
func (cv *ConfigValidator) validateTimeouts(config *conf.GrpcClient, result *ValidationResult) {
	if config.DefaultTimeout != nil {
		timeout := config.DefaultTimeout.AsDuration()
		if timeout <= 0 {
			result.AddError("default_timeout", timeout, "timeout must be positive")
		} else if timeout > 5*time.Minute {
			result.AddWarning("default_timeout is very large (>5 minutes), this may cause performance issues")
		}
	}

	if config.DefaultKeepAlive != nil {
		keepAlive := config.DefaultKeepAlive.AsDuration()
		if keepAlive <= 0 {
			result.AddError("default_keep_alive", keepAlive, "keep alive must be positive")
		} else if keepAlive < 10*time.Second {
			result.AddWarning("default_keep_alive is very short (<10 seconds), this may cause connection instability")
		}
	}
}

// validateConnectionPooling validates connection pooling configuration
func (cv *ConfigValidator) validateConnectionPooling(config *conf.GrpcClient, result *ValidationResult) {
	poolEnabled := config.GetConnectionPooling()
	if !poolEnabled {
		return
	}

	poolSize := config.GetPoolSize()
	if poolSize <= 0 {
		result.AddError("connection_pooling.pool_size", poolSize, "pool size must be positive when pooling is enabled")
	} else if poolSize > 1000 {
		result.AddWarning("connection_pooling.pool_size is very large (>1000), this may consume excessive resources")
	}

	if config.GetIdleTimeout() != nil {
		idleTimeout := config.GetIdleTimeout().AsDuration()
		if idleTimeout <= 0 {
			result.AddError("connection_pooling.idle_timeout", idleTimeout, "idle timeout must be positive")
		} else if idleTimeout < time.Minute {
			result.AddWarning("connection_pooling.idle_timeout is very short (<1 minute), connections may be recycled too frequently")
		}
	}
}

// validateRetryConfig validates retry configuration
func (cv *ConfigValidator) validateRetryConfig(config *conf.GrpcClient, result *ValidationResult) {
	if config.MaxRetries < 0 {
		result.AddError("max_retries", config.MaxRetries, "max retries cannot be negative")
	} else if config.MaxRetries > 10 {
		result.AddWarning("max_retries is very high (>10), this may cause excessive delays")
	}

	if config.RetryBackoff != nil {
		backoff := config.RetryBackoff.AsDuration()
		if backoff <= 0 {
			result.AddError("retry_backoff", backoff, "retry backoff must be positive")
		} else if backoff > 30*time.Second {
			result.AddWarning("retry_backoff is very large (>30 seconds), this may cause excessive delays")
		}
	}
}

// validateSubscribeServices validates subscribe services configuration
func (cv *ConfigValidator) validateSubscribeServices(config *conf.GrpcClient, result *ValidationResult) {
	subscribeServices := config.GetSubscribeServices()
	if subscribeServices == nil {
		return
	}

	serviceNames := make(map[string]bool)

	for i, serviceConfig := range subscribeServices {
		serviceName := serviceConfig.GetName()
		// Check for duplicate service names
		if serviceNames[serviceName] {
			result.AddError(fmt.Sprintf("subscribe_services[%d]", i), serviceName, "duplicate service name")
			continue
		}
		serviceNames[serviceName] = true

		// Validate service name
		if err := cv.validateServiceName(serviceName); err != nil {
			result.AddError(fmt.Sprintf("subscribe_services[%d].name", i), serviceName, err.Error())
		}

		// Validate endpoint (if provided)
		if serviceConfig.GetEndpoint() != "" {
			if err := cv.validateEndpoint(serviceConfig.GetEndpoint()); err != nil {
				result.AddError("subscribe_services."+serviceName+".endpoint", serviceConfig.GetEndpoint(), err.Error())
			}
		}

		// Validate load balancer strategy
		if serviceConfig.GetLoadBalancer() != "" {
			if err := cv.validateLoadBalancerStrategy(serviceConfig.GetLoadBalancer()); err != nil {
				result.AddError("subscribe_services."+serviceName+".load_balancer", serviceConfig.GetLoadBalancer(), err.Error())
			}
		}

		// Validate circuit breaker configuration
		cv.validateCircuitBreakerConfig(serviceName, serviceConfig, result)

		// Validate metadata
		cv.validateMetadata(serviceName, serviceConfig.GetMetadata(), result)
	}
}

// validateLegacyServices validates legacy services configuration
func (cv *ConfigValidator) validateLegacyServices(config *conf.GrpcClient, result *ValidationResult) {
	services := config.GetSubscribeServices()
	if len(services) == 0 {
		return
	}

	// Warn about using legacy configuration
	result.AddWarning("using legacy 'services' configuration, consider migrating to 'subscribe_services'")

	for i, service := range services {
		prefix := fmt.Sprintf("services[%d]", i)

		if service.GetEndpoint() == "" {
			result.AddError(prefix+".endpoint", "", "endpoint is required for legacy services")
		} else {
			if err := cv.validateEndpoint(service.GetEndpoint()); err != nil {
				result.AddError(prefix+".endpoint", service.GetEndpoint(), err.Error())
			}
		}
	}
}

// validateTLSConfig validates TLS configuration
func (cv *ConfigValidator) validateTLSConfig(config *conf.GrpcClient, result *ValidationResult) {
	if !config.GetTlsEnable() {
		return
	}

	// Validate TLS auth type
	authType := config.GetTlsAuthType()
	if authType < 0 || authType > 4 {
		result.AddError("tls_auth_type", authType, "invalid TLS auth type")
	}

	// Additional TLS validation could be added here
	// For example, certificate file validation, cipher suite validation, etc.
}

// validateServiceName validates a service name
func (cv *ConfigValidator) validateServiceName(serviceName string) error {
	if serviceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	if len(serviceName) > 253 {
		return fmt.Errorf("service name too long (max 253 characters)")
	}

	// Check for valid DNS name format
	if !cv.isValidDNSName(serviceName) {
		return fmt.Errorf("service name must be a valid DNS name")
	}

	return nil
}

// validateEndpoint validates an endpoint URL
func (cv *ConfigValidator) validateEndpoint(endpoint string) error {
	if endpoint == "" {
		return fmt.Errorf("endpoint cannot be empty")
	}

	// Check if it's a discovery URL
	if strings.HasPrefix(endpoint, "discovery://") {
		serviceName := strings.TrimPrefix(endpoint, "discovery://")
		return cv.validateServiceName(serviceName)
	}

	// Parse as regular URL
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	// Validate scheme
	if parsedURL.Scheme == "" {
		return fmt.Errorf("endpoint must include a scheme (e.g., grpc://)")
	}

	// Validate host
	if parsedURL.Host == "" {
		return fmt.Errorf("endpoint must include a host")
	}

	// Validate host format
	host, port, err := net.SplitHostPort(parsedURL.Host)
	if err != nil {
		// No port specified, that's okay
		host = parsedURL.Host
	} else {
		// Validate port
		if port == "" {
			return fmt.Errorf("empty port in endpoint")
		}
	}

	// Validate host (IP or hostname)
	if net.ParseIP(host) == nil && !cv.isValidHostname(host) {
		return fmt.Errorf("invalid host in endpoint: %s", host)
	}

	return nil
}

// validateLoadBalancerStrategy validates load balancer strategy
func (cv *ConfigValidator) validateLoadBalancerStrategy(strategy string) error {
	validStrategies := map[string]bool{
		"round_robin":          true,
		"random":               true,
		"weighted_round_robin": true,
		"p2c":                  true,
		"consistent_hash":      true,
	}

	if !validStrategies[strategy] {
		return fmt.Errorf("invalid load balancer strategy: %s", strategy)
	}

	return nil
}

// validateCircuitBreakerConfig validates circuit breaker configuration
func (cv *ConfigValidator) validateCircuitBreakerConfig(serviceName string, serviceConfig *conf.SubscribeService, result *ValidationResult) {
	if !serviceConfig.GetCircuitBreakerEnabled() {
		return
	}

	threshold := serviceConfig.GetCircuitBreakerThreshold()
	if threshold <= 0 {
		result.AddError("subscribe_services."+serviceName+".circuit_breaker_threshold", threshold,
			"circuit breaker threshold must be positive when circuit breaker is enabled")
	} else if threshold > 100 {
		result.AddWarning("subscribe_services." + serviceName + ".circuit_breaker_threshold is very high (>100), circuit breaker may not trigger effectively")
	}
}

// validateMetadata validates service metadata
func (cv *ConfigValidator) validateMetadata(serviceName string, metadata map[string]string, result *ValidationResult) {
	if len(metadata) == 0 {
		return
	}

	for key, value := range metadata {
		if key == "" {
			result.AddError("subscribe_services."+serviceName+".metadata", key, "metadata key cannot be empty")
		}

		if len(key) > 255 {
			result.AddError("subscribe_services."+serviceName+".metadata."+key, key, "metadata key too long (max 255 characters)")
		}

		if len(value) > 1024 {
			result.AddWarning("subscribe_services." + serviceName + ".metadata." + key + " value is very long (>1024 characters)")
		}
	}
}

// isValidDNSName checks if a string is a valid DNS name
func (cv *ConfigValidator) isValidDNSName(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}

	// Split by dots and validate each label
	labels := strings.Split(name, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}

		// Check for valid characters (alphanumeric and hyphens)
		for i, r := range label {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
				return false
			}
			// Hyphens cannot be at the beginning or end
			if r == '-' && (i == 0 || i == len(label)-1) {
				return false
			}
		}
	}

	return true
}

// isValidHostname checks if a string is a valid hostname
func (cv *ConfigValidator) isValidHostname(hostname string) bool {
	return cv.isValidDNSName(hostname)
}

// GetValidationSummary returns a formatted summary of validation results
func (cv *ConfigValidator) GetValidationSummary(result *ValidationResult) string {
	if result.Valid && !result.HasWarnings() {
		return "Configuration validation passed with no issues."
	}

	var summary strings.Builder

	if result.HasErrors() {
		summary.WriteString(fmt.Sprintf("Configuration validation failed with %d error(s):\n", len(result.Errors)))
		for i, err := range result.Errors {
			summary.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
		}
	}

	if result.HasWarnings() {
		if result.HasErrors() {
			summary.WriteString("\n")
		}
		summary.WriteString(fmt.Sprintf("Configuration validation completed with %d warning(s):\n", len(result.Warnings)))
		for i, warning := range result.Warnings {
			summary.WriteString(fmt.Sprintf("  %d. %s\n", i+1, warning))
		}
	}

	return summary.String()
}

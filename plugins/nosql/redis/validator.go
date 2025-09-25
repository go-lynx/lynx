package redis

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-lynx/lynx/app/config"
	"github.com/go-lynx/lynx/plugins/nosql/redis/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in field '%s': %s", e.Field, e.Message)
}

// ValidationResult represents the configuration validation result
type ValidationResult struct {
	IsValid bool
	Errors  []ValidationError
}

// AddError adds a validation error
func (r *ValidationResult) AddError(field, message string) {
	r.Errors = append(r.Errors, ValidationError{Field: field, Message: message})
	r.IsValid = false
}

// Error returns a string representation of all validation errors
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

// ValidateRedisConfig validates the completeness and reasonableness of Redis configuration
func ValidateRedisConfig(config *conf.Redis) *ValidationResult {
	result := &ValidationResult{IsValid: true}

	if config == nil {
		result.AddError("config", "configuration cannot be nil")
		return result
	}

	// Basic connection validation
	validateBasicConnection(config, result)

	// Connection pool configuration validation
	validateConnectionPool(config, result)

	// Timeout configuration validation
	validateTimeouts(config, result)

	// Retry configuration validation
	validateRetryConfig(config, result)

	// TLS configuration validation
	validateTLSConfig(config, result)

	// Sentinel configuration validation
	validateSentinelConfig(config, result)

	// Database configuration validation
	validateDatabaseConfig(config, result)

	// Client name validation
	validateClientName(config, result)

	// Network configuration validation
	validateNetworkConfig(config, result)

	return result
}

// validateBasicConnection validates basic connection configuration
func validateBasicConnection(config *conf.Redis, result *ValidationResult) {
	// Validate address list
	if len(config.Addrs) == 0 {
		result.AddError("addrs", "at least one address must be provided")
		return
	}

	// Validate address format
	for i, addr := range config.Addrs {
		if strings.TrimSpace(addr) == "" {
			result.AddError(fmt.Sprintf("addrs[%d]", i), "address cannot be empty")
			continue
		}

		// Validate address format
		if err := validateAddress(addr); err != nil {
			result.AddError(fmt.Sprintf("addrs[%d]", i), err.Error())
		}
	}
}

// validateAddress validates a single address format
func validateAddress(addr string) error {
	// Support rediss:// prefix (TLS)
	if strings.HasPrefix(strings.ToLower(addr), "rediss://") {
		addr = strings.TrimPrefix(addr, "rediss://")
	}

	// Support redis:// prefix
	if strings.HasPrefix(strings.ToLower(addr), "redis://") {
		addr = strings.TrimPrefix(addr, "redis://")
	}

	// Validate host:port format
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid address format: %s", err.Error())
	}

	// Validate host
	if host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	// Validate port
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}

	// Validate port range
	if portNum, err := strconv.Atoi(port); err != nil || portNum <= 0 || portNum > 65535 {
		return fmt.Errorf("invalid port number: %s (must be 1-65535)", port)
	}

	return nil
}

// validateConnectionPool validates connection pool configuration
func validateConnectionPool(config *conf.Redis, result *ValidationResult) {
	// Validate minimum idle connections
	if config.MinIdleConns < 0 {
		result.AddError("min_idle_conns", "cannot be negative")
	}

	// Validate maximum idle connections
	if config.MaxIdleConns < 0 {
		result.AddError("max_idle_conns", "cannot be negative")
	}

	// Validate maximum active connections
	if config.MaxActiveConns <= 0 {
		result.AddError("max_active_conns", "must be greater than 0")
	}

	// Validate connection pool size relationship
	if config.MinIdleConns > 0 && config.MaxActiveConns > 0 {
		if config.MinIdleConns > config.MaxActiveConns {
			result.AddError("min_idle_conns", "cannot be greater than max_active_conns")
		}
	}

	// Validate connection maximum idle time
	if config.ConnMaxIdleTime != nil {
		duration := config.ConnMaxIdleTime.AsDuration()
		if duration < 0 {
			result.AddError("conn_max_idle_time", "cannot be negative")
		}
		if duration > 24*time.Hour {
			result.AddError("conn_max_idle_time", "cannot exceed 24 hours")
		}
	}

	// Validate connection maximum lifetime
	if config.MaxConnAge != nil {
		duration := config.MaxConnAge.AsDuration()
		if duration < 0 {
			result.AddError("max_conn_age", "cannot be negative")
		}
		if duration > 7*24*time.Hour {
			result.AddError("max_conn_age", "cannot exceed 7 days")
		}
	}

	// Validate connection pool timeout
	if config.PoolTimeout != nil {
		duration := config.PoolTimeout.AsDuration()
		if duration < 0 {
			result.AddError("pool_timeout", "cannot be negative")
		}
		// Relax limit to allow longer timeouts for high latency networks
		if duration > 5*time.Minute {
			result.AddError("pool_timeout", "cannot exceed 5 minutes")
		}
	}
}

// validateTimeouts validates timeout configuration
func validateTimeouts(config *conf.Redis, result *ValidationResult) {
	// Validate connection timeout
	if config.DialTimeout != nil {
		duration := config.DialTimeout.AsDuration()
		if duration < 0 {
			result.AddError("dial_timeout", "cannot be negative")
		}
		if duration > 60*time.Second {
			result.AddError("dial_timeout", "cannot exceed 60 seconds")
		}
	}

	// Validate read timeout
	if config.ReadTimeout != nil {
		duration := config.ReadTimeout.AsDuration()
		if duration < 0 {
			result.AddError("read_timeout", "cannot be negative")
		}
		if duration > 300*time.Second {
			result.AddError("read_timeout", "cannot exceed 5 minutes")
		}
	}

	// Validate write timeout
	if config.WriteTimeout != nil {
		duration := config.WriteTimeout.AsDuration()
		if duration < 0 {
			result.AddError("write_timeout", "cannot be negative")
		}
		if duration > 300*time.Second {
			result.AddError("write_timeout", "cannot exceed 5 minutes")
		}
	}

	// Validate timeout reasonableness
	if config.DialTimeout != nil && config.ReadTimeout != nil {
		dialDuration := config.DialTimeout.AsDuration()
		readDuration := config.ReadTimeout.AsDuration()
		if dialDuration > readDuration {
			result.AddError("dial_timeout", "should not be greater than read_timeout")
		}
	}
}

// validateRetryConfig validates retry configuration
func validateRetryConfig(config *conf.Redis, result *ValidationResult) {
	// Validate maximum retries
	if config.MaxRetries < 0 {
		result.AddError("max_retries", "cannot be negative")
	}
	if config.MaxRetries > 10 {
		result.AddError("max_retries", "cannot exceed 10")
	}

	// Validate retry backoff time
	if config.MinRetryBackoff != nil {
		duration := config.MinRetryBackoff.AsDuration()
		if duration < 0 {
			result.AddError("min_retry_backoff", "cannot be negative")
		}
		if duration > 1*time.Second {
			result.AddError("min_retry_backoff", "cannot exceed 1 second")
		}
	}

	if config.MaxRetryBackoff != nil {
		duration := config.MaxRetryBackoff.AsDuration()
		if duration < 0 {
			result.AddError("max_retry_backoff", "cannot be negative")
		}
		if duration > 30*time.Second {
			result.AddError("max_retry_backoff", "cannot exceed 30 seconds")
		}
	}

	// Validate backoff time reasonableness
	if config.MinRetryBackoff != nil && config.MaxRetryBackoff != nil {
		minDuration := config.MinRetryBackoff.AsDuration()
		maxDuration := config.MaxRetryBackoff.AsDuration()
		if minDuration > maxDuration {
			result.AddError("min_retry_backoff", "cannot be greater than max_retry_backoff")
		}
	}
}

// validateTLSConfig validates TLS configuration
func validateTLSConfig(config *conf.Redis, result *ValidationResult) {
	if config.Tls == nil {
		return
	}

	// If TLS is enabled, check if addresses support TLS
	if config.Tls.Enabled {
        // reference result to avoid unused parameter warning in this validation stub
        _ = result
		hasTLSSupport := false
		for _, addr := range config.Addrs {
			if strings.HasPrefix(strings.ToLower(addr), "rediss://") {
				hasTLSSupport = true
				break
			}
		}

		if !hasTLSSupport {
			// Warning: TLS enabled but addresses are not in rediss:// format
			// In this case, TLS configuration is still valid, but it is recommended to use the rediss:// prefix
		}
	}
}

// validateSentinelConfig validates Sentinel configuration
func validateSentinelConfig(config *conf.Redis, result *ValidationResult) {
	if config.Sentinel == nil {
		return
	}

	// Validate master node name
	if strings.TrimSpace(config.Sentinel.MasterName) == "" {
		result.AddError("sentinel.master_name", "cannot be empty when sentinel mode is enabled")
	}

	// Validate Sentinel addresses
	if len(config.Sentinel.Addrs) > 0 {
		for i, addr := range config.Sentinel.Addrs {
			if strings.TrimSpace(addr) == "" {
				result.AddError(fmt.Sprintf("sentinel.addrs[%d]", i), "address cannot be empty")
				continue
			}

			if err := validateAddress(addr); err != nil {
				result.AddError(fmt.Sprintf("sentinel.addrs[%d]", i), err.Error())
			}
		}
	}
}

// validateDatabaseConfig validates database configuration
func validateDatabaseConfig(config *conf.Redis, result *ValidationResult) {
	// Validate database number
	if config.Db < 0 {
		result.AddError("db", "database number cannot be negative")
	}
	
	// Check if it's cluster mode
	isClusterMode := len(config.Addrs) > 1 && (config.Sentinel == nil || config.Sentinel.MasterName == "")
	
	if isClusterMode {
		// Redis Cluster only supports database 0
		if config.Db != 0 {
			result.AddError("db", "Redis Cluster mode only supports database 0")
		}
	} else {
		// Single node and sentinel mode support 0-15 databases
		if config.Db > 15 {
			result.AddError("db", "database number cannot exceed 15 (Redis default limit)")
		}
	}
}

// validateClientName validates client name
func validateClientName(config *conf.Redis, result *ValidationResult) {
	if config.ClientName != "" {
		// Validate client name length
		if len(config.ClientName) > 64 {
			result.AddError("client_name", "cannot exceed 64 characters")
		}

		// Validate client name format (only letters, numbers, underscores, and hyphens allowed)
		matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, config.ClientName)
		if !matched {
			result.AddError("client_name", "can only contain letters, numbers, underscores, and hyphens")
		}
	}
}

// validateNetworkConfig validates network configuration
func validateNetworkConfig(config *conf.Redis, result *ValidationResult) {
	// Validate network type
	if config.Network != "" {
		validNetworks := []string{"tcp", "tcp4", "tcp6", "unix", "unixpacket"}
		isValid := false
		for _, valid := range validNetworks {
			if config.Network == valid {
				isValid = true
				break
			}
		}

		if !isValid {
			result.AddError("network", fmt.Sprintf("must be one of: %s", strings.Join(validNetworks, ", ")))
		}
	}
}

// ValidateAndSetDefaults validates configuration and sets reasonable defaults
func ValidateAndSetDefaults(config *conf.Redis) error {
	// First validate the configuration
	result := ValidateRedisConfig(config)
	if !result.IsValid {
		return fmt.Errorf("configuration validation failed: %s", result.Error())
	}

	// Set default values
	setDefaultValues(config)

	// Validate again (ensure configuration remains valid after default values are set)
	result = ValidateRedisConfig(config)
	if !result.IsValid {
		return fmt.Errorf("configuration validation failed after setting defaults: %s", result.Error())
	}

	return nil
}

// setDefaultValues sets default values
func setDefaultValues(config *conf.Redis) {
	// Network type default value
	if config.Network == "" {
		config.Network = "tcp"
	}

	// Connection pool default values
	if config.MinIdleConns == 0 {
		config.MinIdleConns = 10
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 20
	}
	if config.MaxActiveConns == 0 {
		config.MaxActiveConns = 20
	}

	// Timeout default values
	if config.DialTimeout == nil {
		config.DialTimeout = durationpb.New(10 * time.Second)
	}
	if config.ReadTimeout == nil {
		config.ReadTimeout = durationpb.New(10 * time.Second)
	}
	if config.WriteTimeout == nil {
		config.WriteTimeout = durationpb.New(10 * time.Second)
	}

	// Connection pool timeout default value
	if config.PoolTimeout == nil {
		config.PoolTimeout = durationpb.New(3 * time.Second)
	}

	// Connection lifecycle default values
	if config.ConnMaxIdleTime == nil {
		config.ConnMaxIdleTime = durationpb.New(10 * time.Second)
	}
	if config.MaxConnAge == nil {
		config.MaxConnAge = durationpb.New(30 * time.Minute)
	}

	// Retry configuration default values
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.MinRetryBackoff == nil {
		config.MinRetryBackoff = durationpb.New(8 * time.Millisecond)
	}
	if config.MaxRetryBackoff == nil {
		config.MaxRetryBackoff = durationpb.New(512 * time.Millisecond)
	}
}

// RedisConfigValidator provides Redis-specific configuration validation
type RedisConfigValidator struct {
	*config.ConfigValidator
	defaultSetter *config.DefaultValueSetter
}

// NewRedisConfigValidator creates a new Redis configuration validator
func NewRedisConfigValidator(strictMode bool, environment string) *RedisConfigValidator {
	validator := config.NewConfigValidator(strictMode, environment)
	defaultSetter := config.NewDefaultValueSetter()
	
	rcv := &RedisConfigValidator{
		ConfigValidator: validator,
		defaultSetter:   defaultSetter,
	}
	
	// Register validation rules
	rcv.registerValidationRules()
	
	// Register default values
	rcv.registerDefaultValues()
	
	return rcv
}

// registerValidationRules registers all Redis-specific validation rules
func (rcv *RedisConfigValidator) registerValidationRules() {
	// Address validation
	rcv.AddRule("Addrs", config.ValidationRule{
		Name:        "redis_addresses",
		Description: "Validates Redis server addresses",
		Validator:   rcv.validateAddresses,
		Required:    true,
	})
	
	// Connection pool validation
	rcv.AddRule("MinIdleConns", config.ValidationRule{
		Name:        "min_idle_connections",
		Description: "Validates minimum idle connections",
		Validator:   config.ValidateRange(0, 1000),
		Required:    false,
	})
	
	rcv.AddRule("MaxActiveConns", config.ValidationRule{
		Name:        "max_active_connections",
		Description: "Validates maximum active connections",
		Validator:   config.ValidateRange(1, 10000),
		Required:    false,
	})
	
	rcv.AddRule("MaxIdleConns", config.ValidationRule{
		Name:        "max_idle_connections",
		Description: "Validates maximum idle connections",
		Validator:   config.ValidateRange(0, 1000),
		Required:    false,
	})
	
	// Timeout validation
	rcv.AddRule("DialTimeout", config.ValidationRule{
		Name:        "dial_timeout",
		Description: "Validates dial timeout",
		Validator:   config.ValidateDuration(100*time.Millisecond, 30*time.Second),
		Required:    false,
	})
	
	rcv.AddRule("ReadTimeout", config.ValidationRule{
		Name:        "read_timeout",
		Description: "Validates read timeout",
		Validator:   config.ValidateDuration(100*time.Millisecond, 60*time.Second),
		Required:    false,
	})
	
	rcv.AddRule("WriteTimeout", config.ValidationRule{
		Name:        "write_timeout",
		Description: "Validates write timeout",
		Validator:   config.ValidateDuration(100*time.Millisecond, 60*time.Second),
		Required:    false,
	})
	
	// Database validation
	rcv.AddRule("DB", config.ValidationRule{
		Name:        "database_number",
		Description: "Validates Redis database number",
		Validator:   config.ValidateRange(0, 15),
		Required:    false,
	})
	
	// Client name validation
	rcv.AddRule("ClientName", config.ValidationRule{
		Name:        "client_name",
		Description: "Validates Redis client name",
		Validator:   rcv.validateClientName,
		Required:    false,
	})
	
	// Retry configuration validation
	rcv.AddRule("MaxRetries", config.ValidationRule{
		Name:        "max_retries",
		Description: "Validates maximum retry attempts",
		Validator:   config.ValidateRange(0, 10),
		Required:    false,
	})
	
	rcv.AddRule("MinRetryBackoff", config.ValidationRule{
		Name:        "min_retry_backoff",
		Description: "Validates minimum retry backoff",
		Validator:   config.ValidateDuration(1*time.Millisecond, 10*time.Second),
		Required:    false,
	})
	
	rcv.AddRule("MaxRetryBackoff", config.ValidationRule{
		Name:        "max_retry_backoff",
		Description: "Validates maximum retry backoff",
		Validator:   config.ValidateDuration(1*time.Millisecond, 60*time.Second),
		Required:    false,
	})
}

// registerDefaultValues registers default values for Redis configuration
func (rcv *RedisConfigValidator) registerDefaultValues() {
	// Connection pool defaults
	rcv.defaultSetter.SetDefault("MinIdleConns", int32(5))
	rcv.defaultSetter.SetDefault("MaxActiveConns", int32(100))
	rcv.defaultSetter.SetDefault("MaxIdleConns", int32(20))
	
	// Timeout defaults
	rcv.defaultSetter.SetDefaultFunc("DialTimeout", func(interface{}) interface{} {
		return durationpb.New(5 * time.Second)
	})
	rcv.defaultSetter.SetDefaultFunc("ReadTimeout", func(interface{}) interface{} {
		return durationpb.New(3 * time.Second)
	})
	rcv.defaultSetter.SetDefaultFunc("WriteTimeout", func(interface{}) interface{} {
		return durationpb.New(3 * time.Second)
	})
	rcv.defaultSetter.SetDefaultFunc("PoolTimeout", func(interface{}) interface{} {
		return durationpb.New(4 * time.Second)
	})
	
	// Connection lifecycle defaults
	rcv.defaultSetter.SetDefaultFunc("ConnMaxIdleTime", func(interface{}) interface{} {
		return durationpb.New(30 * time.Minute)
	})
	rcv.defaultSetter.SetDefaultFunc("MaxConnAge", func(interface{}) interface{} {
		return durationpb.New(0) // 0 means no limit
	})
	
	// Retry defaults
	rcv.defaultSetter.SetDefault("MaxRetries", int32(3))
	rcv.defaultSetter.SetDefaultFunc("MinRetryBackoff", func(interface{}) interface{} {
		return durationpb.New(8 * time.Millisecond)
	})
	rcv.defaultSetter.SetDefaultFunc("MaxRetryBackoff", func(interface{}) interface{} {
		return durationpb.New(512 * time.Millisecond)
	})
	
	// Database default
	rcv.defaultSetter.SetDefault("DB", int32(0))
	
	// Network defaults
	rcv.defaultSetter.SetDefault("Network", "tcp")
}

// ValidateRedisConfigWithFramework validates Redis configuration using the new framework
func (rcv *RedisConfigValidator) ValidateRedisConfigWithFramework(cfg *conf.Redis) *config.ValidationResult {
	// Apply defaults first
	if err := rcv.defaultSetter.ApplyDefaults(cfg); err != nil {
		result := &config.ValidationResult{Valid: false}
		result.AddError("config", cfg, fmt.Sprintf("Failed to apply defaults: %v", err))
		return result
	}
	
	// Validate configuration
	result := rcv.ValidateConfig(cfg, "redis")
	
	// Add custom validations that require multiple fields
	rcv.validateConnectionPoolRelationships(cfg, result)
	rcv.validateTimeoutRelationships(cfg, result)
	rcv.validateRetryBackoffRelationships(cfg, result)
	
	return result
}

// Custom validation functions

// validateAddresses validates Redis server addresses
func (rcv *RedisConfigValidator) validateAddresses(value interface{}, context config.ValidationContext) error {
	addrs, ok := value.([]string)
	if !ok {
		return fmt.Errorf("expected []string, got %T", value)
	}
	
	if len(addrs) == 0 {
		return fmt.Errorf("at least one address is required")
	}
	
	for i, addr := range addrs {
		if strings.TrimSpace(addr) == "" {
			return fmt.Errorf("address[%d] cannot be empty", i)
		}
		
		if err := rcv.validateSingleAddress(addr); err != nil {
			return fmt.Errorf("address[%d]: %w", i, err)
		}
	}
	
	return nil
}

// validateSingleAddress validates a single Redis address
func (rcv *RedisConfigValidator) validateSingleAddress(addr string) error {
	// Support rediss:// prefix (TLS)
	if strings.HasPrefix(strings.ToLower(addr), "rediss://") {
		addr = strings.TrimPrefix(addr, "rediss://")
	}
	
	// Support redis:// prefix
	if strings.HasPrefix(strings.ToLower(addr), "redis://") {
		addr = strings.TrimPrefix(addr, "redis://")
	}
	
	// Validate host:port format
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid address format: %w", err)
	}
	
	// Validate host
	if host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	
	// Validate port
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}
	
	// Validate port range
	if portNum, err := strconv.Atoi(port); err != nil || portNum <= 0 || portNum > 65535 {
		return fmt.Errorf("invalid port number: %s (must be 1-65535)", port)
	}
	
	return nil
}

// validateClientName validates Redis client name
func (rcv *RedisConfigValidator) validateClientName(value interface{}, context config.ValidationContext) error {
	clientName, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", value)
	}
	
	if clientName == "" {
		return nil // Empty client name is allowed
	}
	
	// Client name should not contain spaces or special characters
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, clientName); !matched {
		return fmt.Errorf("client name can only contain alphanumeric characters, underscores, and hyphens")
	}
	
	// Client name length limit
	if len(clientName) > 64 {
		return fmt.Errorf("client name cannot exceed 64 characters")
	}
	
	return nil
}

// validateConnectionPoolRelationships validates relationships between connection pool settings
func (rcv *RedisConfigValidator) validateConnectionPoolRelationships(config *conf.Redis, result *config.ValidationResult) {
	if config.MinIdleConns > 0 && config.MaxActiveConns > 0 {
		if config.MinIdleConns > config.MaxActiveConns {
			result.AddError("MinIdleConns", config.MinIdleConns, "cannot be greater than MaxActiveConns")
		}
	}
	
	if config.MaxIdleConns > 0 && config.MaxActiveConns > 0 {
		if config.MaxIdleConns > config.MaxActiveConns {
			result.AddWarning("MaxIdleConns", config.MaxIdleConns, "should not exceed MaxActiveConns")
		}
	}
}

// validateTimeoutRelationships validates relationships between timeout settings
func (rcv *RedisConfigValidator) validateTimeoutRelationships(config *conf.Redis, result *config.ValidationResult) {
	if config.DialTimeout != nil && config.PoolTimeout != nil {
		dialTimeout := config.DialTimeout.AsDuration()
		poolTimeout := config.PoolTimeout.AsDuration()
		
		if poolTimeout < dialTimeout {
			result.AddWarning("PoolTimeout", poolTimeout, "should be greater than or equal to DialTimeout")
		}
	}
}

// validateRetryBackoffRelationships validates relationships between retry backoff settings
func (rcv *RedisConfigValidator) validateRetryBackoffRelationships(config *conf.Redis, result *config.ValidationResult) {
	if config.MinRetryBackoff != nil && config.MaxRetryBackoff != nil {
		minBackoff := config.MinRetryBackoff.AsDuration()
		maxBackoff := config.MaxRetryBackoff.AsDuration()
		
		if minBackoff > maxBackoff {
			result.AddError("MinRetryBackoff", minBackoff, "cannot be greater than MaxRetryBackoff")
		}
	}
}

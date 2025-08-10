package redis

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-lynx/lynx/plugins/nosql/redis/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// ValidationError 表示配置验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in field '%s': %s", e.Field, e.Message)
}

// ValidationResult 表示配置验证结果
type ValidationResult struct {
	IsValid bool
	Errors  []ValidationError
}

// AddError 添加验证错误
func (r *ValidationResult) AddError(field, message string) {
	r.Errors = append(r.Errors, ValidationError{Field: field, Message: message})
	r.IsValid = false
}

// Error 返回所有验证错误的字符串表示
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

// ValidateRedisConfig 验证Redis配置的完整性和合理性
func ValidateRedisConfig(config *conf.Redis) *ValidationResult {
	result := &ValidationResult{IsValid: true}

	if config == nil {
		result.AddError("config", "configuration cannot be nil")
		return result
	}

	// 基础连接验证
	validateBasicConnection(config, result)

	// 连接池配置验证
	validateConnectionPool(config, result)

	// 超时配置验证
	validateTimeouts(config, result)

	// 重试配置验证
	validateRetryConfig(config, result)

	// TLS配置验证
	validateTLSConfig(config, result)

	// Sentinel配置验证
	validateSentinelConfig(config, result)

	// 数据库配置验证
	validateDatabaseConfig(config, result)

	// 客户端名称验证
	validateClientName(config, result)

	// 网络配置验证
	validateNetworkConfig(config, result)

	return result
}

// validateBasicConnection 验证基础连接配置
func validateBasicConnection(config *conf.Redis, result *ValidationResult) {
	// 验证地址列表
	if len(config.Addrs) == 0 {
		result.AddError("addrs", "at least one address must be provided")
		return
	}

	// 验证地址格式
	for i, addr := range config.Addrs {
		if strings.TrimSpace(addr) == "" {
			result.AddError(fmt.Sprintf("addrs[%d]", i), "address cannot be empty")
			continue
		}

		// 验证地址格式
		if err := validateAddress(addr); err != nil {
			result.AddError(fmt.Sprintf("addrs[%d]", i), err.Error())
		}
	}
}

// validateAddress 验证单个地址格式
func validateAddress(addr string) error {
	// 支持 rediss:// 前缀（TLS）
	if strings.HasPrefix(strings.ToLower(addr), "rediss://") {
		addr = strings.TrimPrefix(addr, "rediss://")
	}

	// 支持 redis:// 前缀
	if strings.HasPrefix(strings.ToLower(addr), "redis://") {
		addr = strings.TrimPrefix(addr, "redis://")
	}

	// 验证主机:端口格式
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid address format: %s", err.Error())
	}

	// 验证主机
	if host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	// 验证端口
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}

	// 验证端口范围
	if portNum, err := strconv.Atoi(port); err != nil || portNum <= 0 || portNum > 65535 {
		return fmt.Errorf("invalid port number: %s (must be 1-65535)", port)
	}

	return nil
}

// validateConnectionPool 验证连接池配置
func validateConnectionPool(config *conf.Redis, result *ValidationResult) {
	// 验证最小空闲连接数
	if config.MinIdleConns < 0 {
		result.AddError("min_idle_conns", "cannot be negative")
	}

	// 验证最大空闲连接数
	if config.MaxIdleConns < 0 {
		result.AddError("max_idle_conns", "cannot be negative")
	}

	// 验证最大活跃连接数
	if config.MaxActiveConns <= 0 {
		result.AddError("max_active_conns", "must be greater than 0")
	}

	// 验证连接池大小关系
	if config.MinIdleConns > 0 && config.MaxActiveConns > 0 {
		if config.MinIdleConns > config.MaxActiveConns {
			result.AddError("min_idle_conns", "cannot be greater than max_active_conns")
		}
	}

	// 验证连接最大空闲时间
	if config.ConnMaxIdleTime != nil {
		duration := config.ConnMaxIdleTime.AsDuration()
		if duration < 0 {
			result.AddError("conn_max_idle_time", "cannot be negative")
		}
		if duration > 24*time.Hour {
			result.AddError("conn_max_idle_time", "cannot exceed 24 hours")
		}
	}

	// 验证连接最大存活时间
	if config.MaxConnAge != nil {
		duration := config.MaxConnAge.AsDuration()
		if duration < 0 {
			result.AddError("max_conn_age", "cannot be negative")
		}
		if duration > 7*24*time.Hour {
			result.AddError("max_conn_age", "cannot exceed 7 days")
		}
	}

	// 验证连接池超时
	if config.PoolTimeout != nil {
		duration := config.PoolTimeout.AsDuration()
		if duration < 0 {
			result.AddError("pool_timeout", "cannot be negative")
		}
		if duration > 30*time.Second {
			result.AddError("pool_timeout", "cannot exceed 30 seconds")
		}
	}
}

// validateTimeouts 验证超时配置
func validateTimeouts(config *conf.Redis, result *ValidationResult) {
	// 验证建连超时
	if config.DialTimeout != nil {
		duration := config.DialTimeout.AsDuration()
		if duration < 0 {
			result.AddError("dial_timeout", "cannot be negative")
		}
		if duration > 60*time.Second {
			result.AddError("dial_timeout", "cannot exceed 60 seconds")
		}
	}

	// 验证读超时
	if config.ReadTimeout != nil {
		duration := config.ReadTimeout.AsDuration()
		if duration < 0 {
			result.AddError("read_timeout", "cannot be negative")
		}
		if duration > 300*time.Second {
			result.AddError("read_timeout", "cannot exceed 5 minutes")
		}
	}

	// 验证写超时
	if config.WriteTimeout != nil {
		duration := config.WriteTimeout.AsDuration()
		if duration < 0 {
			result.AddError("write_timeout", "cannot be negative")
		}
		if duration > 300*time.Second {
			result.AddError("write_timeout", "cannot exceed 5 minutes")
		}
	}

	// 验证超时时间的合理性
	if config.DialTimeout != nil && config.ReadTimeout != nil {
		dialDuration := config.DialTimeout.AsDuration()
		readDuration := config.ReadTimeout.AsDuration()
		if dialDuration > readDuration {
			result.AddError("dial_timeout", "should not be greater than read_timeout")
		}
	}
}

// validateRetryConfig 验证重试配置
func validateRetryConfig(config *conf.Redis, result *ValidationResult) {
	// 验证最大重试次数
	if config.MaxRetries < 0 {
		result.AddError("max_retries", "cannot be negative")
	}
	if config.MaxRetries > 10 {
		result.AddError("max_retries", "cannot exceed 10")
	}

	// 验证重试退避时间
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

	// 验证退避时间的合理性
	if config.MinRetryBackoff != nil && config.MaxRetryBackoff != nil {
		minDuration := config.MinRetryBackoff.AsDuration()
		maxDuration := config.MaxRetryBackoff.AsDuration()
		if minDuration > maxDuration {
			result.AddError("min_retry_backoff", "cannot be greater than max_retry_backoff")
		}
	}
}

// validateTLSConfig 验证TLS配置
func validateTLSConfig(config *conf.Redis, result *ValidationResult) {
	if config.Tls == nil {
		return
	}

	// 如果启用了TLS，检查地址是否支持TLS
	if config.Tls.Enabled {
		hasTLSSupport := false
		for _, addr := range config.Addrs {
			if strings.HasPrefix(strings.ToLower(addr), "rediss://") {
				hasTLSSupport = true
				break
			}
		}

		if !hasTLSSupport {
			// 警告：启用了TLS但地址不是rediss://格式
			// 这种情况下，TLS配置仍然有效，但建议使用rediss://前缀
		}
	}
}

// validateSentinelConfig 验证Sentinel配置
func validateSentinelConfig(config *conf.Redis, result *ValidationResult) {
	if config.Sentinel == nil {
		return
	}

	// 验证主节点名称
	if strings.TrimSpace(config.Sentinel.MasterName) == "" {
		result.AddError("sentinel.master_name", "cannot be empty when sentinel mode is enabled")
	}

	// 验证Sentinel地址
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

// validateDatabaseConfig 验证数据库配置
func validateDatabaseConfig(config *conf.Redis, result *ValidationResult) {
	// 验证数据库编号
	if config.Db < 0 {
		result.AddError("db", "database number cannot be negative")
	}
	if config.Db > 15 {
		result.AddError("db", "database number cannot exceed 15 (Redis default limit)")
	}
}

// validateClientName 验证客户端名称
func validateClientName(config *conf.Redis, result *ValidationResult) {
	if config.ClientName != "" {
		// 验证客户端名称长度
		if len(config.ClientName) > 64 {
			result.AddError("client_name", "cannot exceed 64 characters")
		}

		// 验证客户端名称格式（只允许字母、数字、下划线、连字符）
		matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, config.ClientName)
		if !matched {
			result.AddError("client_name", "can only contain letters, numbers, underscores, and hyphens")
		}
	}
}

// validateNetworkConfig 验证网络配置
func validateNetworkConfig(config *conf.Redis, result *ValidationResult) {
	// 验证网络类型
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

// ValidateAndSetDefaults 验证配置并设置合理的默认值
func ValidateAndSetDefaults(config *conf.Redis) error {
	// 首先验证配置
	result := ValidateRedisConfig(config)
	if !result.IsValid {
		return fmt.Errorf("configuration validation failed: %s", result.Error())
	}

	// 设置默认值
	setDefaultValues(config)

	// 再次验证（确保默认值设置后配置仍然有效）
	result = ValidateRedisConfig(config)
	if !result.IsValid {
		return fmt.Errorf("configuration validation failed after setting defaults: %s", result.Error())
	}

	return nil
}

// setDefaultValues 设置默认值
func setDefaultValues(config *conf.Redis) {
	// 网络类型默认值
	if config.Network == "" {
		config.Network = "tcp"
	}

	// 连接池默认值
	if config.MinIdleConns == 0 {
		config.MinIdleConns = 10
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 20
	}
	if config.MaxActiveConns == 0 {
		config.MaxActiveConns = 20
	}

	// 超时默认值
	if config.DialTimeout == nil {
		config.DialTimeout = durationpb.New(10 * time.Second)
	}
	if config.ReadTimeout == nil {
		config.ReadTimeout = durationpb.New(10 * time.Second)
	}
	if config.WriteTimeout == nil {
		config.WriteTimeout = durationpb.New(10 * time.Second)
	}

	// 连接池超时默认值
	if config.PoolTimeout == nil {
		config.PoolTimeout = durationpb.New(3 * time.Second)
	}

	// 连接生命周期默认值
	if config.ConnMaxIdleTime == nil {
		config.ConnMaxIdleTime = durationpb.New(10 * time.Second)
	}
	if config.MaxConnAge == nil {
		config.MaxConnAge = durationpb.New(30 * time.Minute)
	}

	// 重试配置默认值
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

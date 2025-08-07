package polaris

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-lynx/lynx/plugins/polaris/conf"
)

// ValidationError 配置验证错误
type ValidationError struct {
	Field   string
	Message string
	Value   interface{}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
}

// ValidationResult 验证结果
type ValidationResult struct {
	IsValid bool
	Errors  []*ValidationError
}

// NewValidationResult 创建验证结果
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		IsValid: true,
		Errors:  make([]*ValidationError, 0),
	}
}

// AddError 添加错误
func (r *ValidationResult) AddError(field, message string, value interface{}) {
	r.IsValid = false
	r.Errors = append(r.Errors, &ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// Error 返回错误信息
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

// Validator 配置验证器
type Validator struct {
	config *conf.Polaris
}

// NewValidator 创建新的验证器
func NewValidator(config *conf.Polaris) *Validator {
	return &Validator{
		config: config,
	}
}

// Validate 验证配置
func (v *Validator) Validate() *ValidationResult {
	result := NewValidationResult()

	// 验证基本字段
	v.validateBasicFields(result)

	// 验证数值范围
	v.validateNumericRanges(result)

	// 验证枚举值
	v.validateEnumValues(result)

	// 验证时间相关配置
	v.validateTimeConfigs(result)

	// 验证依赖关系
	v.validateDependencies(result)

	return result
}

// validateBasicFields 验证基本字段
func (v *Validator) validateBasicFields(result *ValidationResult) {
	// 验证命名空间
	if v.config.Namespace == "" {
		result.AddError("namespace", "namespace cannot be empty", v.config.Namespace)
	} else if len(v.config.Namespace) > 64 {
		result.AddError("namespace", "namespace length must not exceed 64 characters", v.config.Namespace)
	} else {
		// 验证命名空间格式（只允许字母、数字、下划线、连字符）
		namespaceRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
		if !namespaceRegex.MatchString(v.config.Namespace) {
			result.AddError("namespace", "namespace can only contain letters, numbers, underscores, and hyphens", v.config.Namespace)
		}
	}

	// 验证 Token（如果提供）
	if v.config.Token != "" && len(v.config.Token) > 1024 {
		result.AddError("token", "token length must not exceed 1024 characters", v.config.Token)
	}
	if v.config.Token != "" && len(v.config.Token) < 8 {
		result.AddError("token", "token must be at least 8 characters long", v.config.Token)
	}
}

// validateNumericRanges 验证数值范围
func (v *Validator) validateNumericRanges(result *ValidationResult) {
	// 验证权重
	if v.config.Weight < conf.MinWeight || v.config.Weight > conf.MaxWeight {
		result.AddError("weight", fmt.Sprintf("weight must be between %d and %d", conf.MinWeight, conf.MaxWeight), v.config.Weight)
	}

	// 验证 TTL
	if v.config.Ttl < conf.MinTTL || v.config.Ttl > conf.MaxTTL {
		result.AddError("ttl", fmt.Sprintf("ttl must be between %d and %d seconds", conf.MinTTL, conf.MaxTTL), v.config.Ttl)
	}
}

// validateEnumValues 验证枚举值
func (v *Validator) validateEnumValues(result *ValidationResult) {
	// 当前配置中没有枚举值字段，跳过验证
}

// validateTimeConfigs 验证时间相关配置
func (v *Validator) validateTimeConfigs(result *ValidationResult) {
	// 验证超时时间
	if v.config.Timeout != nil {
		timeout := time.Duration(v.config.Timeout.Seconds) * time.Second
		if timeout < time.Duration(conf.MinTimeoutSeconds)*time.Second || timeout > time.Duration(conf.MaxTimeoutSeconds)*time.Second {
			result.AddError("timeout", fmt.Sprintf("timeout must be between %d and %d seconds", conf.MinTimeoutSeconds, conf.MaxTimeoutSeconds), timeout)
		}
	}
}

// validateDependencies 验证依赖关系
func (v *Validator) validateDependencies(result *ValidationResult) {
	// 权重和 TTL 的合理性验证
	if v.config.Weight > 0 && v.config.Ttl < 5 {
		result.AddError("ttl", "TTL should be at least 5 seconds when weight is greater than 0", v.config.Ttl)
	}

	// 超时和 TTL 的协调性验证
	if v.config.Timeout != nil && v.config.Ttl > 0 {
		timeout := time.Duration(v.config.Timeout.Seconds) * time.Second
		ttlDuration := time.Duration(v.config.Ttl) * time.Second

		if timeout >= ttlDuration {
			result.AddError("timeout", "Timeout should be less than TTL to ensure proper operation", timeout)
		}
	}

	// 命名空间和服务的协调性验证
	if v.config.Namespace == "default" && v.config.Token != "" {
		result.AddError("token", "Token should not be required for default namespace", v.config.Token)
	}
}

// contains 检查切片是否包含指定值
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// ValidateConfig 便捷的配置验证函数
func ValidateConfig(config *conf.Polaris) error {
	validator := NewValidator(config)
	result := validator.Validate()

	if !result.IsValid {
		return fmt.Errorf("configuration validation failed: %s", result.Error())
	}

	return nil
}

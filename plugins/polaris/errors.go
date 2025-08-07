package polaris

import (
	"fmt"
	"strings"
)

// ErrorCode 错误码类型
type ErrorCode string

// 错误码常量
const (
	// 配置相关错误
	ErrCodeConfigInvalid    ErrorCode = "CONFIG_INVALID"
	ErrCodeConfigMissing    ErrorCode = "CONFIG_MISSING"
	ErrCodeConfigValidation ErrorCode = "CONFIG_VALIDATION"

	// 初始化相关错误
	ErrCodeInitFailed         ErrorCode = "INIT_FAILED"
	ErrCodeAlreadyInitialized ErrorCode = "ALREADY_INITIALIZED"
	ErrCodeNotInitialized     ErrorCode = "NOT_INITIALIZED"

	// SDK 相关错误
	ErrCodeSDKContextFailed ErrorCode = "SDK_CONTEXT_FAILED"
	ErrCodeAPIInitFailed    ErrorCode = "API_INIT_FAILED"
	ErrCodeSDKDestroyed     ErrorCode = "SDK_DESTROYED"

	// 服务相关错误
	ErrCodeServiceNotFound       ErrorCode = "SERVICE_NOT_FOUND"
	ErrCodeServiceUnavailable    ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeServiceRegistration   ErrorCode = "SERVICE_REGISTRATION"
	ErrCodeServiceDeregistration ErrorCode = "SERVICE_DEREGISTRATION"

	// 配置管理相关错误
	ErrCodeConfigNotFound    ErrorCode = "CONFIG_NOT_FOUND"
	ErrCodeConfigGetFailed   ErrorCode = "CONFIG_GET_FAILED"
	ErrCodeConfigWatchFailed ErrorCode = "CONFIG_WATCH_FAILED"

	// 限流相关错误
	ErrCodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrCodeRateLimitFailed   ErrorCode = "RATE_LIMIT_FAILED"

	// 健康检查相关错误
	ErrCodeHealthCheckFailed  ErrorCode = "HEALTH_CHECK_FAILED"
	ErrCodeHealthCheckTimeout ErrorCode = "HEALTH_CHECK_TIMEOUT"

	// 网络相关错误
	ErrCodeNetworkError     ErrorCode = "NETWORK_ERROR"
	ErrCodeTimeout          ErrorCode = "TIMEOUT"
	ErrCodeConnectionFailed ErrorCode = "CONNECTION_FAILED"

	// 重试相关错误
	ErrCodeRetryExhausted     ErrorCode = "RETRY_EXHAUSTED"
	ErrCodeCircuitBreakerOpen ErrorCode = "CIRCUIT_BREAKER_OPEN"

	// 监听器相关错误
	ErrCodeWatcherFailed  ErrorCode = "WATCHER_FAILED"
	ErrCodeWatcherTimeout ErrorCode = "WATCHER_TIMEOUT"

	// 监控相关错误
	ErrCodeMetricsFailed ErrorCode = "METRICS_FAILED"

	// 优雅关闭相关错误
	ErrCodeShutdownFailed  ErrorCode = "SHUTDOWN_FAILED"
	ErrCodeShutdownTimeout ErrorCode = "SHUTDOWN_TIMEOUT"
)

// PolarisError Polaris 插件错误
type PolarisError struct {
	Code    ErrorCode
	Message string
	Cause   error
	Context map[string]interface{}
}

// NewPolarisError 创建新的 Polaris 错误
func NewPolarisError(code ErrorCode, message string) *PolarisError {
	return &PolarisError{
		Code:    code,
		Message: message,
		Context: make(map[string]interface{}),
	}
}

// WithCause 设置错误原因
func (e *PolarisError) WithCause(cause error) *PolarisError {
	e.Cause = cause
	return e
}

// WithContext 添加上下文信息
func (e *PolarisError) WithContext(key string, value interface{}) *PolarisError {
	e.Context[key] = value
	return e
}

// Error 实现 error 接口
func (e *PolarisError) Error() string {
	var parts []string

	// 添加错误码
	parts = append(parts, fmt.Sprintf("[%s]", e.Code))

	// 添加错误消息
	parts = append(parts, e.Message)

	// 添加原因
	if e.Cause != nil {
		parts = append(parts, fmt.Sprintf("caused by: %v", e.Cause))
	}

	// 添加上下文
	if len(e.Context) > 0 {
		var contextParts []string
		for k, v := range e.Context {
			contextParts = append(contextParts, fmt.Sprintf("%s=%v", k, v))
		}
		parts = append(parts, fmt.Sprintf("context: {%s}", strings.Join(contextParts, ", ")))
	}

	return strings.Join(parts, " ")
}

// Unwrap 返回错误原因
func (e *PolarisError) Unwrap() error {
	return e.Cause
}

// Is 检查错误类型
func (e *PolarisError) Is(target error) bool {
	if targetError, ok := target.(*PolarisError); ok {
		return e.Code == targetError.Code
	}
	return false
}

// 便捷错误创建函数

// NewConfigError 创建配置错误
func NewConfigError(message string) *PolarisError {
	return NewPolarisError(ErrCodeConfigInvalid, message)
}

// NewInitError 创建初始化错误
func NewInitError(message string) *PolarisError {
	return NewPolarisError(ErrCodeInitFailed, message)
}

// NewServiceError 创建服务错误
func NewServiceError(code ErrorCode, message string) *PolarisError {
	return NewPolarisError(code, message)
}

// NewNetworkError 创建网络错误
func NewNetworkError(message string) *PolarisError {
	return NewPolarisError(ErrCodeNetworkError, message)
}

// NewTimeoutError 创建超时错误
func NewTimeoutError(operation string) *PolarisError {
	return NewPolarisError(ErrCodeTimeout, fmt.Sprintf("operation '%s' timed out", operation))
}

// NewRetryError 创建重试错误
func NewRetryError(message string) *PolarisError {
	return NewPolarisError(ErrCodeRetryExhausted, message)
}

// NewHealthCheckError 创建健康检查错误
func NewHealthCheckError(message string) *PolarisError {
	return NewPolarisError(ErrCodeHealthCheckFailed, message)
}

// 错误检查函数

// IsConfigError 检查是否为配置错误
func IsConfigError(err error) bool {
	return isErrorCode(err, ErrCodeConfigInvalid, ErrCodeConfigMissing, ErrCodeConfigValidation)
}

// IsInitError 检查是否为初始化错误
func IsInitError(err error) bool {
	return isErrorCode(err, ErrCodeInitFailed, ErrCodeAlreadyInitialized, ErrCodeNotInitialized)
}

// IsServiceError 检查是否为服务错误
func IsServiceError(err error) bool {
	return isErrorCode(err, ErrCodeServiceNotFound, ErrCodeServiceUnavailable, ErrCodeServiceRegistration, ErrCodeServiceDeregistration)
}

// IsNetworkError 检查是否为网络错误
func IsNetworkError(err error) bool {
	return isErrorCode(err, ErrCodeNetworkError, ErrCodeTimeout, ErrCodeConnectionFailed)
}

// IsRetryError 检查是否为重试错误
func IsRetryError(err error) bool {
	return isErrorCode(err, ErrCodeRetryExhausted, ErrCodeCircuitBreakerOpen)
}

// isErrorCode 检查错误码
func isErrorCode(err error, codes ...ErrorCode) bool {
	if polarisErr, ok := err.(*PolarisError); ok {
		for _, code := range codes {
			if polarisErr.Code == code {
				return true
			}
		}
	}
	return false
}

// 错误包装函数

// WrapError 包装错误
func WrapError(err error, code ErrorCode, message string) *PolarisError {
	return NewPolarisError(code, message).WithCause(err)
}

// WrapConfigError 包装配置错误
func WrapConfigError(err error, message string) *PolarisError {
	return WrapError(err, ErrCodeConfigInvalid, message)
}

// WrapInitError 包装初始化错误
func WrapInitError(err error, message string) *PolarisError {
	return WrapError(err, ErrCodeInitFailed, message)
}

// WrapServiceError 包装服务错误
func WrapServiceError(err error, code ErrorCode, message string) *PolarisError {
	return WrapError(err, code, message)
}

// WrapNetworkError 包装网络错误
func WrapNetworkError(err error, message string) *PolarisError {
	return WrapError(err, ErrCodeNetworkError, message)
}

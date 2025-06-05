package plugins

import (
	"errors"
	"fmt"
)

// Common error variables for plugin-related operations
// 插件相关操作的通用错误变量
var (
	// ErrPluginNotFound indicates that a requested plugin could not be found in the system
	// This error occurs when attempting to access or operate on a non-existent plugin
	// ErrPluginNotFound 表示在系统中找不到请求的插件。
	// 当尝试访问或操作一个不存在的插件时会出现此错误。
	ErrPluginNotFound = errors.New("plugin not found")

	// ErrPluginAlreadyExists indicates an attempt to register a plugin with an ID that is already in use
	// This error helps maintain unique plugin identifiers across the system
	// ErrPluginAlreadyExists 表示尝试注册一个使用了已存在 ID 的插件。
	// 此错误有助于确保系统中插件标识符的唯一性。
	ErrPluginAlreadyExists = errors.New("plugin already exists")

	// ErrPluginNotInitialized indicates an attempt to use a plugin that hasn't been properly initialized
	// Operations on uninitialized plugins are not allowed to prevent undefined behavior
	// ErrPluginNotInitialized 表示尝试使用一个未正确初始化的插件。
	// 为避免未定义行为，不允许对未初始化的插件进行操作。
	ErrPluginNotInitialized = errors.New("plugin not initialized")

	// ErrPluginNotActive indicates an attempt to use a plugin that is not in the active state
	// The plugin must be in StatusActive to perform the requested operation
	// ErrPluginNotActive 表示尝试使用一个未处于活动状态的插件。
	// 插件必须处于活动状态才能执行请求的操作。
	ErrPluginNotActive = errors.New("plugin not active")

	// ErrPluginAlreadyActive indicates an attempt to start an already active plugin
	// Prevents duplicate activation of plugins
	// ErrPluginAlreadyActive 表示尝试启动一个已经处于活动状态的插件。
	// 用于防止插件被重复激活。
	ErrPluginAlreadyActive = errors.New("plugin already active")

	// ErrInvalidPluginID indicates that the provided plugin ID is invalid
	// Plugin IDs must follow specific formatting rules and be non-empty
	// ErrInvalidPluginID 表示提供的插件 ID 无效。
	// 插件 ID 必须遵循特定的格式规则且不能为空。
	ErrInvalidPluginID = errors.New("invalid plugin ID")

	// ErrInvalidPluginVersion indicates that the provided plugin version is invalid
	// Version strings must follow semantic versioning format
	// ErrInvalidPluginVersion 表示提供的插件版本无效。
	// 版本字符串必须遵循语义化版本格式。
	ErrInvalidPluginVersion = errors.New("invalid plugin version")

	// ErrInvalidPluginConfig indicates that the provided plugin configuration is invalid
	// Configuration must meet the plugin's specific requirements
	// ErrInvalidPluginConfig 表示提供的插件配置无效。
	// 配置必须满足插件的特定要求。
	ErrInvalidPluginConfig = errors.New("invalid plugin configuration")

	// ErrInvalidConfiguration indicates that the provided configuration is not of the expected type
	// This error occurs when attempting to configure a plugin with an incompatible configuration type
	// ErrInvalidConfiguration 表示提供的配置不是预期的类型。
	// 当尝试使用不兼容的配置类型来配置插件时会出现此错误。
	ErrInvalidConfiguration = errors.New("invalid configuration type")

	// ErrPluginDependencyNotMet indicates that one or more plugin dependencies are not satisfied
	// All required dependencies must be available and properly configured
	// ErrPluginDependencyNotMet 表示一个或多个插件依赖项未满足。
	// 所有必需的依赖项必须可用且已正确配置。
	ErrPluginDependencyNotMet = errors.New("plugin dependency not met")

	// ErrPluginUpgradeNotSupported indicates that the plugin does not support the requested upgrade operation
	// The plugin must implement the Upgradable interface and support the specific upgrade capability
	// ErrPluginUpgradeNotSupported 表示插件不支持请求的升级操作。
	// 插件必须实现 Upgradable 接口并支持特定的升级能力。
	ErrPluginUpgradeNotSupported = errors.New("plugin upgrade not supported")

	// ErrPluginUpgradeFailed indicates that the plugin upgrade process failed
	// Contains details about the specific failure in upgrade process
	// ErrPluginUpgradeFailed 表示插件升级过程失败。
	// 包含升级过程中具体失败的详细信息。
	ErrPluginUpgradeFailed = errors.New("plugin upgrade failed")

	// ErrPluginResourceNotFound indicates that a requested plugin resource is not available
	// The resource must be registered before it can be accessed
	// ErrPluginResourceNotFound 表示请求的插件资源不可用。
	// 资源必须先注册才能被访问。
	ErrPluginResourceNotFound = errors.New("plugin resource not found")

	// ErrPluginResourceInvalid indicates that a plugin resource is in an invalid state
	// The resource must be properly initialized and maintained
	// ErrPluginResourceInvalid 表示插件资源处于无效状态。
	// 资源必须正确初始化并维护。
	ErrPluginResourceInvalid = errors.New("plugin resource invalid")

	// ErrPluginOperationTimeout indicates that a plugin operation exceeded its time limit
	// Operations must complete within their specified timeout period
	// ErrPluginOperationTimeout 表示插件操作超过了其时间限制。
	// 操作必须在指定的超时时间内完成。
	ErrPluginOperationTimeout = errors.New("plugin operation timeout")

	// ErrPluginOperationCancelled indicates that a plugin operation was cancelled
	// The operation was terminated before completion, either by user request or system action
	// ErrPluginOperationCancelled 表示插件操作被取消。
	// 该操作在完成前被终止，可能是用户请求或系统操作导致的。
	ErrPluginOperationCancelled = errors.New("plugin operation cancelled")

	// ErrPluginHealthCheckFailed indicates that the plugin's health check failed
	// The plugin is in an unhealthy state and may need attention
	// ErrPluginHealthCheckFailed 表示插件的健康检查失败。
	// 插件处于不健康状态，可能需要关注。
	ErrPluginHealthCheckFailed = errors.New("plugin health check failed")

	// ErrPluginSecurityViolation indicates a security-related violation in the plugin
	// Security policies or constraints have been breached
	// ErrPluginSecurityViolation 表示插件中发生了与安全相关的违规行为。
	// 安全策略或约束已被违反。
	ErrPluginSecurityViolation = errors.New("plugin security violation")
)

// PluginError represents a detailed error that occurred during plugin operations
// PluginError 表示插件操作过程中发生的详细错误。
type PluginError struct {
	// PluginID identifies the plugin where the error occurred
	// PluginID 标识出错的插件。
	PluginID string

	// Operation describes the action that was being performed when the error occurred
	// Operation 描述出错时正在执行的操作。
	Operation string

	// Message provides a detailed description of the error
	// Message 提供错误的详细描述。
	Message string

	// Err is the underlying error that caused this PluginError
	// Err 是导致此 PluginError 的底层错误。
	Err error
}

// Error implements the error interface for PluginError
// Returns a formatted error message including plugin ID, operation, and details
// Error 为 PluginError 实现 error 接口。
// 返回包含插件 ID、操作和详细信息的格式化错误消息。
func (e *PluginError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("plugin %s: %s failed: %s (%v)", e.PluginID, e.Operation, e.Message, e.Err)
	}
	return fmt.Sprintf("plugin %s: %s failed: %s", e.PluginID, e.Operation, e.Message)
}

// Unwrap implements the errors unwrap interface
// Returns the underlying error for error chain handling
// Unwrap 实现 errors 解包接口。
// 返回底层错误以进行错误链处理。
func (e *PluginError) Unwrap() error {
	return e.Err
}

// NewPluginError creates a new PluginError with the given details
// Provides a convenient way to create structured plugin errors
// NewPluginError 使用给定的详细信息创建一个新的 PluginError。
// 提供了一种方便的方式来创建结构化的插件错误。
func NewPluginError(pluginID, operation, message string, err error) *PluginError {
	return &PluginError{
		PluginID:  pluginID,
		Operation: operation,
		Message:   message,
		Err:       err,
	}
}

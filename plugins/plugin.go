package plugins

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
)

// PluginStatus represents the current operational status of a plugin in the system.
// It tracks the plugin's lifecycle state from initialization through termination.
// PluginStatus 表示系统中插件的当前运行状态。
// 它跟踪插件从初始化到终止的整个生命周期状态。
type PluginStatus int

const (
	// StatusInactive indicates that the plugin is loaded but not yet initialized
	// This is the initial state of a plugin when it is first loaded into the system
	// StatusInactive 表示插件已加载但尚未初始化。
	// 这是插件首次加载到系统中的初始状态。
	StatusInactive PluginStatus = iota

	// StatusInitializing indicates that the plugin is currently performing initialization
	// During this state, the plugin is setting up resources, establishing connections,
	// and preparing for normal operation
	// StatusInitializing 表示插件当前正在进行初始化。
	// 在此状态下，插件会设置资源、建立连接并为正常运行做准备。
	StatusInitializing

	// StatusActive indicates that the plugin is fully operational and running normally
	// In this state, the plugin is processing requests and performing its intended functions
	// StatusActive 表示插件已完全投入运行且正常工作。
	// 在此状态下，插件会处理请求并执行其预定功能。
	StatusActive

	// StatusSuspended indicates that the plugin is temporarily paused
	// The plugin retains its resources but is not processing new requests
	// Can be resumed to StatusActive without full reinitialization
	// StatusSuspended 表示插件已被暂时暂停。
	// 插件会保留其资源，但不会处理新请求。
	// 可以在不进行完全重新初始化的情况下恢复到 StatusActive 状态。
	StatusSuspended

	// StatusStopping indicates that the plugin is in the process of shutting down
	// During this state, the plugin is cleaning up resources and finishing pending operations
	// StatusStopping 表示插件正在关闭过程中。
	// 在此状态下，插件会清理资源并完成未完成的操作。
	StatusStopping

	// StatusTerminated indicates that the plugin has been gracefully shut down
	// All resources have been released and connections closed
	// Requires full reinitialization to become active again
	// StatusTerminated 表示插件已正常关闭。
	// 所有资源已释放，连接已关闭。
	// 若要再次激活，需要进行完全重新初始化。
	StatusTerminated

	// StatusFailed indicates that the plugin has encountered a fatal error
	// The plugin is non-operational and may require manual intervention
	// Should transition to StatusTerminated or attempt recovery
	// StatusFailed 表示插件遇到了致命错误。
	// 插件无法正常工作，可能需要手动干预。
	// 应转换到 StatusTerminated 状态或尝试恢复。
	StatusFailed

	// StatusUpgrading indicates that the plugin is currently being upgraded
	// During this state, the plugin may be partially operational
	// Should transition to StatusActive or StatusFailed
	// StatusUpgrading 表示插件当前正在升级。
	// 在此状态下，插件可能部分可用。
	// 升级后应转换到 StatusActive 或 StatusFailed 状态。
	StatusUpgrading

	// StatusRollback indicates that the plugin is rolling back from a failed upgrade
	// Attempting to restore the previous working state
	// Should transition to StatusActive or StatusFailed
	// StatusRollback 表示插件正在从失败的升级中回滚。
	// 尝试恢复到之前的工作状态。
	// 回滚后应转换到 StatusActive 或 StatusFailed 状态。
	StatusRollback
)

// UpgradeCapability defines the various ways a plugin can be upgraded during runtime
// UpgradeCapability 定义了插件在运行时可进行升级的各种方式。
type UpgradeCapability int

const (
	// UpgradeNone indicates the plugin does not support any runtime upgrades
	// Must be stopped and restarted to apply any changes
	// UpgradeNone 表示插件不支持任何运行时升级。
	// 若要应用任何更改，必须停止并重新启动插件。
	UpgradeNone UpgradeCapability = iota

	// UpgradeConfig indicates the plugin can update its configuration without restart
	// Supports runtime configuration changes but not code updates
	// UpgradeConfig 表示插件可以在不重启的情况下更新其配置。
	// 支持运行时配置更改，但不支持代码更新。
	UpgradeConfig

	// UpgradeVersion indicates the plugin can perform version upgrades without restart
	// Supports both configuration and code updates during runtime
	// UpgradeVersion 表示插件可以在不重启的情况下进行版本升级。
	// 支持在运行时同时更新配置和代码。
	UpgradeVersion

	// UpgradeReplace indicates the plugin supports complete replacement during runtime
	// Can be entirely replaced with a new instance while maintaining service
	// UpgradeReplace 表示插件支持在运行时进行完全替换。
	// 可以在保持服务运行的同时，用新实例完全替换当前插件。
	UpgradeReplace
)

// Plugin is the minimal interface that all plugins must implement
// It combines basic metadata and lifecycle management capabilities
// Plugin 是所有插件都必须实现的最小接口。
// 它整合了基本的元数据和生命周期管理功能。
type Plugin interface {
	Metadata
	Lifecycle
	LifecycleSteps
	DependencyAware
}

// Metadata defines methods for retrieving plugin metadata
// This interface provides essential information about the plugin
// Metadata 定义了用于获取插件元数据的方法。
// 此接口提供了有关插件的基本信息。
type Metadata interface {
	// ID returns the unique identifier of the plugin
	// This ID must be unique across all plugins in the system
	// ID 返回插件的唯一标识符。
	// 该 ID 在系统中的所有插件中必须是唯一的。
	ID() string

	// Name returns the display name of the plugin
	// This is a human-readable name used for display purposes
	// Name 返回插件的显示名称。
	// 这是一个用于显示目的的易读名称。
	Name() string

	// Description returns a detailed description of the plugin
	// Should provide information about the plugin's purpose and functionality
	// Description 返回插件的详细描述。
	// 应提供有关插件用途和功能的信息。
	Description() string

	// Version returns the semantic version of the plugin
	// Should follow semver format (MAJOR.MINOR.PATCH)
	// Version 返回插件的语义化版本。
	// 应遵循语义化版本格式（MAJOR.MINOR.PATCH）。
	Version() string
}

// Lifecycle defines the basic lifecycle methods for a plugin
// Handles initialization, operation, and termination of the plugin
// Lifecycle 定义了插件的基本生命周期方法。
// 处理插件的初始化、运行和终止操作。
type Lifecycle interface {
	// Initialize prepares the plugin for use
	// Sets up resources, connections, and internal state
	// Returns error if initialization fails
	// Initialize 为插件的使用做准备。
	// 设置资源、连接和内部状态。
	// 如果初始化失败，则返回错误。
	Initialize(plugin Plugin, rt Runtime) error

	// Start begins the plugin's main functionality
	// Should only be called after successful initialization
	// Returns error if startup fails
	// Start 启动插件的主要功能。
	// 仅应在初始化成功后调用。
	// 如果启动失败，则返回错误。
	Start(plugin Plugin) error

	// Stop gracefully terminates the plugin's functionality
	// Releases resources and closes connections
	// Returns error if shutdown fails
	// Stop 优雅地终止插件的功能。
	// 释放资源并关闭连接。
	// 如果关闭失败，则返回错误。
	Stop(plugin Plugin) error

	// Status returns the current status of the plugin
	// Provides real-time state information
	// Status 返回插件的当前状态。
	// 提供实时状态信息。
	Status(plugin Plugin) PluginStatus
}

type LifecycleSteps interface {
	InitializeResources(rt Runtime) error
	StartupTasks() error
	CleanupTasks() error
	CheckHealth() error
}

// ResourceManager provides access to shared plugin resources
// Manages resource allocation, sharing, and lifecycle
// ResourceManager 提供对插件共享资源的访问。
// 管理资源分配、共享和生命周期。
type ResourceManager interface {
	// GetResource retrieves a shared plugin resource by name
	// Returns the resource and any error encountered
	// GetResource 根据名称获取插件共享资源。
	// 返回资源和可能遇到的错误。
	GetResource(name string) (any, error)

	// RegisterResource registers a resource to be shared with other plugins
	// Returns error if registration fails
	// RegisterResource 注册一个资源，以便与其他插件共享。
	// 如果注册失败，则返回错误。
	RegisterResource(name string, resource any) error
}

// ConfigProvider provides access to plugin configuration
// Manages plugin configuration loading and access
// ConfigProvider 提供对插件配置的访问。
// 管理插件配置的加载和访问。
type ConfigProvider interface {
	// GetConfig returns the plugin configuration manager
	// Provides access to configuration values and updates
	// GetConfig 返回插件配置管理器。
	// 提供对配置值和更新的访问。
	GetConfig() config.Config
}

// LogProvider provides access to logging functionality
// Manages plugin logging capabilities
// LogProvider 提供对日志记录功能的访问。
// 管理插件的日志记录能力。
type LogProvider interface {
	// GetLogger returns the plugin logger instance
	// Provides structured logging capabilities
	// GetLogger 返回插件日志记录器实例。
	// 提供结构化的日志记录功能。
	GetLogger() log.Logger
}

// Runtime combines all runtime capabilities for plugins
// Provides a complete runtime environment for plugin operation
// Runtime 整合了插件的所有运行时功能。
// 为插件运行提供完整的运行时环境。
type Runtime interface {
	ResourceManager
	ConfigProvider
	LogProvider
	EventEmitter
}

// Suspendable defines methods for temporary plugin suspension
// Manages temporary plugin deactivation and reactivation
// Suspendable 定义了插件临时暂停的方法。
// 管理插件的临时停用和重新激活。
type Suspendable interface {
	// Suspend temporarily suspends plugin operations
	// Pauses plugin activity while maintaining state
	// Suspend 临时暂停插件操作。
	// 在保持状态的同时暂停插件活动。
	Suspend() error

	// Resume restores plugin operations from a suspended state
	// Resumes normal operation without reinitialization
	// Resume 从暂停状态恢复插件操作。
	// 无需重新初始化即可恢复正常运行。
	Resume() error
}

// Configurable defines methods for plugin configuration management
// Manages plugin configuration updates and validation
// Configurable 定义了插件配置管理的方法。
// 管理插件配置的更新和验证。
type Configurable interface {
	// Configure applies and validates the given configuration
	// Updates plugin configuration during runtime
	// Configure 应用并验证给定的配置。
	// 在运行时更新插件配置。
	Configure(conf any) error
}

// DependencyAware defines methods for plugin dependency management
// Manages plugin dependencies and their relationships
// DependencyAware 定义了插件依赖管理的方法。
// 管理插件依赖项及其关系。
type DependencyAware interface {
	// GetDependencies returns the list of plugin dependencies
	// Lists all required and optional dependencies
	// GetDependencies 返回插件依赖项列表。
	// 列出所有必需和可选的依赖项。
	GetDependencies() []Dependency
}

// EventHandler defines methods for plugin event handling
// Processes plugin-related events and notifications
// EventHandler 定义了插件事件处理的方法。
// 处理与插件相关的事件和通知。
type EventHandler interface {
	// HandleEvent processes plugin lifecycle events
	// Handles various plugin system events
	// HandleEvent 处理插件生命周期事件。
	// 处理各种插件系统事件。
	HandleEvent(event PluginEvent)
}

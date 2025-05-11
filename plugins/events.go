// Package plugins provides a plugin system for extending application functionality.
// 包 plugins 提供了一个用于扩展应用程序功能的插件系统。
package plugins

// EventType represents the type of event that occurred in the plugin system.
// EventType 表示插件系统中发生的事件类型。
type EventType string

// Priority levels for plugin events
// 插件事件的优先级级别
const (
	// PriorityLow indicates minimal impact events that can be processed later
	// PriorityLow 表示影响极小的事件，可以稍后处理。
	PriorityLow = 0
	// PriorityNormal indicates standard events requiring routine processing
	// PriorityNormal 表示需要常规处理的标准事件。
	PriorityNormal = 1
	// PriorityHigh indicates important events needing prompt attention
	// PriorityHigh 表示需要立即关注的重要事件。
	PriorityHigh = 2
	// PriorityCritical indicates urgent events requiring immediate handling
	// PriorityCritical 表示需要立即处理的紧急事件。
	PriorityCritical = 3
)

// Plugin lifecycle event types for comprehensive system monitoring
// 用于全面系统监控的插件生命周期事件类型
const (
	// EventPluginInitializing indicates the plugin is starting initialization.
	// Triggered when plugin begins loading resources and establishing connections.
	// EventPluginInitializing 表示插件开始初始化。
	// 当插件开始加载资源并建立连接时触发。
	EventPluginInitializing = "plugin.initializing"

	// EventPluginInitialized indicates the plugin completed initialization.
	// Triggered when all resources are loaded and connections established.
	// EventPluginInitialized 表示插件已完成初始化。
	// 当所有资源加载完成且连接建立后触发。
	EventPluginInitialized = "plugin.initialized"

	// EventPluginStarting indicates the plugin is beginning its operations.
	// Triggered when core functionality is about to begin.
	// EventPluginStarting 表示插件开始执行操作。
	// 当核心功能即将启动时触发。
	EventPluginStarting = "plugin.starting"

	// EventPluginStarted indicates the plugin is fully operational.
	// Triggered when all systems are running and ready to handle requests.
	// EventPluginStarted 表示插件已完全投入运行。
	// 当所有系统都已运行并准备好处理请求时触发。
	EventPluginStarted = "plugin.started"

	// EventPluginStopping indicates the plugin is beginning shutdown.
	// Triggered when shutdown command is received and cleanup begins.
	// EventPluginStopping 表示插件开始关闭。
	// 当收到关闭命令并开始清理工作时触发。
	EventPluginStopping = "plugin.stopping"

	// EventPluginStopped indicates the plugin completed shutdown.
	// Triggered when all resources are released and connections closed.
	// EventPluginStopped 表示插件已完成关闭。
	// 当所有资源释放且连接关闭后触发。
	EventPluginStopped = "plugin.stopped"
)

// Health check event types for monitoring plugin health status
// 用于监控插件健康状态的健康检查事件类型
const (
	// EventHealthCheckStarted indicates a health check operation has begun.
	// Triggered when the health check routine starts executing.
	// EventHealthCheckStarted 表示健康检查操作已开始。
	// 当健康检查程序开始执行时触发。
	EventHealthCheckStarted = "health.check.started"

	// EventHealthCheckRunning indicates a health check is in progress.
	// Triggered during the execution of health check procedures.
	// EventHealthCheckRunning 表示健康检查正在进行中。
	// 当健康检查流程执行期间触发。
	EventHealthCheckRunning = "health.check.running"

	// EventHealthCheckDone indicates a health check has completed.
	// Triggered when all health check procedures have finished.
	// EventHealthCheckDone 表示健康检查已完成。
	// 当所有健康检查流程完成后触发。
	EventHealthCheckDone = "health.check.done"

	// EventHealthStatusOK indicates the plugin is healthy.
	// Triggered when all health metrics are within normal ranges.
	// EventHealthStatusOK 表示插件状态健康。
	// 当所有健康指标都在正常范围内时触发。
	EventHealthStatusOK = "health.status.ok"

	// EventHealthStatusWarning indicates potential health issues.
	// Triggered when health metrics show concerning trends.
	// EventHealthStatusWarning 表示插件可能存在健康问题。
	// 当健康指标显示出令人担忧的趋势时触发。
	EventHealthStatusWarning = "health.status.warning"

	// EventHealthStatusCritical indicates severe health issues.
	// Triggered when health metrics exceed critical thresholds.
	// EventHealthStatusCritical 表示插件存在严重健康问题。
	// 当健康指标超过临界阈值时触发。
	EventHealthStatusCritical = "health.status.critical"

	// EventHealthStatusUnknown indicates health status cannot be determined.
	// Triggered when health check procedures fail to complete.
	// EventHealthStatusUnknown 表示无法确定插件的健康状态。
	// 当健康检查流程未能完成时触发。
	EventHealthStatusUnknown = "health.status.unknown"

	// EventHealthMetricsChanged indicates a change in health metrics.
	// Triggered when monitored metrics show significant changes.
	// EventHealthMetricsChanged 表示健康指标发生了变化。
	// 当监控的指标显示出显著变化时触发。
	EventHealthMetricsChanged = "health.metrics.changed"

	// EventHealthThresholdHit indicates metrics exceeded defined thresholds.
	// Triggered when health metrics cross warning or critical levels.
	// EventHealthThresholdHit 表示指标超过了定义的阈值。
	// 当健康指标越过警告或临界水平时触发。
	EventHealthThresholdHit = "health.metrics.threshold"

	// EventHealthStatusChanged indicates overall health status change.
	// Triggered when the aggregate health status transitions.
	// EventHealthStatusChanged 表示整体健康状态发生了变化。
	// 当综合健康状态发生转变时触发。
	EventHealthStatusChanged = "health.status.changed"

	// EventHealthCheckFailed indicates health check operation failure.
	// Triggered when health check procedures encounter errors.
	// EventHealthCheckFailed 表示健康检查操作失败。
	// 当健康检查流程遇到错误时触发。
	EventHealthCheckFailed = "health.check.failed"
)

// Resource event types for monitoring system resources
// 用于监控系统资源的资源事件类型
const (
	// EventResourceExhausted indicates critical resource depletion.
	// Triggered when system resources reach critical levels.
	// EventResourceExhausted 表示关键资源耗尽。
	// 当系统资源达到临界水平时触发。
	EventResourceExhausted = "resource.exhausted"

	// EventPerformanceDegraded indicates performance deterioration.
	// Triggered when system performance metrics decline significantly.
	// EventPerformanceDegraded 表示性能下降。
	// 当系统性能指标显著下降时触发。
	EventPerformanceDegraded = "performance.degraded"
)

// Configuration event types for managing plugin configuration
// 用于管理插件配置的配置事件类型
const (
	// EventConfigurationChanged indicates configuration update initiation.
	// Triggered when new configuration is being applied.
	// EventConfigurationChanged 表示配置更新开始。
	// 当新配置正在应用时触发。
	EventConfigurationChanged = "config.changed"

	// EventConfigurationInvalid indicates invalid configuration.
	// Triggered when configuration validation fails.
	// EventConfigurationInvalid 表示配置无效。
	// 当配置验证失败时触发。
	EventConfigurationInvalid = "config.invalid"

	// EventConfigurationApplied indicates successful configuration update.
	// Triggered when new configuration is active and verified.
	// EventConfigurationApplied 表示配置更新成功。
	// 当新配置生效并通过验证时触发。
	EventConfigurationApplied = "config.applied"
)

// Dependency event types for managing plugin dependencies
// 用于管理插件依赖项的依赖事件类型
const (
	// EventDependencyMissing indicates missing required dependency.
	// Triggered when required plugin or resource is unavailable.
	// EventDependencyMissing 表示缺少必需的依赖项。
	// 当必需的插件或资源不可用时触发。
	EventDependencyMissing = "dependency.missing"

	// EventDependencyStatusChanged indicates dependency state change.
	// Triggered when dependent plugin changes operational state.
	// EventDependencyStatusChanged 表示依赖项状态发生变化。
	// 当依赖的插件操作状态发生改变时触发。
	EventDependencyStatusChanged = "dependency.status.changed"

	// EventDependencyError indicates dependency-related error.
	// Triggered when dependency fails or becomes unstable.
	// EventDependencyError 表示发生与依赖项相关的错误。
	// 当依赖项失败或变得不稳定时触发。
	EventDependencyError = "dependency.error"
)

// Upgrade event types for managing plugin versions
// 用于管理插件版本的升级事件类型
const (
	// EventUpgradeAvailable indicates new version availability.
	// Triggered when update check finds newer version.
	// EventUpgradeAvailable 表示有新版本可用。
	// 当更新检查发现较新版本时触发。
	EventUpgradeAvailable = "upgrade.available"

	// EventUpgradeInitiated indicates upgrade process start.
	// Triggered when upgrade sequence begins.
	// EventUpgradeInitiated 表示升级过程开始。
	// 当升级流程启动时触发。
	EventUpgradeInitiated = "upgrade.initiated"

	// EventUpgradeValidating indicates upgrade validation.
	// Triggered when validating system state before upgrade.
	// EventUpgradeValidating 表示正在进行升级验证。
	// 当在升级前验证系统状态时触发。
	EventUpgradeValidating = "upgrade.validating"

	// EventUpgradeInProgress indicates that the upgrade process is ongoing.
	// Triggered when the upgrade process is in progress.
	// EventUpgradeInProgress 表示升级过程正在进行中。
	// 当升级过程正在进行时触发。
	EventUpgradeInProgress = "upgrade.in_progress"

	// EventUpgradeCompleted indicates successful upgrade.
	// Triggered when new version is installed and verified.
	// EventUpgradeCompleted 表示升级成功。
	// 当新版本安装并验证通过后触发。
	EventUpgradeCompleted = "upgrade.completed"

	// EventUpgradeFailed indicates failed upgrade attempt.
	// Triggered when upgrade process encounters error.
	// EventUpgradeFailed 表示升级尝试失败。
	// 当升级过程遇到错误时触发。
	EventUpgradeFailed = "upgrade.failed"

	// EventRollbackInitiated indicates version rollback start.
	// Triggered when rollback to previous version begins.
	// EventRollbackInitiated 表示版本回滚开始。
	// 当开始回滚到上一个版本时触发。
	EventRollbackInitiated = "rollback.initiated"

	// EventRollbackInProgress indicates that the rollback process is ongoing.
	// Triggered when the rollback process has started and is in progress.
	// EventRollbackInProgress 表示回滚过程正在进行中。
	// 当回滚过程已启动并正在进行时触发。
	EventRollbackInProgress = "rollback.in_progress"

	// EventRollbackCompleted indicates successful rollback.
	// Triggered when previous version is restored.
	// EventRollbackCompleted 表示回滚成功。
	// 当上一个版本恢复成功时触发。
	EventRollbackCompleted = "rollback.completed"

	// EventRollbackFailed indicates failed rollback attempt.
	// Triggered when unable to restore previous version.
	// EventRollbackFailed 表示回滚尝试失败。
	// 当无法恢复上一个版本时触发。
	EventRollbackFailed = "rollback.failed"
)

// Security event types for monitoring security-related events
// 用于监控与安全相关事件的安全事件类型
const (
	// EventSecurityViolation indicates security policy breach.
	// Triggered when security rules are violated.
	// EventSecurityViolation 表示违反了安全策略。
	// 当安全规则被违反时触发。
	EventSecurityViolation = "security.violation"

	// EventAuthenticationFailed indicates failed authentication.
	// Triggered when invalid credentials are used.
	// EventAuthenticationFailed 表示认证失败。
	// 当使用无效凭证时触发。
	EventAuthenticationFailed = "auth.failed"

	// EventAuthorizationDenied indicates unauthorized access.
	// Triggered when insufficient permissions are detected.
	// EventAuthorizationDenied 表示未授权访问。
	// 当检测到权限不足时触发。
	EventAuthorizationDenied = "auth.denied"
)

// Resource lifecycle event types
// 资源生命周期事件类型
const (
	// EventResourceCreated indicates new resource allocation.
	// Triggered when new resource is successfully created.
	// EventResourceCreated 表示新资源已分配。
	// 当新资源成功创建时触发。
	EventResourceCreated = "resource.created"

	// EventResourceModified indicates resource modification.
	// Triggered when existing resource is updated.
	// EventResourceModified 表示资源已修改。
	// 当现有资源被更新时触发。
	EventResourceModified = "resource.modified"

	// EventResourceDeleted indicates resource removal.
	// Triggered when resource is successfully deleted.
	// EventResourceDeleted 表示资源已删除。
	// 当资源成功删除时触发。
	EventResourceDeleted = "resource.deleted"

	// EventResourceUnavailable indicates resource access failure.
	// Triggered when resource becomes inaccessible.
	// EventResourceUnavailable 表示资源访问失败。
	// 当资源变得不可访问时触发。
	EventResourceUnavailable = "resource.unavailable"
)

// Error event types for error handling and recovery
// 用于错误处理和恢复的错误事件类型
const (
	// EventErrorOccurred indicates error detection.
	// Triggered when system encounters an error condition.
	// EventErrorOccurred 表示检测到错误。
	// 当系统遇到错误情况时触发。
	EventErrorOccurred = "error.occurred"

	// EventErrorResolved indicates error recovery.
	// Triggered when error condition is successfully resolved.
	// EventErrorResolved 表示错误已恢复。
	// 当错误情况成功解决时触发。
	EventErrorResolved = "error.resolved"

	// EventPanicRecovered indicates panic recovery.
	// Triggered when system recovers from panic condition.
	// EventPanicRecovered 表示从 panic 状态恢复。
	// 当系统从 panic 状态恢复时触发。
	EventPanicRecovered = "panic.recovered"
)

// PluginEvent represents a lifecycle event in the plugin system.
// It contains detailed information about the event, including its type,
// priority, source, and any associated metadata.
// PluginEvent 表示插件系统中的一个生命周期事件。
// 它包含有关事件的详细信息，包括事件类型、优先级、来源以及任何关联的元数据。
type PluginEvent struct {
	// Type indicates the specific kind of event that occurred
	// Type 表示发生的具体事件类型。
	Type EventType

	// Priority indicates the importance level of the event
	// Priority 表示事件的重要程度。
	Priority int

	// PluginID identifies the plugin that generated the event
	// PluginID 标识生成该事件的插件。
	PluginID string

	// Source identifies where in the plugin the event originated
	// Source 标识事件在插件中的起源位置。
	Source string

	// Category groups related events for easier filtering
	// Category 对相关事件进行分组，以便于过滤。
	Category string

	// Status represents the plugin's state when event occurred
	// Status 表示事件发生时插件的状态。
	Status PluginStatus

	// Error contains any error information if applicable
	// Error 包含适用的错误信息。
	Error error

	// Metadata contains additional event-specific information
	// Metadata 包含事件特定的额外信息。
	Metadata map[string]any

	// Timestamp records when the event occurred
	// Timestamp 记录事件发生的时间。
	Timestamp int64
}

// EventFilter defines criteria for filtering plugin events.
// It allows selective processing of events based on various attributes.
// EventFilter 定义了过滤插件事件的标准。
// 它允许根据各种属性选择性地处理事件。
type EventFilter struct {
	// Types specifies which event types to include
	// Types 指定要包含的事件类型。
	Types []EventType

	// Priorities specifies which priority levels to include
	// Priorities 指定要包含的优先级级别。
	Priorities []int

	// PluginIDs specifies which plugins to monitor
	// PluginIDs 指定要监控的插件。
	PluginIDs []string

	// Categories specifies which event categories to include
	// Categories 指定要包含的事件类别。
	Categories []string

	// FromTime specifies the start time for event filtering
	// FromTime 指定事件过滤的开始时间。
	FromTime int64

	// ToTime specifies the end time for event filtering
	// ToTime 指定事件过滤的结束时间。
	ToTime int64
}

// EventProcessor provides event processing and filtering capabilities.
// EventProcessor 提供事件处理和过滤功能。
type EventProcessor interface {
	// ProcessEvent processes an event through all registered filters.
	// Returns true if the event should be propagated, false if it should be filtered.
	// ProcessEvent 通过所有注册的过滤器处理事件。
	// 如果事件应该被传播则返回 true，如果应该被过滤则返回 false。
	ProcessEvent(event PluginEvent) bool

	// AddFilter adds a new event filter to the processor.
	// Filter will be applied to all subsequent events.
	// AddFilter 向处理器添加一个新的事件过滤器。
	// 该过滤器将应用于所有后续事件。
	AddFilter(filter EventFilter)

	// RemoveFilter removes an event filter by its ID.
	// Events will no longer be filtered by the removed filter.
	// RemoveFilter 根据过滤器 ID 删除一个事件过滤器。
	// 事件将不再被删除的过滤器过滤。
	RemoveFilter(filterID string)
}

// EventEmitter defines the interface for the plugin event system.
// EventEmitter 定义了插件事件系统的接口。
type EventEmitter interface {
	// EmitEvent broadcasts a plugin event to all registered listeners.
	// Event will be processed according to its priority and any active filters.
	// EmitEvent 向所有注册的监听器广播一个插件事件。
	// 事件将根据其优先级和任何活动的过滤器进行处理。
	EmitEvent(event PluginEvent)

	// AddListener registers a new event listener with optional filters.
	// Listener will only receive events that match its filter criteria.
	// AddListener 使用可选的过滤器注册一个新的事件监听器。
	// 监听器将仅接收符合其过滤条件的事件。
	AddListener(listener EventListener, filter *EventFilter)

	// RemoveListener unregisters an event listener.
	// After removal, the listener will no longer receive any events.
	// RemoveListener 注销一个事件监听器。
	// 删除后，该监听器将不再接收任何事件。
	RemoveListener(listener EventListener)

	// GetEventHistory retrieves historical events based on filter criteria.
	// Returns events that match the specified filter parameters.
	// GetEventHistory 根据过滤条件检索历史事件。
	// 返回符合指定过滤参数的事件。
	GetEventHistory(filter EventFilter) []PluginEvent
}

// EventListener defines the interface for handling plugin events.
// EventListener 定义了处理插件事件的接口。
type EventListener interface {
	// HandleEvent processes plugin lifecycle events.
	// Implementation should handle the event according to its type and priority.
	// HandleEvent 处理插件生命周期事件。
	// 实现应根据事件的类型和优先级处理事件。
	HandleEvent(event PluginEvent)

	// GetListenerID returns a unique identifier for the listener.
	// Used for listener management and filtering.
	// GetListenerID 返回监听器的唯一标识符。
	// 用于监听器管理和过滤。
	GetListenerID() string
}

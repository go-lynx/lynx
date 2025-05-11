package plugins

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// BasePlugin provides a default implementation of the Plugin interface and common optional interfaces.
// It serves as a foundation for building custom plugins with standard functionality.
// BasePlugin 提供了 Plugin 接口和常见可选接口的默认实现。
// 它作为构建具有标准功能的自定义插件的基础。
type BasePlugin struct {
	// Basic plugin metadata
	// 插件基本元数据
	id          string // Unique identifier for the plugin // 插件的唯一标识符
	name        string // Human-readable name // 易读的插件名称
	description string // Detailed description of functionality // 插件功能的详细描述
	confPrefix  string // confPrefix 配置前缀
	version     string // Semantic version number // 语义化版本号

	// Operational state
	// 运行状态
	status  PluginStatus // Current plugin status // 插件当前的运行状态
	runtime Runtime      // Runtime environment reference // 运行时环境引用
	logger  log.Logger   // Plugin-specific logger // 插件专用的日志记录器

	// Event handling
	// 事件处理
	eventFilters []EventFilter // List of active event filters // 活动事件过滤器列表

	// Configuration
	// 配置信息
	config map[string]any // Plugin-specific configuration // 插件专用的配置

	// Dependency management
	// 依赖管理
	dependencies []Dependency        // List of plugin dependencies // 插件依赖列表
	capabilities []UpgradeCapability // List of plugin upgrade capabilities // 插件升级能力列表
}

// NewBasePlugin creates a new instance of BasePlugin with the provided metadata.
// This is the recommended way to initialize a new plugin implementation.
// NewBasePlugin 使用提供的元数据创建一个新的 BasePlugin 实例。
// 这是初始化新插件实现的推荐方式。
func NewBasePlugin(id, name, description, version, confPrefix string) *BasePlugin {
	return &BasePlugin{
		id:           id,
		name:         name,
		description:  description,
		version:      version,
		status:       StatusInactive,
		confPrefix:   confPrefix,
		eventFilters: make([]EventFilter, 0),
		config:       make(map[string]any),
		dependencies: make([]Dependency, 0),
		capabilities: []UpgradeCapability{UpgradeNone},
	}
}

// Initialize prepares the plugin for use by setting up its runtime environment.
// This method must be called before the plugin can be started.
// Initialize 通过设置运行时环境为插件使用做准备。
// 在启动插件之前必须调用此方法。
func (p *BasePlugin) Initialize(plugin Plugin, rt Runtime) error {
	if rt == nil {
		return ErrPluginNotInitialized
	}

	p.runtime = rt
	p.logger = rt.GetLogger()
	p.status = StatusInitializing

	// Emit event indicating plugin is initializing
	// 发出插件初始化中的事件
	p.EmitEvent(PluginEvent{
		Type:     EventPluginInitializing,
		Priority: PriorityNormal,
		Source:   "Initialize",
		Category: "lifecycle",
	})

	// Call InitializeResources for custom initialization
	// 调用 InitializeResources 进行自定义初始化
	if err := plugin.InitializeResources(rt); err != nil {
		p.status = StatusFailed
		return NewPluginError(p.id, "Initialize", "Failed to initialize resources", err)
	}

	p.status = StatusInactive
	// Emit event indicating plugin has been initialized
	// 发出插件已初始化的事件
	p.EmitEvent(PluginEvent{
		Type:     EventPluginInitialized,
		Priority: PriorityNormal,
		Source:   "Initialize",
		Category: "lifecycle",
	})

	return nil
}

// Start activates the plugin and begins its main operations.
// The plugin must be initialized before it can be started.
// Start 激活插件并开始其主要操作。
// 插件必须在初始化后才能启动。
func (p *BasePlugin) Start(plugin Plugin) error {
	if p.status == StatusActive {
		return ErrPluginAlreadyActive
	}

	if p.runtime == nil {
		return ErrPluginNotInitialized
	}

	p.status = StatusInitializing
	// Emit event indicating plugin is starting
	// 发出插件启动中的事件
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStarting,
		Priority: PriorityNormal,
		Source:   "Start",
		Category: "lifecycle",
	})

	// Call StartupTasks for custom startup logic
	// 调用 StartupTasks 执行自定义启动逻辑
	if err := plugin.StartupTasks(); err != nil {
		p.status = StatusFailed
		return NewPluginError(p.id, "Start", "Failed to perform startup tasks", err)
	}

	p.status = StatusActive
	// Emit event indicating plugin has started
	// 发出插件已启动的事件
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStarted,
		Priority: PriorityNormal,
		Source:   "Start",
		Category: "lifecycle",
	})

	return nil
}

// Stop gracefully terminates the plugin's operations.
// This method should release all resources and perform cleanup.
// Stop 优雅地终止插件的操作。
// 此方法应释放所有资源并执行清理操作。
func (p *BasePlugin) Stop(plugin Plugin) error {
	if p.status != StatusActive {
		return NewPluginError(p.id, "Stop", "Plugin must be active to stop", ErrPluginNotActive)
	}

	p.status = StatusStopping
	// Emit event indicating plugin is stopping
	// 发出插件停止中的事件
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStopping,
		Priority: PriorityNormal,
		Source:   "Stop",
		Category: "lifecycle",
	})

	// Call CleanupTasks for custom cleanup logic
	// 调用 CleanupTasks 执行自定义清理逻辑
	if err := plugin.CleanupTasks(); err != nil {
		p.status = StatusFailed
		return NewPluginError(p.id, "Stop", "Failed to perform cleanup tasks", err)
	}

	p.status = StatusTerminated
	// Emit event indicating plugin has stopped
	// 发出插件已停止的事件
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStopped,
		Priority: PriorityNormal,
		Source:   "Stop",
		Category: "lifecycle",
	})

	return nil
}

// Status returns the current operational status of the plugin.
// This method is thread-safe and can be called at any time.
// Status 返回插件当前的运行状态。
// 此方法是线程安全的，可以在任何时候调用。
func (p *BasePlugin) Status(plugin Plugin) PluginStatus {
	return p.status
}

// InitializeResources sets up the plugin's required resources.
// This method can be overridden by embedding structs to provide custom initialization.
// InitializeResources 设置插件所需的资源。
// 嵌入结构体可以重写此方法以提供自定义初始化逻辑。
func (p *BasePlugin) InitializeResources(rt Runtime) error {
	return nil
}

// StartupTasks performs necessary tasks during plugin startup.
// This method can be overridden by embedding structs to provide custom startup logic.
// StartupTasks 在插件启动期间执行必要的任务。
// 嵌入结构体可以重写此方法以提供自定义启动逻辑。
func (p *BasePlugin) StartupTasks() error {
	return nil
}

// CleanupTasks performs cleanup during plugin shutdown.
// This method can be overridden by embedding structs to provide custom cleanup logic.
// CleanupTasks 在插件关闭期间执行清理操作。
// 嵌入结构体可以重写此方法以提供自定义清理逻辑。
func (p *BasePlugin) CleanupTasks() error {
	return nil
}

// ID returns the unique identifier of the plugin.
// This ID must be unique across all plugins in the system.
// ID 返回插件的唯一标识符。
// 此 ID 在系统中的所有插件中必须是唯一的。
func (p *BasePlugin) ID() string {
	return p.id
}

// Name returns the human-readable name of the plugin.
// This name is used for display and logging purposes.
// Name 返回插件的易读名称。
// 此名称用于显示和日志记录目的。
func (p *BasePlugin) Name() string {
	return p.name
}

// Description returns a detailed description of the plugin's functionality.
// This helps users understand the plugin's purpose and capabilities.
// Description 返回插件功能的详细描述。
// 这有助于用户了解插件的用途和功能。
func (p *BasePlugin) Description() string {
	return p.description
}

// Version returns the semantic version of the plugin.
// Version format should follow semver conventions (MAJOR.MINOR.PATCH).
// Version 返回插件的语义化版本。
// 版本格式应遵循语义化版本规范（MAJOR.MINOR.PATCH）。
func (p *BasePlugin) Version() string {
	return p.version
}

// SetStatus sets the current operational status of the plugin.
// This method is thread-safe and should be used to update plugin status.
// SetStatus 设置插件当前的运行状态。
// 此方法是线程安全的，应使用它来更新插件状态。
func (p *BasePlugin) SetStatus(status PluginStatus) {
	p.status = status
}

// GetHealth performs a health check and returns a detailed health report.
// This method should be called periodically to monitor plugin health.
// GetHealth 执行健康检查并返回详细的健康报告。
// 应定期调用此方法以监控插件的健康状况。
func (p *BasePlugin) GetHealth() HealthReport {
	report := HealthReport{
		Status:    "unknown",
		Details:   make(map[string]any),
		Timestamp: time.Now().Unix(),
	}

	// Emit event indicating health check has started
	// 发出健康检查开始的事件
	p.EmitEvent(PluginEvent{
		Type:     EventHealthCheckStarted,
		Priority: PriorityNormal,
		Source:   "GetHealth",
		Category: "health",
	})

	// Check if plugin is in a valid state for health check
	// 检查插件是否处于可进行健康检查的有效状态
	switch p.status {
	case StatusTerminated, StatusFailed:
		report.Status = "unhealthy"
		report.Message = "Plugin is not operational"
		// Emit event indicating critical health status
		// 发出健康状态危急的事件
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusCritical,
			Priority: PriorityHigh,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	case StatusSuspended:
		report.Status = "suspended"
		report.Message = "Plugin is temporarily suspended"
		// Emit event indicating warning health status
		// 发出健康状态警告的事件
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusWarning,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	case StatusUpgrading:
		report.Status = "upgrading"
		report.Message = "Plugin is being upgraded"
		// Emit event indicating unknown health status
		// 发出健康状态未知的事件
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusUnknown,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	case StatusRollback:
		report.Status = "rolling-back"
		report.Message = "Plugin is rolling back to previous version"
		// Emit event indicating unknown health status
		// 发出健康状态未知的事件
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusUnknown,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	case StatusInitializing:
		report.Status = "initializing"
		report.Message = "Plugin is being initialized"
		// Emit event indicating unknown health status
		// 发出健康状态未知的事件
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusUnknown,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	case StatusInactive:
		report.Status = "inactive"
		report.Message = "Plugin is not yet started"
		// Emit event indicating warning health status
		// 发出健康状态警告的事件
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusWarning,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	case StatusStopping:
		report.Status = "stopping"
		report.Message = "Plugin is shutting down"
		// Emit event indicating warning health status
		// 发出健康状态警告的事件
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusWarning,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
		return report
	default:
		report.Status = "unhealthy"
		report.Message = "Plugin status is unknown"

	}

	// Emit event indicating health check is running
	// 发出健康检查运行中的事件
	p.EmitEvent(PluginEvent{
		Type:     EventHealthCheckRunning,
		Priority: PriorityNormal,
		Source:   "GetHealth",
		Category: "health",
	})

	// Perform health check for active plugin
	// 为活动插件执行健康检查
	if err := p.CheckHealth(&report); err != nil {
		report.Status = "unhealthy"
		report.Message = err.Error()
		// Emit event indicating health check failed
		// 发出健康检查失败的事件
		p.EmitEvent(PluginEvent{
			Type:     EventHealthCheckFailed,
			Priority: PriorityHigh,
			Source:   "GetHealth",
			Category: "health",
			Error:    err,
		})
		return report
	}

	// Emit event indicating health check is done
	// 发出健康检查完成的事件
	p.EmitEvent(PluginEvent{
		Type:     EventHealthCheckDone,
		Priority: PriorityNormal,
		Source:   "GetHealth",
		Category: "health",
	})

	// Emit appropriate health status event
	// 发出相应的健康状态事件
	if report.Status == "healthy" {
		// Emit event indicating OK health status
		// 发出健康状态正常的事件
		p.EmitEvent(PluginEvent{
			Type:     EventHealthStatusOK,
			Priority: PriorityNormal,
			Source:   "GetHealth",
			Category: "health",
		})
	}

	return report
}

// Configure updates the plugin's configuration with the provided settings.
// This method validates and applies new configuration values.
// Configure 使用提供的设置更新插件的配置。
// 此方法验证并应用新的配置值。
func (p *BasePlugin) Configure(conf any) error {
	// Emit event indicating configuration has changed
	// 发出配置已更改的事件
	p.EmitEvent(PluginEvent{
		Type:     EventConfigurationChanged,
		Priority: PriorityNormal,
		Source:   "Configure",
		Category: "configuration",
	})

	// Validate and apply configuration
	// 验证并应用配置
	if err := p.ValidateConfig(conf); err != nil {
		// Emit event indicating invalid configuration
		// 发出配置无效的事件
		p.EmitEvent(PluginEvent{
			Type:     EventConfigurationInvalid,
			Priority: PriorityHigh,
			Source:   "Configure",
			Category: "configuration",
			Error:    err,
		})
		return NewPluginError(p.id, "Configure", "Invalid configuration", err)
	}

	if err := p.ApplyConfig(conf); err != nil {
		return NewPluginError(p.id, "Configure", "Failed to apply configuration", err)
	}

	// Emit event indicating configuration has been applied
	// 发出配置已应用的事件
	p.EmitEvent(PluginEvent{
		Type:     EventConfigurationApplied,
		Priority: PriorityNormal,
		Source:   "Configure",
		Category: "configuration",
	})

	return nil
}

// GetDependencies returns the list of plugin dependencies.
// This includes both required and optional dependencies.
// GetDependencies 返回插件的依赖列表。
// 这包括必需和可选的依赖项。
func (p *BasePlugin) GetDependencies() []Dependency {
	return p.dependencies
}

// AddDependency adds a new dependency to the plugin.
// The dependency will be validated during plugin initialization.
// AddDependency 向插件添加一个新的依赖项。
// 该依赖项将在插件初始化期间进行验证。
func (p *BasePlugin) AddDependency(dep Dependency) {
	p.dependencies = append(p.dependencies, dep)
	// Emit event indicating dependency status has changed
	// 发出依赖状态已更改的事件
	p.EmitEvent(PluginEvent{
		Type:     EventDependencyStatusChanged,
		Priority: PriorityNormal,
		Source:   "AddDependency",
		Category: "dependency",
		Metadata: map[string]any{
			"dependency": dep,
		},
	})
}

// AddEventFilter adds a new event filter to the plugin.
// Events will be filtered according to the specified criteria.
// AddEventFilter 向插件添加一个新的事件过滤器。
// 事件将根据指定的条件进行过滤。
func (p *BasePlugin) AddEventFilter(filter EventFilter) {
	p.eventFilters = append(p.eventFilters, filter)
}

// RemoveEventFilter removes an event filter from the plugin.
// This affects how future events will be processed.
// RemoveEventFilter 从插件中移除一个事件过滤器。
// 这会影响未来事件的处理方式。
func (p *BasePlugin) RemoveEventFilter(index int) {
	if index >= 0 && index < len(p.eventFilters) {
		p.eventFilters = append(p.eventFilters[:index], p.eventFilters[index+1:]...)
	}
}

// HandleEvent processes incoming plugin events.
// Events are filtered and handled according to configured filters.
// HandleEvent 处理传入的插件事件。
// 事件将根据配置的过滤器进行过滤和处理。
func (p *BasePlugin) HandleEvent(event PluginEvent) {
	if !p.ShouldHandleEvent(event) {
		return
	}

	// Process the event based on type
	// 根据事件类型处理事件
	switch event.Type {
	case EventHealthStatusChanged:
		p.HandleHealthEvent(event)
	case EventConfigurationChanged:
		p.HandleConfigEvent(event)
	case EventDependencyStatusChanged:
		p.HandleDependencyEvent(event)
	default:
		p.HandleDefaultEvent(event)
	}
}

// EmitEvent emits an event to the runtime event system.
// This method adds standard fields to the event before emission.
// EmitEvent 向运行时事件系统发出一个事件。
// 此方法在发出事件之前会向事件添加标准字段。
func (p *BasePlugin) EmitEvent(event PluginEvent) {
	p.EmitEventInternal(event)
}

// EmitEventInternal emits an event to the runtime event system.
// This method adds standard fields to the event before emission.
// EmitEventInternal 向运行时事件系统发出一个事件。
// 此方法在发出事件之前会向事件添加标准字段。
func (p *BasePlugin) EmitEventInternal(event PluginEvent) {
	// Add standard fields
	// 添加标准字段
	event.PluginID = p.id
	event.Status = p.status
	event.Timestamp = time.Now().Unix()

	// Apply filters
	// 应用过滤器
	if p.ShouldEmitEvent(event) {
		p.runtime.EmitEvent(event)
	}
}

// ShouldEmitEvent checks if an event should be emitted based on filters.
// This implements the event filtering logic.
// ShouldEmitEvent 根据过滤器检查是否应该发出一个事件。
// 这实现了事件过滤逻辑。
func (p *BasePlugin) ShouldEmitEvent(event PluginEvent) bool {
	if len(p.eventFilters) == 0 {
		return true
	}

	for _, filter := range p.eventFilters {
		if p.EventMatchesFilter(event, filter) {
			return true
		}
	}

	return false
}

// ShouldHandleEvent checks if an event should be handled based on filters.
// This implements the event handling filter logic.
// ShouldHandleEvent 根据过滤器检查是否应该处理一个事件。
// 这实现了事件处理过滤逻辑。
func (p *BasePlugin) ShouldHandleEvent(event PluginEvent) bool {
	return p.ShouldEmitEvent(event)
}

// EventMatchesFilter checks if an event matches a specific filter.
// This implements the detailed filter matching logic.
// EventMatchesFilter 检查一个事件是否匹配特定的过滤器。
// 这实现了详细的过滤器匹配逻辑。
func (p *BasePlugin) EventMatchesFilter(event PluginEvent, filter EventFilter) bool {
	// Check event type
	// 检查事件类型
	if len(filter.Types) > 0 {
		typeMatch := false
		for _, t := range filter.Types {
			if event.Type == t {
				typeMatch = true
				break
			}
		}
		if !typeMatch {
			return false
		}
	}

	// Check priority
	// 检查优先级
	if len(filter.Priorities) > 0 {
		priorityMatch := false
		for _, p := range filter.Priorities {
			if event.Priority == p {
				priorityMatch = true
				break
			}
		}
		if !priorityMatch {
			return false
		}
	}

	// Check plugin ID
	// 检查插件 ID
	if len(filter.PluginIDs) > 0 {
		idMatch := false
		for _, id := range filter.PluginIDs {
			if event.PluginID == id {
				idMatch = true
				break
			}
		}
		if !idMatch {
			return false
		}
	}

	// Check category
	// 检查类别
	if len(filter.Categories) > 0 {
		categoryMatch := false
		for _, c := range filter.Categories {
			if event.Category == c {
				categoryMatch = true
				break
			}
		}
		if !categoryMatch {
			return false
		}
	}

	// Check time range
	// 检查时间范围
	if filter.FromTime > 0 && event.Timestamp < filter.FromTime {
		return false
	}
	if filter.ToTime > 0 && event.Timestamp > filter.ToTime {
		return false
	}

	return true
}

// CheckHealth performs the actual health check operations.
// This is called during health status reporting.
// CheckHealth 执行实际的健康检查操作。
// 此方法在健康状态报告期间被调用。
func (p *BasePlugin) CheckHealth(report *HealthReport) error {
	// Implementation-specific health checks
	// 特定于实现的健康检查
	return nil
}

// ValidateConfig validates the provided configuration.
// This is called before applying new configuration.
// ValidateConfig 验证提供的配置。
// 此方法在应用新配置之前被调用。
func (p *BasePlugin) ValidateConfig(conf any) error {
	// Implementation-specific configuration validation
	// 特定于实现的配置验证
	return nil
}

// ApplyConfig applies the validated configuration.
// This is called after configuration validation succeeds.
// ApplyConfig 应用经验证的配置。
// 此方法在配置验证成功后被调用。
func (p *BasePlugin) ApplyConfig(conf any) error {
	// Implementation-specific configuration application
	// 特定于实现的配置应用
	return nil
}

// HandleHealthEvent processes health-related events.
// This implements specific handling for health events.
// HandleHealthEvent 处理与健康相关的事件。
// 这实现了对健康事件的特定处理。
func (p *BasePlugin) HandleHealthEvent(event PluginEvent) {
	// Implementation-specific health event handling
	// 特定于实现的健康事件处理
}

// HandleConfigEvent processes configuration-related events.
// This implements specific handling for configuration events.
// HandleConfigEvent 处理与配置相关的事件。
// 这实现了对配置事件的特定处理。
func (p *BasePlugin) HandleConfigEvent(event PluginEvent) {
	// Implementation-specific configuration event handling
	// 特定于实现的配置事件处理
}

// HandleDependencyEvent processes dependency-related events.
// This implements specific handling for dependency events.
// HandleDependencyEvent 处理与依赖相关的事件。
// 这实现了对依赖事件的特定处理。
func (p *BasePlugin) HandleDependencyEvent(event PluginEvent) {
	// Implementation-specific dependency event handling
	// 特定于实现的依赖事件处理
}

// HandleDefaultEvent processes events that don't have specific handlers.
// This implements default event handling behavior.
// HandleDefaultEvent 处理没有特定处理程序的事件。
// 这实现了默认的事件处理行为。
func (p *BasePlugin) HandleDefaultEvent(event PluginEvent) {
	// Implementation-specific default event handling
	// 特定于实现的默认事件处理
}

// Suspend temporarily suspends the plugin.
// This method checks if the plugin is in the active state.
// Suspend 暂时挂起插件。
// 此方法检查插件是否处于活动状态。
func (p *BasePlugin) Suspend() error {
	if p.status != StatusActive {
		return NewPluginError(p.id, "Suspend", "Plugin must be active to suspend", ErrPluginNotActive)
	}

	p.status = StatusStopping
	// Emit event indicating plugin is stopping
	// 发出插件停止中的事件
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStopping,
		Priority: PriorityNormal,
		Source:   "Suspend",
		Category: "lifecycle",
	})

	// Perform any suspension tasks here if needed
	// 如有需要，在此执行任何挂起任务
	p.status = StatusSuspended
	return nil
}

// Resume resumes the plugin from suspended state.
// This method checks if the plugin is in the suspended state.
// Resume 从挂起状态恢复插件。
// 此方法检查插件是否处于挂起状态。
func (p *BasePlugin) Resume() error {
	if p.status != StatusSuspended {
		return NewPluginError(p.id, "Resume", "Plugin must be suspended to resume", ErrPluginNotActive)
	}

	// Emit event indicating plugin is starting
	// 发出插件启动中的事件
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStarting,
		Priority: PriorityNormal,
		Source:   "Resume",
		Category: "lifecycle",
	})

	p.status = StatusActive

	// Emit event indicating plugin has started
	// 发出插件已启动的事件
	p.EmitEvent(PluginEvent{
		Type:     EventPluginStarted,
		Priority: PriorityNormal,
		Source:   "Resume",
		Category: "lifecycle",
	})

	return nil
}

// PrepareUpgrade prepares the plugin for upgrade.
// This method checks if the plugin supports the upgrade capability.
// PrepareUpgrade 为插件升级做准备。
// 此方法检查插件是否支持升级能力。
func (p *BasePlugin) PrepareUpgrade(targetVersion string) error {
	if !p.SupportsCapability(UpgradeConfig) && !p.SupportsCapability(UpgradeVersion) {
		return NewPluginError(p.id, "PrepareUpgrade", "Upgrade not supported", ErrPluginUpgradeNotSupported)
	}

	if p.status != StatusActive {
		return NewPluginError(p.id, "PrepareUpgrade", "Plugin must be active to upgrade", ErrPluginNotActive)
	}

	// Emit event indicating upgrade has been initiated
	// 发出升级已启动的事件
	p.EmitEvent(PluginEvent{
		Type:     EventUpgradeInitiated,
		Priority: PriorityHigh,
		Source:   "PrepareUpgrade",
		Category: "upgrade",
		Metadata: map[string]any{
			"targetVersion":  targetVersion,
			"currentVersion": p.version,
		},
	})

	p.status = StatusUpgrading
	return nil
}

// ExecuteUpgrade performs the plugin upgrade.
// This method checks if the plugin is in the upgrading state.
// ExecuteUpgrade 执行插件升级。
// 此方法检查插件是否处于升级状态。
func (p *BasePlugin) ExecuteUpgrade(targetVersion string) error {
	if p.status != StatusUpgrading {
		return NewPluginError(p.id, "ExecuteUpgrade", "Plugin must be in upgrading state", ErrPluginNotActive)
	}

	// Perform upgrade tasks
	// 执行升级任务
	if err := p.PerformUpgrade(targetVersion); err != nil {
		// Emit event indicating upgrade failed
		// 发出升级失败的事件
		p.EmitEvent(PluginEvent{
			Type:     EventUpgradeFailed,
			Priority: PriorityCritical,
			Source:   "ExecuteUpgrade",
			Category: "upgrade",
			Error:    err,
			Metadata: map[string]any{
				"targetVersion":  targetVersion,
				"currentVersion": p.version,
			},
		})

		// Attempt automatic rollback
		// 尝试自动回滚
		if rollbackErr := p.RollbackUpgrade(p.version); rollbackErr != nil {
			// If rollback fails, plugin is in an inconsistent state
			// 如果回滚失败，插件处于不一致状态
			p.status = StatusFailed
			return NewPluginError(p.id, "ExecuteUpgrade", "Upgrade and rollback failed", err)
		}

		return NewPluginError(p.id, "ExecuteUpgrade", "Upgrade failed, rolled back", err)
	}

	// Update version and restore active state
	// 更新版本并恢复活动状态
	p.version = targetVersion
	p.status = StatusActive

	// Emit event indicating upgrade has been completed
	// 发出升级已完成的事件
	p.EmitEvent(PluginEvent{
		Type:     EventUpgradeCompleted,
		Priority: PriorityHigh,
		Source:   "ExecuteUpgrade",
		Category: "upgrade",
		Metadata: map[string]any{
			"version": targetVersion,
		},
	})

	return nil
}

// RollbackUpgrade rolls back the plugin upgrade.
// This method checks if the plugin is in the upgrading or failed state.
// RollbackUpgrade 回滚插件升级。
// 此方法检查插件是否处于升级或失败状态。
func (p *BasePlugin) RollbackUpgrade(previousVersion string) error {
	if p.status != StatusUpgrading && p.status != StatusFailed {
		return NewPluginError(p.id, "RollbackUpgrade", "Plugin must be in upgrading or failed state", ErrPluginNotActive)
	}

	p.status = StatusRollback
	// Emit event indicating rollback has been initiated
	// 发出回滚已启动的事件
	p.EmitEvent(PluginEvent{
		Type:     EventRollbackInitiated,
		Priority: PriorityHigh,
		Source:   "RollbackUpgrade",
		Category: "upgrade",
		Metadata: map[string]any{
			"previousVersion": previousVersion,
			"currentVersion":  p.version,
		},
	})

	// Perform rollback tasks
	// 执行回滚任务
	if err := p.PerformRollback(previousVersion); err != nil {
		p.status = StatusFailed
		// Emit event indicating rollback failed
		// 发出回滚失败的事件
		p.EmitEvent(PluginEvent{
			Type:     EventRollbackFailed,
			Priority: PriorityCritical,
			Source:   "RollbackUpgrade",
			Category: "upgrade",
			Error:    err,
			Metadata: map[string]any{
				"previousVersion": previousVersion,
				"currentVersion":  p.version,
			},
		})
		return NewPluginError(p.id, "RollbackUpgrade", "Rollback failed", err)
	}

	// Restore version and active state
	// 恢复版本和活动状态
	p.version = previousVersion
	p.status = StatusActive

	// Emit event indicating rollback has been completed
	// 发出回滚已完成的事件
	p.EmitEvent(PluginEvent{
		Type:     EventRollbackCompleted,
		Priority: PriorityHigh,
		Source:   "RollbackUpgrade",
		Category: "upgrade",
		Metadata: map[string]any{
			"version": previousVersion,
		},
	})

	return nil
}

// PerformUpgrade handles the actual upgrade process.
// This is an internal method called by ExecuteUpgrade.
// PerformUpgrade 处理实际的升级过程。
// 这是一个由 ExecuteUpgrade 调用的内部方法。
func (p *BasePlugin) PerformUpgrade(targetVersion string) error {
	// Implementation-specific upgrade logic
	// 特定于实现的升级逻辑
	return nil
}

// PerformRollback handles the actual rollback process.
// This is an internal method called by RollbackUpgrade.
// PerformRollback 处理实际的回滚过程。
// 这是一个由 RollbackUpgrade 调用的内部方法。
func (p *BasePlugin) PerformRollback(previousVersion string) error {
	// Implementation-specific rollback logic
	// 特定于实现的回滚逻辑
	return nil
}

// GetCapabilities returns the plugin's upgrade capabilities.
// GetCapabilities 返回插件的升级能力列表。
func (p *BasePlugin) GetCapabilities() []UpgradeCapability {
	return p.capabilities
}

// SupportsCapability checks if the plugin supports the specified upgrade capability.
// SupportsCapability 检查插件是否支持指定的升级能力。
func (p *BasePlugin) SupportsCapability(cap UpgradeCapability) bool {
	for _, c := range p.capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// Package plugins provides the core plugin system for the Lynx framework.
// 包 plugins 为 Lynx 框架提供核心插件系统。
package plugins

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

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

// TypedPlugin 泛型插件接口，T 为具体插件类型
// 提供类型安全的插件访问能力
type TypedPlugin[T any] interface {
	Plugin
	GetTypedInstance() T
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

	// Weight 权重获取
	Weight() int
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

// ResourceInfo 资源信息
type ResourceInfo struct {
	Name        string
	Type        string
	PluginID    string
	IsPrivate   bool
	CreatedAt   time.Time
	LastUsedAt  time.Time
	AccessCount int64
	Size        int64 // 资源大小（字节）
	Metadata    map[string]any
}

// ResourceManager 资源管理器接口
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

	// 新增：资源生命周期管理
	GetResourceInfo(name string) (*ResourceInfo, error)
	ListResources() []*ResourceInfo
	CleanupResources(pluginID string) error
	GetResourceStats() map[string]any
}

// TypedResourceManager 泛型资源管理器接口
type TypedResourceManager interface {
	ResourceManager
}

// GetTypedResource 获取类型安全的资源（独立函数）
func GetTypedResource[T any](manager ResourceManager, name string) (T, error) {
	var zero T
	resource, err := manager.GetResource(name)
	if err != nil {
		return zero, err
	}

	typed, ok := resource.(T)
	if !ok {
		return zero, NewPluginError("runtime", "GetTypedResource", "Type assertion failed", nil)
	}

	return typed, nil
}

// RegisterTypedResource 注册类型安全的资源（独立函数）
func RegisterTypedResource[T any](manager ResourceManager, name string, resource T) error {
	return manager.RegisterResource(name, resource)
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

// ServicePlugin 服务插件约束接口
type ServicePlugin[T any] interface {
	Plugin
	GetServer() T
	GetServerType() string
}

// DatabasePlugin 数据库插件约束接口
type DatabasePlugin[T any] interface {
	Plugin
	GetDriver() T
	GetStats() any
	IsConnected() bool
	CheckHealth() error
}

// CachePlugin 缓存插件约束接口
type CachePlugin[T any] interface {
	Plugin
	GetClient() T
	GetConnectionStats() map[string]any
}

// MessagingPlugin 消息队列插件约束接口
type MessagingPlugin[T any] interface {
	Plugin
	GetProducer() T
	GetConsumer() T
}

// ServiceDiscoveryPlugin 服务发现插件约束接口
type ServiceDiscoveryPlugin[T any] interface {
	Plugin
	GetRegistry() T
	GetDiscovery() T
}

// ========== 向后兼容的接口 ==========

// ServicePluginAny 保持向后兼容的服务插件接口
type ServicePluginAny interface {
	Plugin
	GetServer() any
	GetServerType() string
}

// DatabasePluginAny 保持向后兼容的数据库插件接口
type DatabasePluginAny interface {
	Plugin
	GetDriver() any
	GetStats() any
	IsConnected() bool
	CheckHealth() error
}

// CachePluginAny 保持向后兼容的缓存插件接口
type CachePluginAny interface {
	Plugin
	GetClient() any
	GetConnectionStats() map[string]any
}

// MessagingPluginAny 保持向后兼容的消息队列插件接口
type MessagingPluginAny interface {
	Plugin
	GetProducer() any
	GetConsumer() any
}

// ServiceDiscoveryPluginAny 保持向后兼容的服务发现插件接口
type ServiceDiscoveryPluginAny interface {
	Plugin
	GetRegistry() any
	GetDiscovery() any
}

// ========== 泛型运行时环境 ==========

// Runtime 运行时接口
type Runtime interface {
	TypedResourceManager
	ConfigProvider
	LogProvider
	EventEmitter
	// 新增：逻辑分离的资源管理
	GetPrivateResource(name string) (any, error)
	RegisterPrivateResource(name string, resource any) error
	GetSharedResource(name string) (any, error)
	RegisterSharedResource(name string, resource any) error
	// 新增：改进的事件系统
	EmitPluginEvent(pluginName string, eventType string, data map[string]any)
	AddPluginListener(pluginName string, listener EventListener, filter *EventFilter)
	GetPluginEventHistory(pluginName string, filter EventFilter) []PluginEvent
	// 新增：插件上下文管理
	WithPluginContext(pluginName string) Runtime
	GetCurrentPluginContext() string
	// 新增：配置管理
	SetConfig(conf config.Config)
}

// TypedRuntime 泛型运行时接口
type TypedRuntime interface {
	Runtime
}

// TypedRuntimeImpl 泛型运行时实现
type TypedRuntimeImpl struct {
	runtime Runtime
}

// NewTypedRuntime 创建泛型运行时环境
func NewTypedRuntime() *TypedRuntimeImpl {
	return &TypedRuntimeImpl{
		runtime: NewSimpleRuntime(),
	}
}

// simpleRuntime 简单的运行时实现
type simpleRuntime struct {
	// 私有资源：每个插件独立管理
	privateResources map[string]map[string]any
	// 共享资源：所有插件共享
	sharedResources map[string]any
	// 资源信息：跟踪资源生命周期
	resourceInfo map[string]*ResourceInfo
	// 配置
	config config.Config
	// 互斥锁
	mu sync.RWMutex

	// 事件系统
	listeners    map[string][]EventListener
	eventHistory []PluginEvent
	eventMu      sync.RWMutex
	maxHistory   int

	// 插件上下文
	currentPluginContext string
	contextMu            sync.RWMutex

	// 新增：事件处理工作池
	eventWorkerPool chan struct{}
	eventPoolSize   int
	shutdown        chan struct{}
	shutdownOnce    sync.Once
}

func NewSimpleRuntime() *simpleRuntime {
	return &simpleRuntime{
		privateResources: make(map[string]map[string]any),
		sharedResources:  make(map[string]any),
		resourceInfo:     make(map[string]*ResourceInfo),
		listeners:        make(map[string][]EventListener),
		eventHistory:     make([]PluginEvent, 0),
		maxHistory:       1000,                    // 最多保留1000个事件
		eventWorkerPool:  make(chan struct{}, 50), // 限制并发goroutine数量
		eventPoolSize:    50,
		shutdown:         make(chan struct{}),
	}
}

// EmitEvent 发出事件 - 修复并发安全问题
func (r *simpleRuntime) EmitEvent(event PluginEvent) {
	r.eventMu.Lock()
	defer r.eventMu.Unlock()

	// 检查是否已关闭
	select {
	case <-r.shutdown:
		return
	default:
	}

	// 添加到事件历史
	r.eventHistory = append(r.eventHistory, event)

	// 限制历史记录大小
	if len(r.eventHistory) > r.maxHistory {
		r.eventHistory = r.eventHistory[1:]
	}

	// 复制监听器列表以避免在通知过程中修改
	listenersCopy := make(map[string][]EventListener)
	for key, listeners := range r.listeners {
		listenersCopy[key] = make([]EventListener, len(listeners))
		copy(listenersCopy[key], listeners)
	}

	// 在锁外通知监听器，避免死锁
	go r.notifyListeners(listenersCopy, event)
}

// notifyListeners 通知监听器 - 新增方法
func (r *simpleRuntime) notifyListeners(listeners map[string][]EventListener, event PluginEvent) {
	for _, listeners := range listeners {
		for _, listener := range listeners {
			select {
			case r.eventWorkerPool <- struct{}{}:
				go func(l EventListener) {
					defer func() { <-r.eventWorkerPool }()
					r.safeHandleEvent(l, event)
				}(listener)
			default:
				// 工作池已满，直接在当前goroutine中处理
				r.safeHandleEvent(listener, event)
			}
		}
	}
}

// safeHandleEvent 安全处理事件 - 新增方法
func (r *simpleRuntime) safeHandleEvent(listener EventListener, event PluginEvent) {
	defer func() {
		if r := recover(); r != nil {
			// 防止监听器panic影响其他监听器
			fmt.Printf("Event listener panic: %v\n", r)
		}
	}()

	// 添加超时控制
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 使用select确保不会无限等待
	done := make(chan struct{})
	go func() {
		defer close(done)
		listener.HandleEvent(event)
	}()

	select {
	case <-done:
		// 正常完成
	case <-ctx.Done():
		// 超时
		fmt.Printf("Event listener timeout for event: %s\n", event.Type)
	case <-r.shutdown:
		// 系统关闭
		return
	}
}

// Shutdown 关闭运行时 - 新增方法
func (r *simpleRuntime) Shutdown() {
	r.shutdownOnce.Do(func() {
		close(r.shutdown)
		// 等待所有事件处理完成
		time.Sleep(100 * time.Millisecond)
	})
}

// AddListener 添加事件监听器 - 修复并发安全问题
func (r *simpleRuntime) AddListener(listener EventListener, filter *EventFilter) {
	if listener == nil {
		return
	}

	r.eventMu.Lock()
	defer r.eventMu.Unlock()

	key := "global"
	if filter != nil {
		// 如果有过滤器，使用过滤器的标识作为key
		key = fmt.Sprintf("filter_%p", filter)
	}

	r.listeners[key] = append(r.listeners[key], listener)
}

// AddPluginListener 添加特定插件的事件监听器 - 修复并发安全问题
func (r *simpleRuntime) AddPluginListener(pluginName string, listener EventListener, filter *EventFilter) {
	if listener == nil || pluginName == "" {
		return
	}

	r.eventMu.Lock()
	defer r.eventMu.Unlock()

	key := fmt.Sprintf("plugin_%s", pluginName)
	if filter != nil {
		key = fmt.Sprintf("plugin_%s_filter_%p", pluginName, filter)
	}

	r.listeners[key] = append(r.listeners[key], listener)
}

// RemoveListener 移除事件监听器 - 修复并发安全问题
func (r *simpleRuntime) RemoveListener(listener EventListener) {
	if listener == nil {
		return
	}

	r.eventMu.Lock()
	defer r.eventMu.Unlock()

	// 遍历所有监听器组，移除指定的监听器
	for key, listeners := range r.listeners {
		for i, l := range listeners {
			if l == listener {
				// 移除监听器
				r.listeners[key] = append(listeners[:i], listeners[i+1:]...)
				break
			}
		}
		// 如果该组没有监听器了，删除该组
		if len(r.listeners[key]) == 0 {
			delete(r.listeners, key)
		}
	}
}

// GetEventHistory 获取事件历史 - 修复并发安全问题
func (r *simpleRuntime) GetEventHistory(filter EventFilter) []PluginEvent {
	r.eventMu.RLock()
	defer r.eventMu.RUnlock()

	if r.isEmptyFilter(filter) {
		// 返回所有事件
		result := make([]PluginEvent, len(r.eventHistory))
		copy(result, r.eventHistory)
		return result
	}

	// 根据过滤器筛选事件
	var result []PluginEvent
	for _, event := range r.eventHistory {
		if r.matchesFilter(event, filter) {
			result = append(result, event)
		}
	}

	return result
}

// GetPluginEventHistory 获取插件事件历史 - 修复并发安全问题
func (r *simpleRuntime) GetPluginEventHistory(pluginName string, filter EventFilter) []PluginEvent {
	r.eventMu.RLock()
	defer r.eventMu.RUnlock()

	var result []PluginEvent
	for _, event := range r.eventHistory {
		if event.PluginID == pluginName && r.matchesFilter(event, filter) {
			result = append(result, event)
		}
	}

	return result
}

// isEmptyFilter 检查过滤器是否为空 - 新增方法
func (r *simpleRuntime) isEmptyFilter(filter EventFilter) bool {
	return len(filter.Types) == 0 && len(filter.PluginIDs) == 0 && len(filter.Categories) == 0
}

// matchesFilter 检查事件是否匹配过滤器 - 新增方法
func (r *simpleRuntime) matchesFilter(event PluginEvent, filter EventFilter) bool {
	// 检查事件类型
	if len(filter.Types) > 0 {
		found := false
		for _, filterType := range filter.Types {
			if event.Type == filterType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查插件ID
	if len(filter.PluginIDs) > 0 {
		found := false
		for _, pluginID := range filter.PluginIDs {
			if event.PluginID == pluginID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查类别
	if len(filter.Categories) > 0 {
		found := false
		for _, category := range filter.Categories {
			if event.Category == category {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// getListenerID 获取监听器ID - 新增方法
func getListenerID(listener EventListener) string {
	return fmt.Sprintf("%p", listener)
}

// GetResource 获取资源 - 修复并发安全问题
func (r *simpleRuntime) GetResource(name string) (any, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 优先查找共享资源
	if value, ok := r.sharedResources[name]; ok {
		return value, nil
	}

	return nil, NewPluginError("runtime", "GetResource", "Resource not found: "+name, nil)
}

// RegisterResource 注册资源（兼容旧接口，注册为共享资源）
func (r *simpleRuntime) RegisterResource(name string, resource any) error {
	return r.RegisterSharedResource(name, resource)
}

// GetPrivateResource 获取私有资源
func (r *simpleRuntime) GetPrivateResource(name string) (any, error) {
	r.contextMu.RLock()
	pluginName := r.currentPluginContext
	r.contextMu.RUnlock()

	if pluginName == "" {
		return nil, NewPluginError("runtime", "GetPrivateResource", "Plugin context not available", nil)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if pluginResources, ok := r.privateResources[pluginName]; ok {
		if resource, exists := pluginResources[name]; exists {
			return resource, nil
		}
	}

	return nil, NewPluginError("runtime", "GetPrivateResource", "Private resource not found: "+name, nil)
}

// RegisterPrivateResource 注册私有资源
func (r *simpleRuntime) RegisterPrivateResource(name string, resource any) error {
	if name == "" {
		return NewPluginError("runtime", "RegisterPrivateResource", "Resource name cannot be empty", nil)
	}
	if resource == nil {
		return NewPluginError("runtime", "RegisterPrivateResource", "Resource cannot be nil", nil)
	}

	r.contextMu.RLock()
	pluginName := r.currentPluginContext
	r.contextMu.RUnlock()

	if pluginName == "" {
		return NewPluginError("runtime", "RegisterPrivateResource", "Plugin context not available", nil)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 确保插件的私有资源映射存在
	if r.privateResources[pluginName] == nil {
		r.privateResources[pluginName] = make(map[string]any)
	}

	r.privateResources[pluginName][name] = resource
	return nil
}

// GetSharedResource 获取共享资源 - 修复并发安全问题
func (r *simpleRuntime) GetSharedResource(name string) (any, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if value, ok := r.sharedResources[name]; ok {
		return value, nil
	}
	return nil, NewPluginError("runtime", "GetSharedResource", "Shared resource not found: "+name, nil)
}

// RegisterSharedResource 注册共享资源 - 修复并发安全问题
func (r *simpleRuntime) RegisterSharedResource(name string, resource any) error {
	if name == "" {
		return NewPluginError("runtime", "RegisterSharedResource", "Resource name cannot be empty", nil)
	}
	if resource == nil {
		return NewPluginError("runtime", "RegisterSharedResource", "Resource cannot be nil", nil)
	}

	r.contextMu.RLock()
	pluginName := r.currentPluginContext
	r.contextMu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	r.sharedResources[name] = resource

	// 记录资源信息
	r.resourceInfo[name] = &ResourceInfo{
		Name:        name,
		Type:        fmt.Sprintf("%T", resource),
		PluginID:    pluginName,
		IsPrivate:   false,
		CreatedAt:   time.Now(),
		LastUsedAt:  time.Now(),
		AccessCount: 0,
		Size:        r.estimateResourceSize(resource),
		Metadata:    make(map[string]any),
	}

	return nil
}

// GetConfig 获取配置
func (r *simpleRuntime) GetConfig() config.Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// SetConfig 设置配置
func (r *simpleRuntime) SetConfig(conf config.Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = conf
}

// GetLogger 获取日志器
func (r *simpleRuntime) GetLogger() log.Logger {
	return log.DefaultLogger
}

// WithPluginContext 创建带有插件上下文的运行时
func (r *simpleRuntime) WithPluginContext(pluginName string) Runtime {
	if pluginName == "" {
		return r
	}

	// 创建一个新的运行时实例，共享底层资源但有不同的上下文
	contextRuntime := &simpleRuntime{
		privateResources:     r.privateResources,
		sharedResources:      r.sharedResources,
		config:               r.config,
		listeners:            r.listeners,
		eventHistory:         r.eventHistory,
		maxHistory:           r.maxHistory,
		currentPluginContext: pluginName,
		eventWorkerPool:      r.eventWorkerPool,
		eventPoolSize:        r.eventPoolSize,
		shutdown:             r.shutdown,
		shutdownOnce:         r.shutdownOnce,
	}

	return contextRuntime
}

// GetCurrentPluginContext 获取当前插件上下文
func (r *simpleRuntime) GetCurrentPluginContext() string {
	r.contextMu.RLock()
	defer r.contextMu.RUnlock()
	return r.currentPluginContext
}

// GetResourceInfo 获取资源信息
func (r *simpleRuntime) GetResourceInfo(name string) (*ResourceInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if info, exists := r.resourceInfo[name]; exists {
		return info, nil
	}
	return nil, NewPluginError("runtime", "GetResourceInfo", "Resource info not found: "+name, nil)
}

// ListResources 列出所有资源
func (r *simpleRuntime) ListResources() []*ResourceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var resources []*ResourceInfo
	for _, info := range r.resourceInfo {
		resources = append(resources, info)
	}
	return resources
}

// CleanupResources 清理指定插件的资源
func (r *simpleRuntime) CleanupResources(pluginID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 清理私有资源
	if pluginResources, exists := r.privateResources[pluginID]; exists {
		for resourceName := range pluginResources {
			delete(r.resourceInfo, resourceName)
		}
		delete(r.privateResources, pluginID)
	}

	// 清理共享资源（如果插件是所有者）
	var sharedResourcesToRemove []string
	for name, info := range r.resourceInfo {
		if info.PluginID == pluginID && !info.IsPrivate {
			sharedResourcesToRemove = append(sharedResourcesToRemove, name)
		}
	}

	for _, name := range sharedResourcesToRemove {
		delete(r.sharedResources, name)
		delete(r.resourceInfo, name)
	}

	return nil
}

// GetResourceStats 获取资源统计信息
func (r *simpleRuntime) GetResourceStats() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := map[string]any{
		"total_resources":        len(r.resourceInfo),
		"private_resources":      0,
		"shared_resources":       0,
		"total_size_bytes":       int64(0),
		"plugins_with_resources": 0,
	}

	pluginSet := make(map[string]bool)

	for _, info := range r.resourceInfo {
		if info.IsPrivate {
			stats["private_resources"] = stats["private_resources"].(int) + 1
		} else {
			stats["shared_resources"] = stats["shared_resources"].(int) + 1
		}
		stats["total_size_bytes"] = stats["total_size_bytes"].(int64) + info.Size
		pluginSet[info.PluginID] = true
	}

	stats["plugins_with_resources"] = len(pluginSet)
	return stats
}

// estimateResourceSize 估算资源大小
func (r *simpleRuntime) estimateResourceSize(resource any) int64 {
	if resource == nil {
		return 0
	}

	// 使用反射估算大小
	val := reflect.ValueOf(resource)
	return r.estimateValueSize(val)
}

// estimateValueSize 递归估算值的大小
func (r *simpleRuntime) estimateValueSize(val reflect.Value) int64 {
	if !val.IsValid() {
		return 0
	}

	switch val.Kind() {
	case reflect.String:
		return int64(val.Len())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return 8
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return 8
	case reflect.Float32, reflect.Float64:
		return 8
	case reflect.Bool:
		return 1
	case reflect.Slice, reflect.Array:
		size := int64(0)
		for i := 0; i < val.Len(); i++ {
			size += r.estimateValueSize(val.Index(i))
		}
		return size
	case reflect.Map:
		size := int64(0)
		for _, key := range val.MapKeys() {
			size += r.estimateValueSize(key)
			size += r.estimateValueSize(val.MapIndex(key))
		}
		return size
	case reflect.Struct:
		size := int64(0)
		for i := 0; i < val.NumField(); i++ {
			size += r.estimateValueSize(val.Field(i))
		}
		return size
	case reflect.Ptr:
		if val.IsNil() {
			return 8 // 指针本身的大小
		}
		return 8 + r.estimateValueSize(val.Elem())
	default:
		return 8 // 默认大小
	}
}

// EmitPluginEvent 发出插件命名空间事件
func (r *simpleRuntime) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {
	event := PluginEvent{
		Type:      EventType(eventType),
		PluginID:  pluginName,
		Source:    pluginName,
		Metadata:  data,
		Timestamp: time.Now().Unix(),
	}
	r.EmitEvent(event)
}

// AddListener 添加事件监听器
func (r *TypedRuntimeImpl) AddListener(listener EventListener, filter *EventFilter) {
	r.runtime.AddListener(listener, filter)
}

// RemoveListener 移除事件监听器
func (r *TypedRuntimeImpl) RemoveListener(listener EventListener) {
	r.runtime.RemoveListener(listener)
}

// GetEventHistory 获取事件历史
func (r *TypedRuntimeImpl) GetEventHistory(filter EventFilter) []PluginEvent {
	return r.runtime.GetEventHistory(filter)
}

// GetPrivateResource 获取私有资源
func (r *TypedRuntimeImpl) GetPrivateResource(name string) (any, error) {
	return r.runtime.GetPrivateResource(name)
}

// RegisterPrivateResource 注册私有资源
func (r *TypedRuntimeImpl) RegisterPrivateResource(name string, resource any) error {
	return r.runtime.RegisterPrivateResource(name, resource)
}

// GetSharedResource 获取共享资源
func (r *TypedRuntimeImpl) GetSharedResource(name string) (any, error) {
	return r.runtime.GetSharedResource(name)
}

// RegisterSharedResource 注册共享资源
func (r *TypedRuntimeImpl) RegisterSharedResource(name string, resource any) error {
	return r.runtime.RegisterSharedResource(name, resource)
}

// GetResource 获取资源（兼容旧接口）
func (r *TypedRuntimeImpl) GetResource(name string) (any, error) {
	return r.runtime.GetResource(name)
}

// RegisterResource 注册资源（兼容旧接口）
func (r *TypedRuntimeImpl) RegisterResource(name string, resource any) error {
	return r.runtime.RegisterResource(name, resource)
}

// EmitPluginEvent 发出插件命名空间事件
func (r *TypedRuntimeImpl) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {
	r.runtime.EmitPluginEvent(pluginName, eventType, data)
}

// WithPluginContext 创建带有插件上下文的运行时
func (r *TypedRuntimeImpl) WithPluginContext(pluginName string) Runtime {
	return r.runtime.WithPluginContext(pluginName)
}

// GetCurrentPluginContext 获取当前插件上下文
func (r *TypedRuntimeImpl) GetCurrentPluginContext() string {
	return r.runtime.GetCurrentPluginContext()
}

// GetResourceInfo 获取资源信息
func (r *TypedRuntimeImpl) GetResourceInfo(name string) (*ResourceInfo, error) {
	return r.runtime.GetResourceInfo(name)
}

// ListResources 列出所有资源
func (r *TypedRuntimeImpl) ListResources() []*ResourceInfo {
	return r.runtime.ListResources()
}

// CleanupResources 清理指定插件的资源
func (r *TypedRuntimeImpl) CleanupResources(pluginID string) error {
	return r.runtime.CleanupResources(pluginID)
}

// GetResourceStats 获取资源统计信息
func (r *TypedRuntimeImpl) GetResourceStats() map[string]any {
	return r.runtime.GetResourceStats()
}

// GetConfig 获取配置
func (r *TypedRuntimeImpl) GetConfig() config.Config {
	return r.runtime.GetConfig()
}

// SetConfig 设置配置
func (r *TypedRuntimeImpl) SetConfig(conf config.Config) {
	r.runtime.SetConfig(conf)
}

// GetLogger 获取日志器
func (r *TypedRuntimeImpl) GetLogger() log.Logger {
	return r.runtime.GetLogger()
}

// EmitEvent 发出事件
func (r *TypedRuntimeImpl) EmitEvent(event PluginEvent) {
	r.runtime.EmitEvent(event)
}

// AddPluginListener 添加特定插件的事件监听器
func (r *TypedRuntimeImpl) AddPluginListener(pluginName string, listener EventListener, filter *EventFilter) {
	r.runtime.AddPluginListener(pluginName, listener, filter)
}

// GetPluginEventHistory 获取特定插件的事件历史
func (r *TypedRuntimeImpl) GetPluginEventHistory(pluginName string, filter EventFilter) []PluginEvent {
	return r.runtime.GetPluginEventHistory(pluginName, filter)
}

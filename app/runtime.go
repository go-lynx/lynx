package app

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
)

type RuntimePlugin struct {
	// resources stores shared resources between plugins
	// resources 存储插件之间共享的资源
	resources sync.Map

	// eventListeners stores registered event listeners with their filters
	// eventListeners 存储已注册的事件监听器及其过滤器
	listeners []listenerEntry

	// eventHistory stores historical events for querying
	// eventHistory 存储用于查询的历史事件
	eventHistory []plugins.PluginEvent

	// maxHistorySize is the maximum number of events to keep in history
	// maxHistorySize 是历史记录中保留的最大事件数
	maxHistorySize int

	// mu protects the listeners and eventHistory
	// mu 保护 listeners 和 eventHistory
	mu sync.RWMutex

	// logger is the plugin's logger instance
	// logger 是插件的日志记录器实例
	logger log.Logger

	// config is the plugin's configuration
	// config 是插件的配置
	config config.Config
}

// listenerEntry represents a registered event listener with its filter
// listenerEntry 表示一个已注册的事件监听器及其过滤器
type listenerEntry struct {
	listener plugins.EventListener
	filter   *plugins.EventFilter
}

// NewRuntimePlugin creates a new RuntimePlugin instance with default settings.
// NewRuntimePlugin 创建一个带有默认设置的 RuntimePlugin 实例。
func NewRuntimePlugin() *RuntimePlugin {
	return &RuntimePlugin{
		maxHistorySize: 1000, // Default to keeping last 1000 events
		listeners:      make([]listenerEntry, 0),
		eventHistory:   make([]plugins.PluginEvent, 0),
		logger:         log.DefaultLogger,
	}
}

// RegisterResource registers a resource to be shared with other plugins
// Returns error if registration fails
// RegisterResource 注册一个资源，以便与其他插件共享。
// 如果注册失败，则返回错误。

// GetResource retrieves a shared plugin resource by name
// Returns the resource and any error encountered
// GetResource 根据名称获取插件共享资源。
// 返回资源和可能遇到的错误。
func (r *RuntimePlugin) GetResource(name string) (any, error) {
	if value, ok := r.resources.Load(name); ok {
		return value, nil
	}
	return nil, fmt.Errorf("resource not found: %s", name)
}

// RegisterResource registers a resource to be shared with other plugins
// Returns error if registration fails
// RegisterResource 注册一个资源，以便与其他插件共享。
// 如果注册失败，则返回错误。
func (r *RuntimePlugin) RegisterResource(name string, resource any) error {
	if name == "" {
		return fmt.Errorf("resource name cannot be empty")
	}
	if resource == nil {
		return fmt.Errorf("resource cannot be nil")
	}
	
	// Store the resource using sync.Map
	r.resources.Store(name, resource)
	return nil
}

// GetConfig returns the plugin configuration manager
// Provides access to configuration values and updates
// GetConfig 返回插件配置管理器。
// 提供对配置值和更新的访问。
func (r *RuntimePlugin) GetConfig() config.Config {
	if r.config == nil {
		r.config = Lynx().GetGlobalConfig()
	}
	return r.config
}

// GetLogger returns the plugin logger instance
// Provides structured logging capabilities
// GetLogger 返回插件日志记录器实例。
// 提供结构化的日志记录功能。
func (r *RuntimePlugin) GetLogger() log.Logger {
	if r.logger == nil {
		// Initialize with a default logger if not set
		r.logger = log.DefaultLogger
	}
	return r.logger
}

// EmitEvent broadcasts a plugin event to all registered listeners.
// Event will be processed according to its priority and any active filters.
// EmitEvent 向所有注册的监听器广播一个插件事件。
// 事件将根据其优先级和任何活动的过滤器进行处理。
func (r *RuntimePlugin) EmitEvent(event plugins.PluginEvent) {
	if event.Type == "" { // Check for zero value of EventType
		return
	}

	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Add to history
	r.eventHistory = append(r.eventHistory, event)

	// Trim history if it exceeds max size
	if r.maxHistorySize > 0 && len(r.eventHistory) > r.maxHistorySize {
		r.eventHistory = r.eventHistory[len(r.eventHistory)-r.maxHistorySize:]
	}

	// Notify listeners
	for _, entry := range r.listeners {
		if entry.filter == nil || r.eventMatchesFilter(event, *entry.filter) {
			// Non-blocking event dispatch
			go entry.listener.HandleEvent(event)
		}
	}
}

// AddListener registers a new event listener with optional filters.
// Listener will only receive events that match its filter criteria.
// AddListener 使用可选的过滤器注册一个新的事件监听器。
// 监听器将仅接收符合其过滤条件的事件。
func (r *RuntimePlugin) AddListener(listener plugins.EventListener, filter *plugins.EventFilter) {
	if listener == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Add new listener with its filter
	r.listeners = append(r.listeners, listenerEntry{
		listener: listener,
		filter:   filter,
	})
}

// RemoveListener unregisters an event listener.
// After removal, the listener will no longer receive any events.
// RemoveListener 注销一个事件监听器。
// 删除后，该监听器将不再接收任何事件。
func (r *RuntimePlugin) RemoveListener(listener plugins.EventListener) {
	if listener == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove the listener
	newListeners := make([]listenerEntry, 0, len(r.listeners))
	for _, entry := range r.listeners {
		if entry.listener != listener {
			newListeners = append(newListeners, entry)
		}
	}
	r.listeners = newListeners
}

// GetEventHistory retrieves historical events based on filter criteria.
// Returns events that match the specified filter parameters.
// GetEventHistory 根据过滤条件检索历史事件。
// 返回符合指定过滤参数的事件。
func (r *RuntimePlugin) GetEventHistory(filter plugins.EventFilter) []plugins.PluginEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// If no filter criteria are set, return all events
	if len(filter.Types) == 0 && len(filter.Categories) == 0 && 
		len(filter.PluginIDs) == 0 && len(filter.Priorities) == 0 && 
		filter.FromTime == 0 && filter.ToTime == 0 {
		result := make([]plugins.PluginEvent, len(r.eventHistory))
		copy(result, r.eventHistory)
		return result
	}

	// Apply filter
	result := make([]plugins.PluginEvent, 0, len(r.eventHistory))
	for _, event := range r.eventHistory {
		if r.eventMatchesFilter(event, filter) {
			result = append(result, event)
		}
	}
	return result
}

// eventMatchesFilter checks if an event matches a specific filter.
// This implements the detailed filter matching logic.
// eventMatchesFilter 检查一个事件是否匹配特定的过滤器。
// 这实现了详细的过滤器匹配逻辑。
func (r *RuntimePlugin) eventMatchesFilter(event plugins.PluginEvent, filter plugins.EventFilter) bool {
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

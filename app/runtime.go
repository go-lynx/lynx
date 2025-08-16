package app

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
)

// 默认事件队列与worker参数
const (
	defaultEventQueueSize   = 1024
	defaultEventWorkerCount = 10
	// 每个监听器的独立队列大小，避免单个慢监听器阻塞全局
	defaultListenerQueueSize = 256
	// 历史事件默认 1000 条，<=0 表示不保留
	defaultHistorySize = 1000
	// 关闭时排空监听器队列的默认超时（毫秒）。默认 500ms，兼顾快速关停与尽量不丢事件
	defaultDrainTimeoutMs = 500
)

// TypedRuntimePlugin 泛型运行时插件
type TypedRuntimePlugin struct {
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

	// 事件队列与worker
	// eventCh 承载事件的缓冲通道，提供背压
	eventCh chan plugins.PluginEvent
	// workerCount 工作协程数量
	workerCount int
	// 每个监听器的独立队列大小
	listenerQueueSize int
	// 关闭控制
	closeOnce sync.Once
	closed    chan struct{}

	// goroutine 跟踪
	workerWg   sync.WaitGroup
	listenerWg sync.WaitGroup

	// 停止标记，Emit 阶段快速丢弃
	stopped int32

	// 生产者发送互斥，避免 Close 同步关闭 eventCh 时发生并发发送
	sendMu sync.Mutex

	// 排空监听器队列超时
	drainTimeout time.Duration
}

// listenerEntry represents a registered event listener with its filter
// listenerEntry 表示一个已注册的事件监听器及其过滤器
type listenerEntry struct {
	listener plugins.EventListener
	filter   *plugins.EventFilter
	// 每个监听器的独立事件队列
	ch chan plugins.PluginEvent
	// 监听器退出信号，不关闭 ch 以避免 worker 发送时的竞态
	quit chan struct{}
	// 监听器活跃标志（共享指针，便于在快照后仍能感知 Remove 的状态变更）1=active, 0=inactive
	active *int32
}

// NewTypedRuntimePlugin creates a new TypedRuntimePlugin instance with default settings.
// NewTypedRuntimePlugin 创建一个带有默认设置的 TypedRuntimePlugin 实例。
func NewTypedRuntimePlugin() *TypedRuntimePlugin {
	// 读取可配置的队列大小与 worker 数量（仅从 boot 配置）
	qsize := defaultEventQueueSize
	wcount := defaultEventWorkerCount
	lqsize := defaultListenerQueueSize
	hsize := defaultHistorySize
	drainMs := defaultDrainTimeoutMs
	if app := Lynx(); app != nil {
		// 仅使用引导配置（boot.pb.go）；未配置则使用默认值
		if app.bootConfig != nil && app.bootConfig.Lynx != nil && app.bootConfig.Lynx.Runtime != nil && app.bootConfig.Lynx.Runtime.Event != nil {
			if v := app.bootConfig.Lynx.Runtime.Event.QueueSize; v > 0 {
				qsize = int(v)
			}
			if v := app.bootConfig.Lynx.Runtime.Event.WorkerCount; v > 0 {
				wcount = int(v)
			}
			if v := app.bootConfig.Lynx.Runtime.Event.ListenerQueueSize; v > 0 {
				lqsize = int(v)
			}
			// 历史事件大小（<=0 表示不保留）
			if v := app.bootConfig.Lynx.Runtime.Event.HistorySize; v != 0 {
				hsize = int(v)
			}
			// 关闭排空超时（毫秒）
			if v := app.bootConfig.Lynx.Runtime.Event.DrainTimeoutMs; v > 0 {
				drainMs = int(v)
			}
		}
	}

	r := &TypedRuntimePlugin{
		maxHistorySize:    hsize, // 历史事件保留条数（<=0 表示不保留）
		listeners:         make([]listenerEntry, 0),
		eventHistory:      make([]plugins.PluginEvent, 0),
		logger:            log.DefaultLogger,
		eventCh:           make(chan plugins.PluginEvent, qsize),
		workerCount:       wcount,
		listenerQueueSize: lqsize,
		closed:            make(chan struct{}),
	}
	if drainMs > 0 {
		r.drainTimeout = time.Duration(drainMs) * time.Millisecond
	}
	// 启动固定数量的事件分发worker
	for i := 0; i < r.workerCount; i++ {
		r.workerWg.Add(1)
		go r.eventWorkerLoop()
	}
	return r
}

// GetResource retrieves a shared plugin resource by name
// Returns the resource and any error encountered
// GetResource 根据名称获取插件共享资源。
// 返回资源和可能遇到的错误。
func (r *TypedRuntimePlugin) GetResource(name string) (any, error) {
	if value, ok := r.resources.Load(name); ok {
		return value, nil
	}
	return nil, fmt.Errorf("resource not found: %s", name)
}

// RegisterResource registers a resource to be shared with other plugins
// Returns error if registration fails
// RegisterResource 注册一个资源，以便与其他插件共享。
// 如果注册失败，则返回错误。
func (r *TypedRuntimePlugin) RegisterResource(name string, resource any) error {
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

// GetTypedResource 获取类型安全的资源（独立函数）
func GetTypedResource[T any](r *TypedRuntimePlugin, name string) (T, error) {
	var zero T
	resource, err := r.GetResource(name)
	if err != nil {
		return zero, err
	}

	typed, ok := resource.(T)
	if !ok {
		return zero, fmt.Errorf("type assertion failed for resource %s", name)
	}

	return typed, nil
}

// RegisterTypedResource 注册类型安全的资源（独立函数）
func RegisterTypedResource[T any](r *TypedRuntimePlugin, name string, resource T) error {
	return r.RegisterResource(name, resource)
}

// GetConfig returns the plugin configuration manager
// Provides access to configuration values and updates
// GetConfig 返回插件配置管理器。
// 提供对配置值和更新的访问。
func (r *TypedRuntimePlugin) GetConfig() config.Config {
	if r.config == nil {
		if app := Lynx(); app != nil {
			if cfg := app.GetGlobalConfig(); cfg != nil {
				r.config = cfg
			}
		}
	}
	return r.config
}

// GetLogger returns the plugin logger instance
// Provides structured logging capabilities
// GetLogger 返回插件日志记录器实例。
// 提供结构化的日志记录功能。
func (r *TypedRuntimePlugin) GetLogger() log.Logger {
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
func (r *TypedRuntimePlugin) EmitEvent(event plugins.PluginEvent) {
	if event.Type == "" { // Check for zero value of EventType
		return
	}

	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}

	// 已停止则直接丢弃
	if atomic.LoadInt32(&r.stopped) == 1 {
		return
	}

	// 先写入历史（可关闭），后入队
	if r.maxHistorySize > 0 {
		r.mu.Lock()
		// Add to history
		r.eventHistory = append(r.eventHistory, event)
		// Trim history if it exceeds max size
		if len(r.eventHistory) > r.maxHistorySize {
			r.eventHistory = r.eventHistory[len(r.eventHistory)-r.maxHistorySize:]
		}
		r.mu.Unlock()
	}

	// 将事件写入队列；队列满时丢弃并记录debug日志，避免无背压的goroutine泛滥
	r.sendMu.Lock()
	select {
	case r.eventCh <- event:
	default:
		if r.logger != nil {
			_ = r.logger.Log(log.LevelDebug, "msg", "event queue full, dropping event", "type", event.Type, "plugin", event.PluginID)
		}
	}
	r.sendMu.Unlock()
}

// eventWorkerLoop 顺序处理事件并派发给符合过滤条件的监听器
func (r *TypedRuntimePlugin) eventWorkerLoop() {
	defer r.workerWg.Done()
	for ev := range r.eventCh {
		// 复制监听器快照，避免长时间持锁
		r.mu.RLock()
		listeners := make([]listenerEntry, len(r.listeners))
		copy(listeners, r.listeners)
		r.mu.RUnlock()

		for _, entry := range listeners {
			// 若监听器已被标记为不活跃，则跳过
			if entry.active != nil && atomic.LoadInt32(entry.active) == 0 {
				continue
			}
			if entry.filter == nil || r.eventMatchesFilter(ev, *entry.filter) {
				// 分发到监听器独立队列，避免头阻塞
				select {
				case entry.ch <- ev:
				default:
					if r.logger != nil {
						_ = r.logger.Log(log.LevelDebug, "msg", "listener queue full, dropping event", "listener_id", entry.listener.GetListenerID(), "type", ev.Type)
					}
				}
			}
		}
	}
}

// Close 关闭事件分发（可选调用）
func (r *TypedRuntimePlugin) Close() {
	r.closeOnce.Do(func() {
		// Phase 1: 停止接收新事件并让 worker 自然退出
		atomic.StoreInt32(&r.stopped, 1)
		// 与生产者互斥，避免 send on closed channel
		r.sendMu.Lock()
		close(r.eventCh)
		r.sendMu.Unlock()
		r.workerWg.Wait()

		// Phase 2.1: 可选排空监听器队列（仅等待，不再继续生产）
		if r.drainTimeout > 0 {
			deadline := time.Now().Add(r.drainTimeout)
			for {
				allEmpty := true
				r.mu.RLock()
				for _, entry := range r.listeners {
					if entry.ch != nil && len(entry.ch) > 0 {
						allEmpty = false
						break
					}
				}
				r.mu.RUnlock()
				if allEmpty || time.Now().After(deadline) {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}

		// Phase 2.2: 通知监听器退出并等待其优雅退出
		r.mu.RLock()
		for _, entry := range r.listeners {
			if entry.active != nil {
				atomic.StoreInt32(entry.active, 0)
			}
			if entry.quit != nil {
				close(entry.quit)
			}
		}
		r.mu.RUnlock()
		r.listenerWg.Wait()

		// 最后标记 closed（可用于其他 select 信号）
		close(r.closed)
	})
}

// AddListener registers a new event listener with optional filters.
// Listener will only receive events that match its filter criteria.
// AddListener 使用可选的过滤器注册一个新的事件监听器。
// 监听器将仅接收符合其过滤条件的事件。
func (r *TypedRuntimePlugin) AddListener(listener plugins.EventListener, filter *plugins.EventFilter) {
	if listener == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Add new listener with its filter
	var activeFlag int32 = 1
	entry := listenerEntry{
		listener: listener,
		filter:   filter,
		ch:       make(chan plugins.PluginEvent, r.listenerQueueSize),
		quit:     make(chan struct{}),
		active:   &activeFlag,
	}
	r.listeners = append(r.listeners, entry)

	// 启动监听器独立 goroutine
	r.listenerWg.Add(1)
	go func(le listenerEntry) {
		defer r.listenerWg.Done()
		for {
			select {
			case <-le.quit:
				return
			case ev, ok := <-le.ch:
				if !ok {
					return
				}
				func() {
					defer func() {
						if rec := recover(); rec != nil {
							if r.logger != nil {
								_ = r.logger.Log(log.LevelError, "msg", "panic in EventListener.HandleEvent", "listener_id", le.listener.GetListenerID(), "err", rec)
							}
						}
					}()
					le.listener.HandleEvent(ev)
				}()
			}
		}
	}(entry)
}

// RemoveListener unregisters an event listener.
// After removal, the listener will no longer receive any events.
// RemoveListener 注销一个事件监听器。
// 删除后，该监听器将不再接收任何事件。
func (r *TypedRuntimePlugin) RemoveListener(listener plugins.EventListener) {
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
		} else {
			// 通知监听器退出，但不关闭其事件通道，避免 worker 向已关闭通道发送
			if entry.active != nil {
				atomic.StoreInt32(entry.active, 0)
			}
			if entry.quit != nil {
				close(entry.quit)
			}
		}
	}
	r.listeners = newListeners
}

// GetEventHistory retrieves historical events based on filter criteria.
// Returns events that match the specified filter parameters.
// GetEventHistory 根据过滤条件检索历史事件。
// 返回符合指定过滤参数的事件。
func (r *TypedRuntimePlugin) GetEventHistory(filter plugins.EventFilter) []plugins.PluginEvent {
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
func (r *TypedRuntimePlugin) eventMatchesFilter(event plugins.PluginEvent, filter plugins.EventFilter) bool {
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

// RuntimePlugin 保持向后兼容的运行时插件
type RuntimePlugin = TypedRuntimePlugin

// NewRuntimePlugin 创建运行时插件（向后兼容）
func NewRuntimePlugin() *RuntimePlugin {
	return NewTypedRuntimePlugin()
}

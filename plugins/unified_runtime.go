package plugins

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
)

// UnifiedRuntime 统一的Runtime实现，整合了所有现有功能
type UnifiedRuntime struct {
	// 资源管理 - 使用sync.Map提供更好的并发性能
	resources *sync.Map // map[string]any - 存储所有资源
	
	// 资源信息跟踪
	resourceInfo *sync.Map // map[string]*ResourceInfo
	
	// 配置和日志
	config config.Config
	logger log.Logger
	
	// 插件上下文管理
	currentPluginContext string
	contextMu           sync.RWMutex
	
	// 事件系统 - 使用统一的事件总线
	eventManager interface{} // 避免循环依赖，运行时设置
	
	// 性能配置
	workerPoolSize int
	eventTimeout   time.Duration
	
	// 运行时状态
	closed bool
	mu     sync.RWMutex
}

// NewUnifiedRuntime 创建新的统一Runtime实例
func NewUnifiedRuntime() *UnifiedRuntime {
	return &UnifiedRuntime{
		resources:      &sync.Map{},
		resourceInfo:   &sync.Map{},
		logger:         log.DefaultLogger,
		workerPoolSize: 10,
		eventTimeout:   5 * time.Second,
		closed:         false,
	}
}

// ============================================================================
// 资源管理接口实现
// ============================================================================

// GetResource 获取资源（兼容旧接口）
func (r *UnifiedRuntime) GetResource(name string) (any, error) {
	return r.GetSharedResource(name)
}

// RegisterResource 注册资源（兼容旧接口）
func (r *UnifiedRuntime) RegisterResource(name string, resource any) error {
	return r.RegisterSharedResource(name, resource)
}

// GetSharedResource 获取共享资源
func (r *UnifiedRuntime) GetSharedResource(name string) (any, error) {
	if r.isClosed() {
		return nil, fmt.Errorf("runtime is closed")
	}
	
	if name == "" {
		return nil, fmt.Errorf("resource name cannot be empty")
	}
	
	value, ok := r.resources.Load(name)
	if !ok {
		return nil, fmt.Errorf("resource not found: %s", name)
	}
	
	// 更新访问统计
	r.updateAccessStats(name, false, "")
	
	return value, nil
}

// RegisterSharedResource 注册共享资源
func (r *UnifiedRuntime) RegisterSharedResource(name string, resource any) error {
	if r.isClosed() {
		return fmt.Errorf("runtime is closed")
	}
	
	if name == "" {
		return fmt.Errorf("resource name cannot be empty")
	}
	
	if resource == nil {
		return fmt.Errorf("resource cannot be nil")
	}
	
	// 存储资源
	r.resources.Store(name, resource)
	
	// 创建资源信息
	info := &ResourceInfo{
		Name:        name,
		Type:        reflect.TypeOf(resource).String(),
		PluginID:    r.getCurrentPluginContext(),
		IsPrivate:   false,
		CreatedAt:   time.Now(),
		LastUsedAt:  time.Now(),
		AccessCount: 0,
		Size:        r.estimateResourceSize(resource),
		Metadata:    make(map[string]any),
	}
	
	r.resourceInfo.Store(name, info)
	
	return nil
}

// GetPrivateResource 获取私有资源（插件特定）
func (r *UnifiedRuntime) GetPrivateResource(name string) (any, error) {
	pluginID := r.getCurrentPluginContext()
	if pluginID == "" {
		return nil, fmt.Errorf("no plugin context set")
	}
	
	privateKey := fmt.Sprintf("%s:%s", pluginID, name)
	return r.GetSharedResource(privateKey)
}

// RegisterPrivateResource 注册私有资源（插件特定）
func (r *UnifiedRuntime) RegisterPrivateResource(name string, resource any) error {
	if r.isClosed() {
		return fmt.Errorf("runtime is closed")
	}
	
	pluginID := r.getCurrentPluginContext()
	if pluginID == "" {
		return fmt.Errorf("no plugin context set")
	}
	
	if name == "" {
		return fmt.Errorf("resource name cannot be empty")
	}
	
	if resource == nil {
		return fmt.Errorf("resource cannot be nil")
	}
	
	privateKey := fmt.Sprintf("%s:%s", pluginID, name)
	
	// 存储资源
	r.resources.Store(privateKey, resource)
	
	// 创建私有资源信息
	info := &ResourceInfo{
		Name:        privateKey,
		Type:        reflect.TypeOf(resource).String(),
		PluginID:    pluginID,
		IsPrivate:   true,
		CreatedAt:   time.Now(),
		LastUsedAt:  time.Now(),
		AccessCount: 0,
		Size:        r.estimateResourceSize(resource),
		Metadata:    make(map[string]any),
	}
	
	r.resourceInfo.Store(privateKey, info)
	
	return nil
}

// ============================================================================
// 配置和日志接口实现
// ============================================================================

// GetConfig 获取配置
func (r *UnifiedRuntime) GetConfig() config.Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// SetConfig 设置配置
func (r *UnifiedRuntime) SetConfig(conf config.Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = conf
}

// GetLogger 获取日志器
func (r *UnifiedRuntime) GetLogger() log.Logger {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.logger == nil {
		return log.DefaultLogger
	}
	return r.logger
}

// SetLogger 设置日志器
func (r *UnifiedRuntime) SetLogger(logger log.Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logger = logger
}

// ============================================================================
// 插件上下文管理
// ============================================================================

// WithPluginContext 创建带插件上下文的Runtime
func (r *UnifiedRuntime) WithPluginContext(pluginName string) Runtime {
	// 创建新的Runtime实例，共享底层资源映射
	contextRuntime := &UnifiedRuntime{
		resources:            r.resources,    // 共享同一个资源映射指针
		resourceInfo:         r.resourceInfo, // 共享同一个资源信息映射指针
		config:               r.config,
		logger:               r.logger,
		currentPluginContext: pluginName,
		contextMu:            sync.RWMutex{}, // 初始化mutex
		eventManager:         r.eventManager,
		workerPoolSize:       r.workerPoolSize,
		eventTimeout:         r.eventTimeout,
		closed:               false,
		mu:                   sync.RWMutex{}, // 初始化mutex
	}
	
	return contextRuntime
}

// GetCurrentPluginContext 获取当前插件上下文
func (r *UnifiedRuntime) GetCurrentPluginContext() string {
	return r.getCurrentPluginContext()
}

func (r *UnifiedRuntime) getCurrentPluginContext() string {
	r.contextMu.RLock()
	defer r.contextMu.RUnlock()
	return r.currentPluginContext
}

// ============================================================================
// 事件系统接口实现
// ============================================================================

// EmitEvent 发送事件
func (r *UnifiedRuntime) EmitEvent(event PluginEvent) {
	if r.isClosed() {
		return
	}
	
	if event.Type == "" {
		return
	}
	
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	
	// 使用全局事件总线适配器
	adapter := EnsureGlobalEventBusAdapter()
	if err := adapter.PublishEvent(event); err != nil {
		// 记录错误但不中断操作
		if logger := r.GetLogger(); logger != nil {
			logger.Log(log.LevelError, "msg", "failed to publish event", "error", err, "event_type", event.Type, "plugin_id", event.PluginID)
		}
	}
}

// EmitPluginEvent 发送插件事件
func (r *UnifiedRuntime) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {
	event := PluginEvent{
		Type:      EventType(eventType),
		PluginID:  pluginName,
		Metadata:  data,
		Timestamp: time.Now().Unix(),
	}
	r.EmitEvent(event)
}

// AddListener 添加事件监听器
func (r *UnifiedRuntime) AddListener(listener EventListener, filter *EventFilter) {
	// 委托给统一事件总线
	if listener == nil {
		return
	}
	
	// 转换为统一事件总线监听器
	// 这里简化实现，实际应该使用事件总线的监听器管理
	// TODO: 实现完整的事件监听器管理
}

// RemoveListener 移除事件监听器
func (r *UnifiedRuntime) RemoveListener(listener EventListener) {
	// 委托给统一事件总线
	// TODO: 实现完整的事件监听器管理
}

// AddPluginListener 添加插件特定的事件监听器
func (r *UnifiedRuntime) AddPluginListener(pluginName string, listener EventListener, filter *EventFilter) {
	// 委托给统一事件总线
	// TODO: 实现完整的插件事件监听器管理
}

// GetEventHistory 获取事件历史
func (r *UnifiedRuntime) GetEventHistory(filter EventFilter) []PluginEvent {
	// 委托给统一事件总线
	// TODO: 实现事件历史查询
	return []PluginEvent{}
}

// GetPluginEventHistory 获取插件事件历史
func (r *UnifiedRuntime) GetPluginEventHistory(pluginName string, filter EventFilter) []PluginEvent {
	// 委托给统一事件总线
	// TODO: 实现插件事件历史查询
	return []PluginEvent{}
}

// ============================================================================
// 性能配置接口
// ============================================================================

// SetEventDispatchMode 设置事件分发模式
func (r *UnifiedRuntime) SetEventDispatchMode(mode string) error {
	// 委托给统一事件总线
	adapter := EnsureGlobalEventBusAdapter()
	if configurable, ok := adapter.(interface{ SetDispatchMode(string) error }); ok {
		return configurable.SetDispatchMode(mode)
	}
	return nil
}

// SetEventWorkerPoolSize 设置事件工作池大小
func (r *UnifiedRuntime) SetEventWorkerPoolSize(size int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if size > 0 {
		r.workerPoolSize = size
	}
}

// SetEventTimeout 设置事件超时时间
func (r *UnifiedRuntime) SetEventTimeout(timeout time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if timeout > 0 {
		r.eventTimeout = timeout
	}
}

// GetEventStats 获取事件统计
func (r *UnifiedRuntime) GetEventStats() map[string]any {
	return map[string]any{
		"worker_pool_size": r.workerPoolSize,
		"event_timeout":    r.eventTimeout.String(),
		"runtime_closed":   r.isClosed(),
	}
}

// ============================================================================
// 资源信息和统计
// ============================================================================

// GetResourceInfo 获取资源信息
func (r *UnifiedRuntime) GetResourceInfo(name string) (*ResourceInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("resource name cannot be empty")
	}
	
	value, ok := r.resourceInfo.Load(name)
	if !ok {
		return nil, fmt.Errorf("resource info not found: %s", name)
	}
	
	info, ok := value.(*ResourceInfo)
	if !ok {
		return nil, fmt.Errorf("invalid resource info type for: %s", name)
	}
	
	return info, nil
}

// ListResources 列出所有资源
func (r *UnifiedRuntime) ListResources() []*ResourceInfo {
	var resources []*ResourceInfo
	
	r.resourceInfo.Range(func(key, value interface{}) bool {
		if info, ok := value.(*ResourceInfo); ok {
			resources = append(resources, info)
		}
		return true
	})
	
	return resources
}

// CleanupResources 清理插件资源
func (r *UnifiedRuntime) CleanupResources(pluginID string) error {
	if pluginID == "" {
		return fmt.Errorf("plugin ID cannot be empty")
	}
	
	var toDelete []string
	
	// 收集需要删除的资源
	r.resourceInfo.Range(func(key, value interface{}) bool {
		if info, ok := value.(*ResourceInfo); ok {
			if info.PluginID == pluginID {
				toDelete = append(toDelete, key.(string))
			}
		}
		return true
	})
	
	// 删除资源
	for _, name := range toDelete {
		r.resources.Delete(name)
		r.resourceInfo.Delete(name)
	}
	
	return nil
}

// GetResourceStats 获取资源统计
func (r *UnifiedRuntime) GetResourceStats() map[string]any {
	var totalResources, privateResources, sharedResources int
	var totalSize int64
	
	r.resourceInfo.Range(func(key, value interface{}) bool {
		if info, ok := value.(*ResourceInfo); ok {
			totalResources++
			totalSize += info.Size
			if info.IsPrivate {
				privateResources++
			} else {
				sharedResources++
			}
		}
		return true
	})
	
	return map[string]any{
		"total_resources":  totalResources,
		"private_resources": privateResources,
		"shared_resources":  sharedResources,
		"total_size_bytes":  totalSize,
		"runtime_closed":    r.isClosed(),
	}
}

// ============================================================================
// 生命周期管理
// ============================================================================

// Shutdown 关闭Runtime
func (r *UnifiedRuntime) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.closed {
		return
	}
	
	// 关闭事件总线
	adapter := GetGlobalEventBusAdapter()
	if adapter != nil {
		if shutdownable, ok := adapter.(interface{ Shutdown() error }); ok {
			if err := shutdownable.Shutdown(); err != nil {
				if logger := r.GetLogger(); logger != nil {
					logger.Log(log.LevelWarn, "msg", "failed to shutdown event bus", "error", err)
				}
			}
		}
	}
	
	r.closed = true
}

// Close 关闭Runtime（兼容接口）
func (r *UnifiedRuntime) Close() {
	r.Shutdown()
}

// ============================================================================
// 内部辅助方法
// ============================================================================

func (r *UnifiedRuntime) isClosed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.closed
}

func (r *UnifiedRuntime) updateAccessStats(name string, isPrivate bool, pluginID string) {
	if value, ok := r.resourceInfo.Load(name); ok {
		if info, ok := value.(*ResourceInfo); ok {
			info.LastUsedAt = time.Now()
			info.AccessCount++
		}
	}
}

func (r *UnifiedRuntime) estimateResourceSize(resource any) int64 {
	if resource == nil {
		return 0
	}
	
	// 简化的大小估算
	val := reflect.ValueOf(resource)
	return r.estimateValueSize(val, 0, 3)
}

func (r *UnifiedRuntime) estimateValueSize(val reflect.Value, depth, maxDepth int) int64 {
	if depth > maxDepth {
		return 8 // 默认指针大小
	}
	
	switch val.Kind() {
	case reflect.Bool:
		return 1
	case reflect.Int, reflect.Int32, reflect.Uint, reflect.Uint32:
		return 4
	case reflect.Int64, reflect.Uint64:
		return 8
	case reflect.Float32:
		return 4
	case reflect.Float64:
		return 8
	case reflect.String:
		return int64(len(val.String()))
	case reflect.Slice, reflect.Array:
		size := int64(0)
		for i := 0; i < val.Len() && i < 100; i++ { // 限制检查数量
			size += r.estimateValueSize(val.Index(i), depth+1, maxDepth)
		}
		return size
	case reflect.Map:
		size := int64(0)
		count := 0
		for _, key := range val.MapKeys() {
			if count >= 100 { // 限制检查数量
				break
			}
			size += r.estimateValueSize(key, depth+1, maxDepth)
			size += r.estimateValueSize(val.MapIndex(key), depth+1, maxDepth)
			count++
		}
		return size
	case reflect.Ptr:
		if !val.IsNil() {
			return r.estimateValueSize(val.Elem(), depth+1, maxDepth)
		}
		return 8
	default:
		return 8 // 默认大小
	}
}

// ============================================================================
// 向后兼容的构造函数
// ============================================================================

// 注意：NewSimpleRuntime 和 NewTypedRuntime 函数已在 plugin.go 中定义
// 这里不重复定义以避免冲突
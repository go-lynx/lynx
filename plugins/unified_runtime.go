package plugins

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
)

// UnifiedRuntime is a unified Runtime implementation that consolidates all existing capabilities
type UnifiedRuntime struct {
	// Resource management - use sync.Map for better concurrent performance
	resources *sync.Map // map[string]any - stores all resources
	
	// Resource info tracking
	resourceInfo *sync.Map // map[string]*ResourceInfo
	
	// Configuration and logging
	config config.Config
	logger log.Logger
	
	// Plugin context management
	currentPluginContext string
	contextMu           sync.RWMutex
	
	// Event system - uses a unified event bus
	eventManager interface{} // avoid circular dependency; set at runtime
	
	// Performance configuration
	workerPoolSize int
	eventTimeout   time.Duration
	
	// Runtime state
	closed bool
	mu     sync.RWMutex
}

// NewUnifiedRuntime creates a new unified Runtime instance
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
// Resource management interface
// ============================================================================

// GetResource gets a resource (backward compatible API)
func (r *UnifiedRuntime) GetResource(name string) (any, error) {
	return r.GetSharedResource(name)
}

// RegisterResource registers a resource (backward compatible API)
func (r *UnifiedRuntime) RegisterResource(name string, resource any) error {
	return r.RegisterSharedResource(name, resource)
}

// GetSharedResource retrieves a shared resource
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
	
	// Update access statistics
	r.updateAccessStats(name, false, "")
	
	return value, nil
}

// RegisterSharedResource registers a shared resource
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
	
	// Store resource
	r.resources.Store(name, resource)
	
	// Create resource info
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

// GetPrivateResource gets a private (plugin-scoped) resource
func (r *UnifiedRuntime) GetPrivateResource(name string) (any, error) {
	pluginID := r.getCurrentPluginContext()
	if pluginID == "" {
		return nil, fmt.Errorf("no plugin context set")
	}
	
	privateKey := fmt.Sprintf("%s:%s", pluginID, name)
	return r.GetSharedResource(privateKey)
}

// RegisterPrivateResource registers a private (plugin-scoped) resource
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
	
	// Store resource
	r.resources.Store(privateKey, resource)
	
	// Create private resource info
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
// Configuration and logging interfaces
// ============================================================================

// GetConfig returns the config
func (r *UnifiedRuntime) GetConfig() config.Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// SetConfig sets the config
func (r *UnifiedRuntime) SetConfig(conf config.Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = conf
}

// GetLogger returns the logger
func (r *UnifiedRuntime) GetLogger() log.Logger {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.logger == nil {
		return log.DefaultLogger
	}
	return r.logger
}

// SetLogger sets the logger
func (r *UnifiedRuntime) SetLogger(logger log.Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logger = logger
}

// ============================================================================
// Plugin context management
// ============================================================================

// WithPluginContext creates a Runtime bound with plugin context
func (r *UnifiedRuntime) WithPluginContext(pluginName string) Runtime {
	// Create a new Runtime instance sharing underlying resource maps
	contextRuntime := &UnifiedRuntime{
		resources:            r.resources,    // share the same resource map pointer
		resourceInfo:         r.resourceInfo, // share the same resource info map pointer
		config:               r.config,
		logger:               r.logger,
		currentPluginContext: pluginName,
		contextMu:            sync.RWMutex{}, // initialize mutex
		eventManager:         r.eventManager,
		workerPoolSize:       r.workerPoolSize,
		eventTimeout:         r.eventTimeout,
		closed:               false,
		mu:                   sync.RWMutex{}, // initialize mutex
	}
	
	return contextRuntime
}

// GetCurrentPluginContext returns current plugin context
func (r *UnifiedRuntime) GetCurrentPluginContext() string {
	return r.getCurrentPluginContext()
}

func (r *UnifiedRuntime) getCurrentPluginContext() string {
	r.contextMu.RLock()
	defer r.contextMu.RUnlock()
	return r.currentPluginContext
}

// ============================================================================
// Event system interfaces
// ============================================================================

// EmitEvent publishes an event
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
	
	// Use global event bus adapter
	adapter := EnsureGlobalEventBusAdapter()
	if err := adapter.PublishEvent(event); err != nil {
		// Log error without interrupting operation
		if logger := r.GetLogger(); logger != nil {
			logger.Log(log.LevelError, "msg", "failed to publish event", "error", err, "event_type", event.Type, "plugin_id", event.PluginID)
		}
	}
}

// EmitPluginEvent publishes a plugin event
func (r *UnifiedRuntime) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {
	event := PluginEvent{
		Type:      EventType(eventType),
		PluginID:  pluginName,
		Metadata:  data,
		Timestamp: time.Now().Unix(),
	}
	r.EmitEvent(event)
}

// AddListener adds an event listener
func (r *UnifiedRuntime) AddListener(listener EventListener, filter *EventFilter) {
	// Delegate to the unified event bus
	if listener == nil {
		return
	}

	// Convert to unified event bus listener
	adapter := EnsureGlobalEventBusAdapter()

	id := listener.GetListenerID()
	if id == "" {
		id = fmt.Sprintf("listener-%d", time.Now().UnixNano())
	}

	// Optional interface: listener management (implemented by app/events adapter)
	type addListenerIface interface {
		AddListener(id string, filter *EventFilter, handler func(interface{}), bus string) error
	}

	if al, ok := adapter.(addListenerIface); ok {
		_ = al.AddListener(id, filter, func(ev interface{}) {
			if pe, ok := ev.(PluginEvent); ok {
				listener.HandleEvent(pe)
			}
		}, "plugin")
		return
	}

	// Fallback: directly subscribe to all matching event types when listener management is unavailable
	// If filter specifies types, subscribe individually; otherwise rely on upper layer to maintain type sets
	if filter != nil && len(filter.Types) > 0 {
		for _, t := range filter.Types {
			_ = adapter.SubscribeTo(t, func(pe PluginEvent) {
				// Basic filtering only (type already constrained by SubscribeTo)
				listener.HandleEvent(pe)
			})
		}
	} else {
		// Without type constraints and unknown event types, bus-level subscription is not possible here
		// Depend on adapter's AddListener capability
	}
}

// RemoveListener removes an event listener
func (r *UnifiedRuntime) RemoveListener(listener EventListener) {
	// Delegate to the unified event bus
	if listener == nil {
		return
	}
	id := listener.GetListenerID()
	if id == "" {
		return
	}
	adapter := EnsureGlobalEventBusAdapter()
	type removeListenerIface interface {
		RemoveListener(id string) error
	}
	if rl, ok := adapter.(removeListenerIface); ok {
		_ = rl.RemoveListener(id)
	}
}

// AddPluginListener adds a plugin-specific event listener
func (r *UnifiedRuntime) AddPluginListener(pluginName string, listener EventListener, filter *EventFilter) {
	// Delegate to the unified event bus
	if listener == nil {
		return
	}
	adapter := EnsureGlobalEventBusAdapter()
	id := listener.GetListenerID()
	if id == "" {
		id = fmt.Sprintf("plugin-listener-%s-%d", pluginName, time.Now().UnixNano())
	}
	type addPluginListenerIface interface {
		AddPluginListener(pluginName string, id string, filter *EventFilter, handler func(interface{})) error
	}
	if apl, ok := adapter.(addPluginListenerIface); ok {
		_ = apl.AddPluginListener(pluginName, id, filter, func(ev interface{}) {
			if pe, ok := ev.(PluginEvent); ok {
				listener.HandleEvent(pe)
			}
		})
		return
	}

	// Fallback: subscribe by event type and filter by PluginID in callback
	if filter != nil && len(filter.Types) > 0 {
		for _, t := range filter.Types {
			_ = adapter.SubscribeTo(t, func(pe PluginEvent) {
				if pe.PluginID == pluginName {
					listener.HandleEvent(pe)
				}
			})
		}
	}
}

// GetEventHistory returns event history
func (r *UnifiedRuntime) GetEventHistory(filter EventFilter) []PluginEvent {
	// Delegate to the unified event bus
	adapter := EnsureGlobalEventBusAdapter()
	type historyIface interface {
		GetEventHistory(filter *EventFilter) []PluginEvent
	}
	if hi, ok := adapter.(historyIface); ok {
		return hi.GetEventHistory(&filter)
	}
	return nil
}

// GetPluginEventHistory returns plugin event history
func (r *UnifiedRuntime) GetPluginEventHistory(pluginName string, filter EventFilter) []PluginEvent {
	// Delegate to the unified event bus
	adapter := EnsureGlobalEventBusAdapter()
	type pluginHistoryIface interface {
		GetPluginEventHistory(pluginName string, filter *EventFilter) []PluginEvent
	}
	if phi, ok := adapter.(pluginHistoryIface); ok {
		return phi.GetPluginEventHistory(pluginName, &filter)
	}
	return nil
}

// ============================================================================
// Performance configuration interfaces
// ============================================================================

// SetEventDispatchMode sets event dispatch mode
func (r *UnifiedRuntime) SetEventDispatchMode(mode string) error {
	// Delegate to the unified event bus
	adapter := EnsureGlobalEventBusAdapter()
	if configurable, ok := adapter.(interface{ SetDispatchMode(string) error }); ok {
		return configurable.SetDispatchMode(mode)
	}
	return nil
}

// SetEventWorkerPoolSize sets event worker pool size
func (r *UnifiedRuntime) SetEventWorkerPoolSize(size int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if size > 0 {
		r.workerPoolSize = size
	}
}

// SetEventTimeout sets event timeout duration
func (r *UnifiedRuntime) SetEventTimeout(timeout time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if timeout > 0 {
		r.eventTimeout = timeout
	}
}

// GetEventStats returns event stats
func (r *UnifiedRuntime) GetEventStats() map[string]any {
	return map[string]any{
		"worker_pool_size": r.workerPoolSize,
		"event_timeout":    r.eventTimeout.String(),
		"runtime_closed":   r.isClosed(),
	}
}

// ============================================================================
// Resource info and statistics
// ============================================================================

// GetResourceInfo returns resource info
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

// ListResources lists all resources
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

// CleanupResources cleans up resources for a plugin
func (r *UnifiedRuntime) CleanupResources(pluginID string) error {
	if pluginID == "" {
		return fmt.Errorf("plugin ID cannot be empty")
	}
	
	var toDelete []string
	
	// Collect resources to be deleted
	r.resourceInfo.Range(func(key, value interface{}) bool {
		if info, ok := value.(*ResourceInfo); ok {
			if info.PluginID == pluginID {
				toDelete = append(toDelete, key.(string))
			}
		}
		return true
	})
	
	// Delete resources
	for _, name := range toDelete {
		r.resources.Delete(name)
		r.resourceInfo.Delete(name)
	}
	
	return nil
}

// GetResourceStats returns resource statistics
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
// Lifecycle management
// ============================================================================

// Shutdown closes the Runtime
func (r *UnifiedRuntime) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.closed {
		return
	}
	
	// Close event bus
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

// Close closes the Runtime (compatibility API)
func (r *UnifiedRuntime) Close() {
	r.Shutdown()
}

// ============================================================================
// Internal helper methods
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
	
	// Simplified size estimation
	val := reflect.ValueOf(resource)
	return r.estimateValueSize(val, 0, 3)
}

func (r *UnifiedRuntime) estimateValueSize(val reflect.Value, depth, maxDepth int) int64 {
	if depth > maxDepth {
		return 8 // default pointer size
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
		for i := 0; i < val.Len() && i < 100; i++ { // limit inspected elements
			size += r.estimateValueSize(val.Index(i), depth+1, maxDepth)
		}
		return size
	case reflect.Map:
		size := int64(0)
		count := 0
		for _, key := range val.MapKeys() {
			if count >= 100 { // limit inspected entries
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
		return 8 // default size
	}
}

// ============================================================================
// Backward-compatible constructors
// ============================================================================

// Note: NewSimpleRuntime and NewTypedRuntime are defined in plugin.go
// They are not redefined here to avoid conflicts
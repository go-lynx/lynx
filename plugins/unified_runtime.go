package plugins

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
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
	contextMu            sync.RWMutex

	// Event system - uses a unified event bus
	eventManager interface{} // avoid circular dependency; set at runtime

	// Runtime state
	closed bool
	mu     sync.RWMutex

	// Context for graceful shutdown of background goroutines
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
}

// NewUnifiedRuntime creates a new unified Runtime instance
func NewUnifiedRuntime() *UnifiedRuntime {
	ctx, cancel := context.WithCancel(context.Background())
	return &UnifiedRuntime{
		resources:      &sync.Map{},
		resourceInfo:   &sync.Map{},
		logger:         log.DefaultLogger,
		closed:         false,
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
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
	r.updateAccessStats(name)

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

	// Create resource info with size estimation
	now := time.Now()
	info := &ResourceInfo{
		Name:        name,
		Type:        reflect.TypeOf(resource).String(),
		PluginID:    r.getCurrentPluginContext(),
		IsPrivate:   false,
		CreatedAt:   now,
		LastUsedAt:  now,
		AccessCount: 0,
		Size:        0, // Will be calculated asynchronously
		Metadata:    make(map[string]any),
	}

	r.resourceInfo.Store(name, info)

	// Asynchronously estimate resource size to avoid blocking registration
	// Use shutdown context to allow graceful cancellation
	go func() {
		select {
		case <-r.shutdownCtx.Done():
			// Runtime is shutting down, skip size estimation
			return
		default:
			size := r.estimateResourceSize(resource)
			// Check if runtime is still active before updating
			select {
			case <-r.shutdownCtx.Done():
				// Runtime closed during estimation, skip update
				return
			default:
				if value, ok := r.resourceInfo.Load(name); ok {
					if existingInfo, ok := value.(*ResourceInfo); ok {
						// Use atomic store for thread-safe update
						atomic.StoreInt64(&existingInfo.Size, size)
					}
				}
			}
		}
	}()

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

	// Create resource info with size estimation
	now := time.Now()
	info := &ResourceInfo{
		Name:        privateKey,
		Type:        reflect.TypeOf(resource).String(),
		PluginID:    pluginID,
		IsPrivate:   true,
		CreatedAt:   now,
		LastUsedAt:  now,
		AccessCount: 0,
		Size:        0, // Will be calculated asynchronously
		Metadata:    make(map[string]any),
	}

	r.resourceInfo.Store(privateKey, info)

	// Asynchronously estimate resource size to avoid blocking registration
	// Use shutdown context to allow graceful cancellation
	go func() {
		select {
		case <-r.shutdownCtx.Done():
			// Runtime is shutting down, skip size estimation
			return
		default:
			size := r.estimateResourceSize(resource)
			// Check if runtime is still active before updating
			select {
			case <-r.shutdownCtx.Done():
				// Runtime closed during estimation, skip update
				return
			default:
				if value, ok := r.resourceInfo.Load(privateKey); ok {
					if existingInfo, ok := value.(*ResourceInfo); ok {
						// Use atomic store for thread-safe update
						atomic.StoreInt64(&existingInfo.Size, size)
					}
				}
			}
		}
	}()

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
// Implements context forging prevention similar to simpleRuntime
func (r *UnifiedRuntime) WithPluginContext(pluginName string) Runtime {
	// Context forging prevention rules (same as simpleRuntime):
	// 1) If current context is empty and pluginName is non-empty: allow one-time set.
	// 2) If current context equals pluginName or pluginName is empty: no-op.
	// 3) Otherwise: deny switching to another plugin, return current runtime.
	r.contextMu.RLock()
	cur := r.currentPluginContext
	r.contextMu.RUnlock()

	// case 2: If current context equals pluginName or pluginName is empty: no-op
	if pluginName == "" || pluginName == cur {
		return r
	}

	// case 1: If current context is empty and pluginName is non-empty: allow one-time set
	if cur == "" && pluginName != "" {
		// Create a new Runtime instance sharing underlying resource maps
		// Note: We share the same shutdown context to ensure coordinated shutdown
		contextRuntime := &UnifiedRuntime{
			resources:            r.resources,    // share the same resource map pointer
			resourceInfo:         r.resourceInfo, // share the same resource info map pointer
			config:               r.config,
			logger:               r.logger,
			currentPluginContext: pluginName,
			contextMu:            sync.RWMutex{}, // new mutex for this instance's context
			eventManager:         r.eventManager,
			closed:               false,
			mu:                   sync.RWMutex{}, // new mutex for this instance's state
			shutdownCtx:           r.shutdownCtx, // share shutdown context
			shutdownCancel:        nil,           // only root instance has cancel function
		}
		return contextRuntime
	}

	// case 3: Deny switching to another plugin
	if logger := r.GetLogger(); logger != nil {
		logger.Log(log.LevelWarn, "msg", "denied WithPluginContext switch",
			"from", cur, "to", pluginName)
	}
	return r
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
// Performance configuration interfaces (delegated to event bus)
// ============================================================================

// SetEventDispatchMode sets event dispatch mode (delegates to event bus)
func (r *UnifiedRuntime) SetEventDispatchMode(mode string) error {
	adapter := EnsureGlobalEventBusAdapter()
	if configurable, ok := adapter.(interface{ SetDispatchMode(string) error }); ok {
		return configurable.SetDispatchMode(mode)
	}
	return nil
}

// SetEventWorkerPoolSize sets event worker pool size (delegates to event bus)
func (r *UnifiedRuntime) SetEventWorkerPoolSize(size int) {
	adapter := EnsureGlobalEventBusAdapter()
	if configurable, ok := adapter.(interface{ SetWorkerPoolSize(int) }); ok {
		configurable.SetWorkerPoolSize(size)
	}
}

// SetEventTimeout sets event timeout (delegates to event bus)
func (r *UnifiedRuntime) SetEventTimeout(timeout time.Duration) {
	adapter := EnsureGlobalEventBusAdapter()
	if configurable, ok := adapter.(interface{ SetEventTimeout(time.Duration) }); ok {
		configurable.SetEventTimeout(timeout)
	}
}

// GetEventStats returns event stats (delegates to event bus)
func (r *UnifiedRuntime) GetEventStats() map[string]any {
	stats := map[string]any{
		"runtime_closed": r.isClosed(),
	}

	// Get stats from event bus adapter
	adapter := GetGlobalEventBusAdapter()
	if statsProvider, ok := adapter.(interface{ GetStats() map[string]any }); ok {
		busStats := statsProvider.GetStats()
		for k, v := range busStats {
			stats[k] = v
		}
	}

	return stats
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

	// Collect resources to be deleted with their actual resource objects
	type resItem struct {
		name string
		res  any
	}
	var toDelete []resItem

	// Phase 1: collect resources under lock
	r.resourceInfo.Range(func(key, value interface{}) bool {
		if info, ok := value.(*ResourceInfo); ok {
			if info.PluginID == pluginID {
				name := key.(string)
				if resource, exists := r.resources.Load(name); exists {
					toDelete = append(toDelete, resItem{name: name, res: resource})
				}
			}
		}
		return true
	})

	// Phase 2: cleanup resources outside lock to avoid holding lock during cleanup
	var errors []error
	var cleanedCount int

	for _, item := range toDelete {
		// Cleanup resource gracefully
		if err := r.cleanupResourceGracefully(item.name, item.res); err != nil {
			errors = append(errors, fmt.Errorf("failed to cleanup resource %s: %w", item.name, err))
		} else {
			cleanedCount++
		}

		// Remove from maps after cleanup attempt
		r.resources.Delete(item.name)
		r.resourceInfo.Delete(item.name)
	}

	// Log cleanup summary
	if len(toDelete) > 0 {
		if logger := r.GetLogger(); logger != nil {
			logger.Log(log.LevelInfo, "msg", "cleaned up resources for plugin",
				"plugin_id", pluginID,
				"total", len(toDelete),
				"cleaned", cleanedCount,
				"errors", len(errors))
		}
	}

	// Return combined error if any cleanup failed
	if len(errors) > 0 {
		return fmt.Errorf("resource cleanup had %d errors: %v", len(errors), errors[0])
	}

	return nil
}

// cleanupResourceGracefully attempts to gracefully cleanup a resource
// This method is similar to simpleRuntime.cleanupResourceGracefully but adapted for UnifiedRuntime
func (r *UnifiedRuntime) cleanupResourceGracefully(name string, resource any) error {
	if resource == nil {
		return nil
	}

	// Guard against panics during cleanup
	defer func() {
		if rec := recover(); rec != nil {
			if logger := r.GetLogger(); logger != nil {
				logger.Log(log.LevelWarn, "msg", "panic during resource cleanup", "resource", name, "panic", rec)
			}
		}
	}()

	// Record cleanup start
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		if duration > 5*time.Second {
			if logger := r.GetLogger(); logger != nil {
				logger.Log(log.LevelWarn, "msg", "slow resource cleanup", "resource", name, "duration", duration)
			}
		}
	}()

	// Prefer context-aware graceful shutdown with configurable timeout
	// Default to 3 seconds, but allow configuration for resources that need more time
	cleanupTimeout := r.getResourceCleanupTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	// Context-aware interfaces first (graceful)
	switch v := resource.(type) {
	case interface{ ShutdownContext(context.Context) error }:
		if err := v.ShutdownContext(ctx); err != nil {
			return fmt.Errorf("shutdown (ctx) failed: %w", err)
		}
		return nil
	case interface{ StopContext(context.Context) error }:
		if err := v.StopContext(ctx); err != nil {
			return fmt.Errorf("stop (ctx) failed: %w", err)
		}
		return nil
	case interface{ CloseContext(context.Context) error }:
		if err := v.CloseContext(ctx); err != nil {
			return fmt.Errorf("close (ctx) failed: %w", err)
		}
		return nil
	case interface{ CleanupContext(context.Context) error }:
		if err := v.CleanupContext(ctx); err != nil {
			return fmt.Errorf("cleanup (ctx) failed: %w", err)
		}
		return nil
	case interface{ DestroyContext(context.Context) error }:
		if err := v.DestroyContext(ctx); err != nil {
			return fmt.Errorf("destroy (ctx) failed: %w", err)
		}
		return nil
	}

	// Then non-context graceful methods (ordered by gracefulness)
	switch v := resource.(type) {
	case interface{ Shutdown() error }:
		return v.Shutdown()
	case interface{ Stop() error }:
		return v.Stop()
	case interface{ Cleanup() error }:
		return v.Cleanup()
	case interface{ Destroy() error }:
		return v.Destroy()
	case interface{ Release() error }:
		return v.Release()
	case interface{ Close() error }:
		return v.Close()
	case context.CancelFunc:
		v()
		return nil
	case func():
		v()
		return nil
	}

	// For channels, attempt to close them safely (only non-recv-only)
	if val := reflect.ValueOf(resource); val.Kind() == reflect.Chan {
		if val.Type().ChanDir() != reflect.RecvDir {
			defer func() {
				if r := recover(); r != nil {
					// Channel already closed or other panic
				}
			}()
			val.Close()
			return nil
		}
	}

	// No cleanup method found - this is not an error, just log it
	return nil
}

// GetResourceStats returns resource statistics including size and plugin information
func (r *UnifiedRuntime) GetResourceStats() map[string]any {
	var totalResources, privateResources, sharedResources int
	var totalSize int64
	pluginSet := make(map[string]bool)

	r.resourceInfo.Range(func(key, value interface{}) bool {
		if info, ok := value.(*ResourceInfo); ok {
			totalResources++
			// Use atomic load for Size to ensure thread-safe reading
			size := atomic.LoadInt64(&info.Size)
			totalSize += size
			pluginSet[info.PluginID] = true
			if info.IsPrivate {
				privateResources++
			} else {
				sharedResources++
			}
		}
		return true
	})

	return map[string]any{
		"total_resources":        totalResources,
		"private_resources":      privateResources,
		"shared_resources":       sharedResources,
		"total_size_bytes":       totalSize,
		"plugins_with_resources": len(pluginSet),
		"runtime_closed":         r.isClosed(),
	}
}

// getResourceCleanupTimeout returns the timeout for resource cleanup, default 3s.
// Can be configured via "lynx.runtime.resource_cleanup_timeout" config key.
func (r *UnifiedRuntime) getResourceCleanupTimeout() time.Duration {
	defaultTimeout := 3 * time.Second
	if r.config == nil {
		return defaultTimeout
	}

	var confStr string
	if err := r.config.Value("lynx.runtime.resource_cleanup_timeout").Scan(&confStr); err == nil {
		if parsed, err2 := time.ParseDuration(confStr); err2 == nil {
			// Validate timeout range: 1s to 30s
			if parsed < 1*time.Second {
				if r.logger != nil {
					r.logger.Log(log.LevelWarn, "msg", "resource_cleanup_timeout too short, using minimum 1s", "configured", parsed)
				}
				return 1 * time.Second
			}
			if parsed > 30*time.Second {
				if r.logger != nil {
					r.logger.Log(log.LevelWarn, "msg", "resource_cleanup_timeout too long, using maximum 30s", "configured", parsed)
				}
				return 30 * time.Second
			}
			return parsed
		}
	}
	return defaultTimeout
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

	// Cancel shutdown context to signal all background goroutines to stop
	if r.shutdownCancel != nil {
		r.shutdownCancel()
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

// updateAccessStats updates access statistics for a resource
// Uses atomic operations for thread-safe updates
// Note: sync.Map only protects the map itself, not the values stored in it.
// We use atomic operations for AccessCount to ensure thread-safety.
// For LastUsedAt, we update it directly - while not fully atomic, this is acceptable
// because: (1) time.Time assignment is generally safe on 64-bit architectures,
// (2) worst case is slightly inaccurate timestamp (eventual consistency), and
// (3) adding a mutex would impact performance for a non-critical metric.
func (r *UnifiedRuntime) updateAccessStats(name string) {
	if value, ok := r.resourceInfo.Load(name); ok {
		if info, ok := value.(*ResourceInfo); ok {
			// Use atomic operation for AccessCount to ensure thread-safety
			atomic.AddInt64(&info.AccessCount, 1)
			// Update LastUsedAt directly - acceptable for eventual consistency
			// In k8s environments where resources can scale, exact timestamps are less critical
			info.LastUsedAt = time.Now()
		}
	}
}

// estimateResourceSize estimates the size of a resource
// This method is similar to simpleRuntime.estimateResourceSize
func (r *UnifiedRuntime) estimateResourceSize(resource any) int64 {
	if resource == nil {
		return 0
	}

	// Use reflection to estimate size with depth limit
	val := reflect.ValueOf(resource)
	visited := make(map[uintptr]bool)
	return r.estimateValueSizeWithDepth(val, 0, 20, visited) // Max depth 20
}

// estimateValueSizeWithDepth recursively estimates value size with protection
func (r *UnifiedRuntime) estimateValueSizeWithDepth(val reflect.Value, depth, maxDepth int, visited map[uintptr]bool) int64 {
	if !val.IsValid() || depth > maxDepth {
		return 0
	}

	// Prevent infinite recursion for circular references
	if val.Kind() == reflect.Ptr && !val.IsNil() {
		ptr := val.Pointer()
		if visited[ptr] {
			return 8 // Just the pointer size
		}
		visited[ptr] = true
		defer func() { delete(visited, ptr) }()
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
		length := val.Len()
		// Limit the number of elements we examine to prevent excessive computation
		maxElements := 1000
		if length > maxElements {
			// Sample first few elements and estimate
			sampleSize := int64(0)
			for i := 0; i < maxElements && i < length; i++ {
				sampleSize += r.estimateValueSizeWithDepth(val.Index(i), depth+1, maxDepth, visited)
			}
			return (sampleSize * int64(length)) / int64(maxElements)
		}
		for i := 0; i < length; i++ {
			size += r.estimateValueSizeWithDepth(val.Index(i), depth+1, maxDepth, visited)
		}
		return size
	case reflect.Map:
		size := int64(0)
		keys := val.MapKeys()
		// Limit the number of map entries we examine
		maxKeys := 1000
		if len(keys) > maxKeys {
			// Sample first few keys and estimate
			sampleSize := int64(0)
			for i := 0; i < maxKeys; i++ {
				key := keys[i]
				sampleSize += r.estimateValueSizeWithDepth(key, depth+1, maxDepth, visited)
				sampleSize += r.estimateValueSizeWithDepth(val.MapIndex(key), depth+1, maxDepth, visited)
			}
			return (sampleSize * int64(len(keys))) / int64(maxKeys)
		}
		for _, key := range keys {
			size += r.estimateValueSizeWithDepth(key, depth+1, maxDepth, visited)
			size += r.estimateValueSizeWithDepth(val.MapIndex(key), depth+1, maxDepth, visited)
		}
		return size
	case reflect.Struct:
		size := int64(0)
		numField := val.NumField()
		for i := 0; i < numField; i++ {
			field := val.Field(i)
			if field.CanInterface() { // Skip unexported fields
				size += r.estimateValueSizeWithDepth(field, depth+1, maxDepth, visited)
			}
		}
		return size
	case reflect.Ptr:
		if val.IsNil() {
			return 8 // Size of pointer itself
		}
		return 8 + r.estimateValueSizeWithDepth(val.Elem(), depth+1, maxDepth, visited)
	case reflect.Interface:
		if val.IsNil() {
			return 8
		}
		return 8 + r.estimateValueSizeWithDepth(val.Elem(), depth+1, maxDepth, visited)
	default:
		return 8 // Default size
	}
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

// ============================================================================
// Backward-compatible constructors
// ============================================================================

// Note: NewSimpleRuntime and NewTypedRuntime are defined in plugin.go
// They are not redefined here to avoid conflicts

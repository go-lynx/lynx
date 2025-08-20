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

// Default event queue and worker parameters
const (
	defaultEventQueueSize   = 1024
	defaultEventWorkerCount = 10
	// Per-listener queue size to prevent a single slow listener from blocking the system
	defaultListenerQueueSize = 256
	// Default history size 1000; <=0 means no history kept
	defaultHistorySize = 1000
	// Default timeout (ms) to drain listener queues on shutdown. 500ms balances fast shutdown with minimal event loss
	defaultDrainTimeoutMs = 500
)

// TypedRuntimePlugin generic runtime plugin
type TypedRuntimePlugin struct {
	// resources stores shared resources between plugins
	resources sync.Map

	// eventListeners stores registered event listeners with their filters
	listeners []listenerEntry

	// eventHistory stores historical events for querying
	eventHistory []plugins.PluginEvent

	// maxHistorySize is the maximum number of events to keep in history
	maxHistorySize int

	// mu protects the listeners and eventHistory
	mu sync.RWMutex

	// logger is the plugin's logger instance
	logger log.Logger

	// config is the plugin's configuration
	config config.Config

	// Event queue and workers
	// eventCh is a buffered channel carrying events, providing backpressure
	eventCh chan plugins.PluginEvent
	// workerCount number of worker goroutines
	workerCount int
	// Independent queue size per listener
	listenerQueueSize int
	// Shutdown control
	closeOnce sync.Once
	closed    chan struct{}

	// Goroutine tracking
	workerWg   sync.WaitGroup
	listenerWg sync.WaitGroup

	// Stop flag; quickly drop events during Emit when stopped
	stopped int32

	// Mutex for producers to avoid concurrent sends when Close synchronously closes eventCh
	sendMu sync.Mutex

	// Timeout for draining listener queues
	drainTimeout time.Duration
}

// listenerEntry represents a registered event listener with its filter
type listenerEntry struct {
	listener plugins.EventListener
	filter   *plugins.EventFilter
	// Independent event queue per listener
	ch chan plugins.PluginEvent
	// Listener quit signal; do not close ch to avoid races with worker sends
	quit chan struct{}
	// Active flag (shared pointer to observe Remove state changes even after snapshot). 1=active, 0=inactive
	active *int32
}

// NewTypedRuntimePlugin creates a new TypedRuntimePlugin instance with default settings.
func NewTypedRuntimePlugin() *TypedRuntimePlugin {
	// Read configurable queue sizes and worker counts (from bootstrap config only)
	qsize := defaultEventQueueSize
	wcount := defaultEventWorkerCount
	lqsize := defaultListenerQueueSize
	hsize := defaultHistorySize
	drainMs := defaultDrainTimeoutMs
	if app := Lynx(); app != nil {
		// Use bootstrap config (boot.pb.go) only; fall back to defaults if not set
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
			// History size (<=0 means disabled)
			if v := app.bootConfig.Lynx.Runtime.Event.HistorySize; v != 0 {
				hsize = int(v)
			}
			// Drain timeout on close (milliseconds)
			if v := app.bootConfig.Lynx.Runtime.Event.DrainTimeoutMs; v > 0 {
				drainMs = int(v)
			}
		}
	}

	r := &TypedRuntimePlugin{
		maxHistorySize:    hsize, // Number of historical events to retain (<=0 disables history)
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
	// Start a fixed number of event dispatch workers
	for i := 0; i < r.workerCount; i++ {
		r.workerWg.Add(1)
		go r.eventWorkerLoop()
	}
	return r
}

// GetResource retrieves a shared plugin resource by name.
// Returns the resource and any error encountered.
func (r *TypedRuntimePlugin) GetResource(name string) (any, error) {
	if value, ok := r.resources.Load(name); ok {
		return value, nil
	}
	return nil, fmt.Errorf("resource not found: %s", name)
}

// RegisterResource registers a resource to be shared with other plugins.
// Returns an error if registration fails.
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

// GetTypedResource retrieves a type-safe resource (standalone helper)
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

// RegisterTypedResource registers a type-safe resource (standalone helper)
func RegisterTypedResource[T any](r *TypedRuntimePlugin, name string, resource T) error {
	return r.RegisterResource(name, resource)
}

// GetConfig returns the plugin configuration manager.
// Provides access to configuration values and updates.
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

// GetLogger returns the plugin logger instance.
// Provides structured logging capabilities.
func (r *TypedRuntimePlugin) GetLogger() log.Logger {
	if r.logger == nil {
		// Initialize with a default logger if not set
		r.logger = log.DefaultLogger
	}
	return r.logger
}

// EmitEvent broadcasts a plugin event to all registered listeners.
// Event will be processed according to its priority and any active filters.
func (r *TypedRuntimePlugin) EmitEvent(event plugins.PluginEvent) {
	if event.Type == "" { // Check for zero value of EventType
		return
	}

	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}

	// If already stopped, drop immediately
	if atomic.LoadInt32(&r.stopped) == 1 {
		return
	}

	// Write to history first (if enabled), then enqueue
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

	// Enqueue the event; if the queue is full, drop and log at debug level to avoid goroutine explosion
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

// eventWorkerLoop processes events and dispatches them to listeners matching the filter
func (r *TypedRuntimePlugin) eventWorkerLoop() {
	defer r.workerWg.Done()
	for ev := range r.eventCh {
		// Copy a snapshot of listeners to avoid holding the lock for long
		r.mu.RLock()
		listeners := make([]listenerEntry, len(r.listeners))
		copy(listeners, r.listeners)
		r.mu.RUnlock()

		for _, entry := range listeners {
			// Skip if the listener has been marked inactive
			if entry.active != nil && atomic.LoadInt32(entry.active) == 0 {
				continue
			}
			if entry.filter == nil || r.eventMatchesFilter(ev, *entry.filter) {
				// Dispatch to the listener's independent queue to avoid head-of-line blocking
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

// Close stops event dispatching (optional to call)
func (r *TypedRuntimePlugin) Close() {
	r.closeOnce.Do(func() {
		// Phase 1: stop receiving new events and let workers exit naturally
		atomic.StoreInt32(&r.stopped, 1)
		// Mutex with producers to avoid "send on closed channel"
		r.sendMu.Lock()
		close(r.eventCh)
		r.sendMu.Unlock()
		r.workerWg.Wait()

		// Phase 2.1: optionally drain listener queues (wait only; no further production)
		if r.drainTimeout > 0 {
			deadline := time.Now().Add(r.drainTimeout)
			for {
				allEmpty := true
				r.mu.RLock()
				for _, entry := range r.listeners {
					if len(entry.ch) > 0 {
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

		// Phase 2.2: notify listeners to quit and wait for graceful exit
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

		// Finally mark closed (can be used in other select statements)
		close(r.closed)
	})
}

// AddListener registers a new event listener with optional filters.
// Listener will only receive events that match its filter criteria.
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

	// Start independent goroutine for this listener
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
			// Notify the listener to quit, but do not close its event channel to avoid worker sends to a closed channel
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
func (r *TypedRuntimePlugin) eventMatchesFilter(event plugins.PluginEvent, filter plugins.EventFilter) bool {
	// Check event type
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
	if filter.FromTime > 0 && event.Timestamp < filter.FromTime {
		return false
	}
	if filter.ToTime > 0 && event.Timestamp > filter.ToTime {
		return false
	}

	return true
}

// RuntimePlugin backward-compatible alias of TypedRuntimePlugin
type RuntimePlugin = TypedRuntimePlugin

// NewRuntimePlugin creates a runtime plugin (backward-compatible)
func NewRuntimePlugin() *RuntimePlugin {
	return NewTypedRuntimePlugin()
}

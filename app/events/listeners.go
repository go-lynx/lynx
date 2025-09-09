package events

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
)

// EventListener represents an event listener
type EventListener struct {
	ID      string
	Filter  *EventFilter
	Handler func(LynxEvent)
	BusType BusType
	Active  atomic.Bool
	Cancel  context.CancelFunc
}

// EventListenerManager manages event listeners
type EventListenerManager struct {
	listeners map[string]*EventListener
	mu        sync.RWMutex
}

// NewEventListenerManager creates a new event listener manager
func NewEventListenerManager() *EventListenerManager {
	return &EventListenerManager{
		listeners: make(map[string]*EventListener),
	}
}

// AddListenerWithContext adds a new event listener bound to a context.
// When ctx is canceled, the listener is automatically removed to avoid leaks.
func (m *EventListenerManager) AddListenerWithContext(ctx context.Context, id string, filter *EventFilter, handler func(LynxEvent), busType BusType) error {
	if ctx == nil {
		// Fallback to normal AddListener if ctx is nil
		return m.AddListener(id, filter, handler, busType)
	}
	if err := m.AddListener(id, filter, handler, busType); err != nil {
		return err
	}
	// Detach goroutine to observe ctx.Done() and cleanup listener
	go func() {
		<-ctx.Done()
		_ = m.RemoveListener(id)
	}()
	return nil
}

// AddListener adds a new event listener
func (m *EventListenerManager) AddListener(id string, filter *EventFilter, handler func(LynxEvent), busType BusType) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.listeners[id]; exists {
		return fmt.Errorf("listener with ID %s already exists", id)
	}

	// Deep-copy filter to avoid external mutation races
	var cloned *EventFilter
	if filter != nil {
		cloned = filter.Clone()
	}
	listener := &EventListener{
		ID:      id,
		Filter:  cloned,
		Handler: handler,
		BusType: busType,
		Active:  atomic.Bool{},
		Cancel:  nil,
	}
	listener.Active.Store(true)

	m.listeners[id] = listener

	// Subscribe to the event bus
	eventManager := GetGlobalEventBus()
	if eventManager == nil {
		delete(m.listeners, id)
		return fmt.Errorf("global event bus not initialized")
	}

	// If filter is empty, subscribe to all events on the bus
	if filter == nil || filter.IsEmpty() {
		cancel, err := eventManager.SubscribeWithCancel(busType, func(event LynxEvent) {
			if listener.Active.Load() {
				listener.Handler(event)
			}
		})
		if err != nil {
			delete(m.listeners, id)
			return fmt.Errorf("failed to subscribe to bus: %w", err)
		}
		listener.Cancel = cancel
	} else {
		// Use cloned filter for thread safety
		lf := listener.Filter
		// If filter has no EventTypes specified, subscribe at bus-level with predicate
		if len(lf.EventTypes) == 0 {
			cancel, err := eventManager.SubscribeWithCancel(busType, func(event LynxEvent) {
				if listener.Active.Load() && lf.Matches(event) {
					listener.Handler(event)
				}
			})
			if err != nil {
				delete(m.listeners, id)
				return fmt.Errorf("failed to subscribe to bus with filter: %w", err)
			}
			listener.Cancel = cancel
		} else {
			// Subscribe to specific event types
			var cancels []context.CancelFunc
			for _, eventType := range lf.EventTypes {
				cancel, err := eventManager.SubscribeToWithCancel(eventType, func(event LynxEvent) {
					if listener.Active.Load() && lf.Matches(event) {
						listener.Handler(event)
					}
				})
				if err != nil {
					// cancel already registered ones
					for _, c := range cancels {
						c()
					}
					delete(m.listeners, id)
					return fmt.Errorf("failed to subscribe to event type %d: %w", eventType, err)
				}
				cancels = append(cancels, cancel)
			}
			// compose cancel
			listener.Cancel = func() {
				for _, c := range cancels {
					if c != nil {
						c()
					}
				}
			}
		}
	}

	return nil
}

// RemoveListener removes an event listener
func (m *EventListenerManager) RemoveListener(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	listener, exists := m.listeners[id]
	if !exists {
		return fmt.Errorf("listener with ID %s not found", id)
	}

	// Cancel the listener
	listener.Active.Store(false)
	if listener.Cancel != nil {
		listener.Cancel()
	}

	delete(m.listeners, id)
	return nil
}

// GetListener returns a listener by ID
func (m *EventListenerManager) GetListener(id string) (*EventListener, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	listener, exists := m.listeners[id]
	if !exists {
		return nil, fmt.Errorf("listener with ID %s not found", id)
	}

	return listener, nil
}

// ListListeners returns all listeners
func (m *EventListenerManager) ListListeners() []*EventListener {
	m.mu.RLock()
	defer m.mu.RUnlock()

	listeners := make([]*EventListener, 0, len(m.listeners))
	for _, listener := range m.listeners {
		listeners = append(listeners, listener)
	}

	return listeners
}

// GetListenersByBusType returns listeners for a specific bus type
func (m *EventListenerManager) GetListenersByBusType(busType BusType) []*EventListener {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var listeners []*EventListener
	for _, listener := range m.listeners {
		if listener.BusType == busType {
			listeners = append(listeners, listener)
		}
	}

	return listeners
}

// GetListenersByFilter returns listeners that match a filter
func (m *EventListenerManager) GetListenersByFilter(filter *EventFilter) []*EventListener {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var listeners []*EventListener
	for _, listener := range m.listeners {
		if filtersEqual(listener.Filter, filter) {
			listeners = append(listeners, listener)
		}
	}

	return listeners
}

// Count returns the number of listeners
func (m *EventListenerManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.listeners)
}

// Clear removes all listeners
func (m *EventListenerManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, listener := range m.listeners {
		if listener.Cancel != nil {
			listener.Cancel()
		}
	}

	m.listeners = make(map[string]*EventListener)
}

// Global listener manager instance
var (
	globalListenerManager *EventListenerManager
	globalListenerOnce    sync.Once
)

// GetGlobalListenerManager returns the global listener manager
func GetGlobalListenerManager() *EventListenerManager {
	globalListenerOnce.Do(func() {
		globalListenerManager = NewEventListenerManager()
	})
	return globalListenerManager
}

// AddGlobalListener adds a listener to the global listener manager
func AddGlobalListener(id string, filter *EventFilter, handler func(LynxEvent), busType BusType) error {
	return GetGlobalListenerManager().AddListener(id, filter, handler, busType)
}

// AddGlobalListenerWithContext adds a listener to the global listener manager and auto-removes on ctx cancellation.
func AddGlobalListenerWithContext(ctx context.Context, id string, filter *EventFilter, handler func(LynxEvent), busType BusType) error {
	return GetGlobalListenerManager().AddListenerWithContext(ctx, id, filter, handler, busType)
}

// RemoveGlobalListener removes a listener from the global listener manager
func RemoveGlobalListener(id string) error {
	return GetGlobalListenerManager().RemoveListener(id)
}

// ListGlobalListeners returns all global listeners
func ListGlobalListeners() []*EventListener {
	return GetGlobalListenerManager().ListListeners()
}

// filtersEqual compares two EventFilter pointers structurally. Nil equals nil.
func filtersEqual(a, b *EventFilter) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if !eventTypeSliceEqual(a.EventTypes, b.EventTypes) {
		return false
	}
	if !prioritySliceEqual(a.Priorities, b.Priorities) {
		return false
	}
	if !stringSliceEqual(a.Sources, b.Sources) {
		return false
	}
	if !stringSliceEqual(a.Categories, b.Categories) {
		return false
	}
	if !stringSliceEqual(a.PluginIDs, b.PluginIDs) {
		return false
	}
	if a.FromTime != b.FromTime || a.ToTime != b.ToTime || a.HasError != b.HasError {
		return false
	}
	if !stringSliceEqual(a.Statuses, b.Statuses) {
		return false
	}
	if len(a.Metadata) != len(b.Metadata) {
		return false
	}
	for k, va := range a.Metadata {
		if vb, ok := b.Metadata[k]; !ok || !reflect.DeepEqual(vb, va) {
			return false
		}
	}
	return true
}

func eventTypeSliceEqual(a, b []EventType) bool { return genericSliceEqual(a, b) }
func prioritySliceEqual(a, b []Priority) bool   { return genericSliceEqual(a, b) }
func stringSliceEqual(a, b []string) bool       { return genericSliceEqual(a, b) }

// genericSliceEqual compares slices by content and order. Adjust to set-equality if needed.
func genericSliceEqual[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

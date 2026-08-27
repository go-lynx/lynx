package events

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// ErrManagerClosed is returned by manager operations after Close has been called.
var ErrManagerClosed = errors.New("event bus manager is closed")

// EventBusManager owns the per-type buses and routes events to them.
//
// Methods that act on a bus take a snapshot of the bus pointer under the
// manager's RLock and then release it before calling into the bus. Holding the
// manager lock across a bus call can deadlock, because bus callbacks (e.g.
// degradation publishing an event) re-enter the manager.
type EventBusManager struct {
	buses      map[BusType]*LynxEventBus
	classifier *EventClassifier
	configs    BusConfigs
	monitor    *EventMonitor
	logger     log.Logger
	mu         sync.RWMutex
	// closed is set (under mu) by Close. Once set, buses is nil and every
	// lookup fails fast with ErrManagerClosed.
	closed bool

	healthCheckMu      sync.Mutex
	healthCheckDone    chan struct{}
	healthCheckRunning bool
}

// SubscribeWithFilter subscribes on a bus with a predicate filter
func (manager *EventBusManager) SubscribeWithFilter(busType BusType, filter func(LynxEvent) bool, handler func(LynxEvent)) (context.CancelFunc, error) {
	bus := manager.GetBus(busType)
	if bus == nil {
		return func() {}, fmt.Errorf("no bus found for bus type: %d", busType)
	}
	cancel := bus.SubscribeWithFilter(filter, handler)
	return cancel, nil
}

// SubscribeToWithFilter subscribes to a specific event type with a predicate filter
func (manager *EventBusManager) SubscribeToWithFilter(eventType EventType, filter func(LynxEvent) bool, handler func(LynxEvent)) (context.CancelFunc, error) {
	dummyEvent := NewLynxEvent(eventType, "system", "event-bus-manager")
	busType := manager.classifier.GetBusType(dummyEvent)
	bus := manager.GetBus(busType)
	if bus == nil {
		return func() {}, fmt.Errorf("no bus found for event type: %d", eventType)
	}
	cancel := bus.SubscribeToWithFilter(eventType, filter, handler)
	return cancel, nil
}

// NewEventBusManager creates a new event bus manager
func NewEventBusManager(configs BusConfigs) (*EventBusManager, error) {
	if err := configs.Validate(); err != nil {
		return nil, fmt.Errorf("invalid event bus configuration: %w", err)
	}

	manager := &EventBusManager{
		buses:      make(map[BusType]*LynxEventBus),
		classifier: NewEventClassifier(),
		configs:    configs,
		monitor:    NewEventMonitor(),
	}

	manager.initBuses()

	return manager, nil
}

// initBuses constructs one bus per BusType using its configured settings.
func (manager *EventBusManager) initBuses() {
	busTypes := []BusType{
		BusTypePlugin,
		BusTypeSystem,
		BusTypeBusiness,
		BusTypeHealth,
		BusTypeConfig,
		BusTypeResource,
		BusTypeSecurity,
		BusTypeMetrics,
	}

	for _, busType := range busTypes {
		config := manager.configs.GetBusConfig(busType)
		bus := NewLynxEventBus(config, busType, manager)
		manager.buses[busType] = bus
	}
}

// GetMonitor returns the monitor bound to this manager.
func (manager *EventBusManager) GetMonitor() *EventMonitor {
	if manager == nil {
		return nil
	}
	return manager.monitor
}

// lookupBus returns the bus for busType, ErrManagerClosed after Close, or a
// not-found error. It never calls into the bus while holding manager.mu.
func (manager *EventBusManager) lookupBus(busType BusType) (*LynxEventBus, error) {
	manager.mu.RLock()
	closed := manager.closed
	bus, exists := manager.buses[busType]
	manager.mu.RUnlock()

	if closed {
		return nil, ErrManagerClosed
	}
	if !exists {
		return nil, fmt.Errorf("no bus found for bus type: %d", busType)
	}
	return bus, nil
}

// GetBus returns the bus for the given bus type, or nil if none is registered
// or the manager has been closed.
func (manager *EventBusManager) GetBus(busType BusType) *LynxEventBus {
	bus, err := manager.lookupBus(busType)
	if err != nil {
		return nil
	}
	return bus
}

// PublishEvent routes an event to the bus chosen by the classifier.
// It returns ErrManagerClosed once the manager has been closed.
func (manager *EventBusManager) PublishEvent(event LynxEvent) error {
	busType := manager.classifier.GetBusType(event)

	bus, err := manager.lookupBus(busType)
	if err != nil {
		return err
	}

	bus.Publish(event)
	return nil
}

// Subscribe registers a catch-all handler on the given bus.
func (manager *EventBusManager) Subscribe(busType BusType, handler func(LynxEvent)) error {
	bus, err := manager.lookupBus(busType)
	if err != nil {
		return err
	}

	bus.Subscribe(handler)
	return nil
}

// SubscribeTo registers a handler for one event type on whichever bus the
// classifier routes that type to.
func (manager *EventBusManager) SubscribeTo(eventType EventType, handler func(LynxEvent)) error {
	dummyEvent := NewLynxEvent(eventType, "system", "event-bus-manager")
	busType := manager.classifier.GetBusType(dummyEvent)

	bus, err := manager.lookupBus(busType)
	if err != nil {
		return err
	}

	bus.SubscribeTo(eventType, handler)
	return nil
}

// SubscribeWithCancel is Subscribe but returns a cancel func to unsubscribe.
func (manager *EventBusManager) SubscribeWithCancel(busType BusType, handler func(LynxEvent)) (context.CancelFunc, error) {
	bus, err := manager.lookupBus(busType)
	if err != nil {
		return func() {}, err
	}

	cancel := bus.Subscribe(handler)
	return cancel, nil
}

// SubscribeToWithCancel is SubscribeTo but returns a cancel func to unsubscribe.
func (manager *EventBusManager) SubscribeToWithCancel(eventType EventType, handler func(LynxEvent)) (context.CancelFunc, error) {
	dummyEvent := NewLynxEvent(eventType, "system", "event-bus-manager")
	busType := manager.classifier.GetBusType(dummyEvent)

	bus, err := manager.lookupBus(busType)
	if err != nil {
		return func() {}, err
	}

	cancel := bus.SubscribeTo(eventType, handler)
	return cancel, nil
}

// Close closes all buses. It is idempotent.
//
// The manager lock is held only long enough to mark the manager closed and
// detach the bus map; buses are closed afterwards, outside the lock and
// concurrently. bus.Close waits (up to CloseTimeout) for in-flight handlers,
// and those handlers may call back into PublishEvent/GetBus; if the lock were
// held across bus.Close such a handler would block on mu.RLock, the bus would
// wait on the handler, and shutdown would stall for CloseTimeout per bus.
func (manager *EventBusManager) Close() error {
	manager.StopHealthCheck()

	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return nil
	}
	manager.closed = true
	buses := manager.buses
	manager.buses = nil
	manager.mu.Unlock()

	type closeResult struct {
		busType BusType
		err     error
	}
	results := make(chan closeResult, len(buses))
	var wg sync.WaitGroup
	for busType, bus := range buses {
		wg.Add(1)
		go func(busType BusType, bus *LynxEventBus) {
			defer wg.Done()
			results <- closeResult{busType: busType, err: bus.Close()}
		}(busType, bus)
	}
	wg.Wait()
	close(results)

	var lastError error
	for r := range results {
		if r.err != nil {
			lastError = fmt.Errorf("failed to close bus %d: %w", r.busType, r.err)
		}
	}
	return lastError
}

// SetLogger sets the logger for all buses
func (manager *EventBusManager) SetLogger(logger log.Logger) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	manager.logger = logger
	for _, bus := range manager.buses {
		bus.SetLogger(logger)
	}
}

// GetClassifier returns the event classifier
func (manager *EventBusManager) GetClassifier() *EventClassifier {
	return manager.classifier
}

// GetConfigs returns the bus configurations
func (manager *EventBusManager) GetConfigs() BusConfigs {
	return manager.configs
}

// GetBusStatus returns the status of all buses
func (manager *EventBusManager) GetBusStatus() map[BusType]BusStatus {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	status := make(map[BusType]BusStatus)
	for busType, bus := range manager.buses {
		pauseDuration, _ := bus.GetPauseStats()
		cap, running, free, waiting := bus.GetWorkerPoolStats()
		status[busType] = BusStatus{
			BusType:             busType,
			IsHealthy:           bus.IsHealthy(),
			IsPaused:            bus.IsPaused(),
			IsDegraded:          bus.IsDegraded(),
			QueueSize:           bus.GetQueueSize(),
			Subscribers:         bus.GetTotalSubscriberCount(),
			PauseDuration:       pauseDuration,
			PauseCount:          bus.GetPauseCount(),
			DegradationDuration: bus.GetDegradationDuration(),
			WorkerCap:           cap,
			WorkerRunning:       running,
			WorkerFree:          free,
			WorkerWaiting:       waiting,
		}
	}

	return status
}

// BusStatus represents the status of a bus
type BusStatus struct {
	BusType             BusType
	IsHealthy           bool
	IsPaused            bool
	IsDegraded          bool
	QueueSize           int
	Subscribers         int
	PauseDuration       time.Duration
	PauseCount          int64
	DegradationDuration time.Duration
	// Worker pool stats (ants)
	WorkerCap     int
	WorkerRunning int
	WorkerFree    int
	WorkerWaiting int
}

// Pause stops a bus from consuming events; Publish keeps enqueuing them.
func (manager *EventBusManager) Pause(busType BusType) error {
	bus, err := manager.lookupBus(busType)
	if err != nil {
		return err
	}

	bus.Pause()
	return nil
}

// Resume restarts consumption on a paused bus.
func (manager *EventBusManager) Resume(busType BusType) error {
	bus, err := manager.lookupBus(busType)
	if err != nil {
		return err
	}

	bus.Resume()
	return nil
}

// PauseAll pauses consumption on all buses; publishing still enqueues
// Returns the number of buses successfully transitioned to paused and the last error if any
func (manager *EventBusManager) PauseAll() (int, error) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	count := 0
	var lastErr error
	for bt, bus := range manager.buses {
		bus.Pause() // idempotent
		if bus.IsPaused() {
			count++
		} else {
			lastErr = fmt.Errorf("failed to pause bus %d", bt)
		}
	}
	return count, lastErr
}

// ResumeAll resumes consumption on all buses
// Returns the number of buses successfully transitioned to running and the last error if any
func (manager *EventBusManager) ResumeAll() (int, error) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	count := 0
	var lastErr error
	for bt, bus := range manager.buses {
		bus.Resume()
		if !bus.IsPaused() {
			count++
		} else {
			lastErr = fmt.Errorf("failed to resume bus %d", bt)
		}
	}
	return count, lastErr
}

// UpdateBusConfig applies runtime-safe config updates to a specific bus.
func (manager *EventBusManager) UpdateBusConfig(busType BusType, cfg BusConfig) error {
	bus, err := manager.lookupBus(busType)
	if err != nil {
		return err
	}

	bus.UpdateConfig(cfg)
	return nil
}

// GetBusMetrics returns a metrics map for one bus, merging the bus's own
// EventMetrics with the manager monitor snapshot.
func (manager *EventBusManager) GetBusMetrics(busType BusType) (map[string]any, error) {
	bus, err := manager.lookupBus(busType)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"bus_type":    busType,
		"is_paused":   bus.IsPaused(),
		"is_healthy":  bus.IsHealthy(),
		"queue_size":  bus.GetQueueSize(),
		"subscribers": bus.GetTotalSubscriberCount(),
	}
	if m := bus.GetMetrics(); m != nil {
		for k, v := range m.GetMetrics() {
			result[k] = v
		}
	}
	if manager.monitor != nil {
		result["monitor"] = manager.monitor.GetMetrics()
	}
	return result, nil
}

// GetAllBusesMetrics returns metrics for all buses
func (manager *EventBusManager) GetAllBusesMetrics() map[BusType]map[string]any {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	out := make(map[BusType]map[string]any, len(manager.buses))
	for bt, bus := range manager.buses {
		pauseDur, _ := bus.GetPauseStats()
		cap, running, free, waiting := bus.GetWorkerPoolStats()
		m := map[string]any{
			"bus_type":    bt,
			"is_paused":   bus.IsPaused(),
			"is_healthy":  bus.IsHealthy(),
			"is_degraded": bus.IsDegraded(),
			"queue_size":  bus.GetQueueSize(),
			"subscribers": bus.GetTotalSubscriberCount(),
			// pause/degradation stats
			"pause_duration_ms":       pauseDur.Milliseconds(),
			"pause_count":             bus.GetPauseCount(),
			"degradation_duration_ms": bus.GetDegradationDuration().Milliseconds(),
			// worker pool stats
			"worker_cap":     cap,
			"worker_running": running,
			"worker_free":    free,
			"worker_waiting": waiting,
		}
		if em := bus.GetMetrics(); em != nil {
			for k, v := range em.GetMetrics() {
				m[k] = v
			}
		}
		out[bt] = m
	}
	return out
}

// GetEventHistory returns events from all buses that match the given filter
func (manager *EventBusManager) GetEventHistory(filter *EventFilter) []LynxEvent {
	var allEvents []LynxEvent

	manager.mu.RLock()
	defer manager.mu.RUnlock()

	for _, bus := range manager.buses {
		events := bus.GetEventHistory(filter)
		allEvents = append(allEvents, events...)
	}

	return allEvents
}

// GetPluginEventHistory returns events from all buses for a specific plugin
func (manager *EventBusManager) GetPluginEventHistory(pluginID string, filter *EventFilter) []LynxEvent {
	var allEvents []LynxEvent

	manager.mu.RLock()
	defer manager.mu.RUnlock()

	for _, bus := range manager.buses {
		events := bus.GetPluginEventHistory(pluginID, filter)
		allEvents = append(allEvents, events...)
	}

	return allEvents
}

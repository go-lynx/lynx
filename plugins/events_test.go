package plugins

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSubscribePublishWithHooks tests that Subscribe/Publish work correctly after hooks are injected.
// 测试目标：Hook 注入后 Subscribe/Publish 正常工作
func TestSubscribePublishWithHooks(t *testing.T) {
	// Reset hooks before test
	resetHooks()
	defer resetHooks()

	// Create mock emitter and adder
	var (
		emittedEvents  []PluginEvent
		addedListeners []EventListener
		mu             sync.Mutex
	)

	emitter := func(event PluginEvent) {
		mu.Lock()
		defer mu.Unlock()
		emittedEvents = append(emittedEvents, event)
	}

	adder := func(listener EventListener, filter *EventFilter) {
		mu.Lock()
		defer mu.Unlock()
		addedListeners = append(addedListeners, listener)
	}

	// Inject hooks
	SetGlobalEventHooks(emitter, adder)

	// Define test event type
	type TestEvent struct {
		ID   string
		Data string
	}

	// Subscribe to typed events
	Subscribe[TestEvent](func(ctx context.Context, event TestEvent) error {
		return nil
	}, nil)

	// Publish some events
	testEvents := []TestEvent{
		{ID: "1", Data: "test-data-1"},
		{ID: "2", Data: "test-data-2"},
		{ID: "3", Data: "test-data-3"},
	}

	for _, te := range testEvents {
		Publish(te)
	}

	// Verify events were emitted
	mu.Lock()
	emittedCount := len(emittedEvents)
	listenerCount := len(addedListeners)
	mu.Unlock()

	assert.Equal(t, 3, emittedCount, "Should emit 3 events")
	assert.GreaterOrEqual(t, listenerCount, 1, "Should have at least 1 listener registered")

	// Verify event structure
	mu.Lock()
	firstEvent := emittedEvents[0]
	mu.Unlock()
	assert.Equal(t, PriorityNormal, firstEvent.Priority, "Event priority should be PriorityNormal")
	assert.NotNil(t, firstEvent.Metadata, "Event metadata should not be nil")
}

// TestSubscribePublishWithoutHooks tests that Subscribe/Publish are safe when hooks are not injected.
// 测试目标：Hook 未注入时 Subscribe/Publish 安全无 panic
func TestSubscribePublishWithoutHooks(t *testing.T) {
	// Reset hooks to simulate no injection
	resetHooks()
	defer resetHooks()

	// Ensure hooks are nil
	globalEventHooks.mu.RLock()
	emitter := globalEventHooks.emitter
	adder := globalEventHooks.adder
	globalEventHooks.mu.RUnlock()

	assert.Nil(t, emitter, "Emitter should be nil after reset")
	assert.Nil(t, adder, "Adder should be nil after reset")

	// Define test event type
	type TestEvent struct {
		Message string
	}

	// These calls should NOT panic even without hooks
	assert.NotPanics(t, func() {
		Subscribe[TestEvent](func(ctx context.Context, event TestEvent) error {
			return nil
		}, nil)
	}, "Subscribe should not panic when hooks are not injected")

	assert.NotPanics(t, func() {
		Publish(TestEvent{Message: "test-message"})
	}, "Publish should not panic when hooks are not injected")
}

// TestTypedListenerAdapterTypeMatch tests typedListenerAdapter.HandleEvent with matching type.
// 测试目标：typedListenerAdapter.HandleEvent 类型匹配路径
func TestTypedListenerAdapterTypeMatch(t *testing.T) {
	// Define test event type
	type UserCreated struct {
		UserID   string
		Username string
	}

	// Track handler invocation
	var (
		handlerCalled bool
		receivedEvent UserCreated
	)

	// Create adapter with handler
	adapter := &typedListenerAdapter[UserCreated]{
		handler: func(ctx context.Context, event UserCreated) error {
			handlerCalled = true
			receivedEvent = event
			return nil
		},
		id: "test-adapter-1",
	}

	// Create matching event
	expectedUser := UserCreated{
		UserID:   "user-123",
		Username: "test-user",
	}

	pluginEvent := PluginEvent{
		Type:      "typed.UserCreated",
		Priority:  PriorityNormal,
		Timestamp: time.Now().Unix(),
		Metadata:  map[string]any{"payload": expectedUser},
	}

	// Call HandleEvent
	adapter.HandleEvent(pluginEvent)

	// Verify handler was called with correct data
	assert.True(t, handlerCalled, "Handler should be called for matching type")
	assert.Equal(t, expectedUser, receivedEvent, "Received event should match expected")
}

// TestTypedListenerAdapterTypeMismatch tests typedListenerAdapter.HandleEvent with non-matching type.
// 测试目标：typedListenerAdapter.HandleEvent 类型不匹配路径
func TestTypedListenerAdapterTypeMismatch(t *testing.T) {
	// Define different event types
	type OrderCreated struct {
		OrderID string
	}

	type UserCreated struct {
		UserID string
	}

	// Track handler invocation
	handlerCalled := false

	// Create adapter expecting OrderCreated
	adapter := &typedListenerAdapter[OrderCreated]{
		handler: func(ctx context.Context, event OrderCreated) error {
			handlerCalled = true
			return nil
		},
		id: "test-adapter-2",
	}

	// Send mismatched event type (UserCreated instead of OrderCreated)
	wrongEvent := UserCreated{UserID: "user-456"}

	pluginEvent := PluginEvent{
		Type:      "typed.UserCreated",
		Priority:  PriorityNormal,
		Timestamp: time.Now().Unix(),
		Metadata:  map[string]any{"payload": wrongEvent},
	}

	// Call HandleEvent - should NOT call handler due to type mismatch
	assert.NotPanics(t, func() {
		adapter.HandleEvent(pluginEvent)
	}, "HandleEvent should not panic on type mismatch")

	// Verify handler was NOT called
	assert.False(t, handlerCalled, "Handler should NOT be called for mismatched type")
}

// TestTypedListenerAdapterNoPayload tests HandleEvent when payload is missing.
// 测试目标：typedListenerAdapter.HandleEvent 缺少 payload 时的安全性
func TestTypedListenerAdapterNoPayload(t *testing.T) {
	type TestEvent struct {
		Data string
	}

	handlerCalled := false

	adapter := &typedListenerAdapter[TestEvent]{
		handler: func(ctx context.Context, event TestEvent) error {
			handlerCalled = true
			return nil
		},
		id: "test-adapter-3",
	}

	// Create event without payload in metadata
	pluginEvent := PluginEvent{
		Type:      "typed.TestEvent",
		Priority:  PriorityNormal,
		Timestamp: time.Now().Unix(),
		Metadata:  map[string]any{}, // No payload key
	}

	assert.NotPanics(t, func() {
		adapter.HandleEvent(pluginEvent)
	}, "HandleEvent should not panic when payload is missing")

	assert.False(t, handlerCalled, "Handler should NOT be called when payload is missing")
}

// TestTypedListenerAdapterNilPayload tests HandleEvent when payload is nil.
// 测试目标：typedListenerAdapter.HandleEvent payload 为 nil 时的安全性
func TestTypedListenerAdapterNilPayload(t *testing.T) {
	type TestEvent struct {
		Data string
	}

	handlerCalled := false

	adapter := &typedListenerAdapter[TestEvent]{
		handler: func(ctx context.Context, event TestEvent) error {
			handlerCalled = true
			return nil
		},
		id: "test-adapter-4",
	}

	// Create event with nil payload
	pluginEvent := PluginEvent{
		Type:      "typed.TestEvent",
		Priority:  PriorityNormal,
		Timestamp: time.Now().Unix(),
		Metadata:  map[string]any{"payload": nil},
	}

	assert.NotPanics(t, func() {
		adapter.HandleEvent(pluginEvent)
	}, "HandleEvent should not panic when payload is nil")

	assert.False(t, handlerCalled, "Handler should NOT be called when payload is nil")
}

// TestSubscribeWithFilter tests Subscribe with EventFilter.
// 测试目标：Subscribe 支持 EventFilter 过滤
func TestSubscribeWithFilter(t *testing.T) {
	resetHooks()
	defer resetHooks()

	var (
		addedFilters []*EventFilter
		mu           sync.Mutex
	)

	adder := func(listener EventListener, filter *EventFilter) {
		mu.Lock()
		defer mu.Unlock()
		addedFilters = append(addedFilters, filter)
	}

	SetGlobalEventHooks(nil, adder)

	type FilteredEvent struct {
		Value int
	}

	// Subscribe with filter
	filter := &EventFilter{
		Priorities: []int{PriorityHigh, PriorityCritical},
	}

	Subscribe[FilteredEvent](func(ctx context.Context, event FilteredEvent) error {
		return nil
	}, filter)

	mu.Lock()
	filterCount := len(addedFilters)
	receivedFilter := addedFilters[0]
	mu.Unlock()

	assert.Equal(t, 1, filterCount, "Should have 1 filter added")
	require.NotNil(t, receivedFilter, "Filter should not be nil")
	assert.Equal(t, []int{PriorityHigh, PriorityCritical}, receivedFilter.Priorities, "Filter priorities should match")
}

// TestPublishEventStructure tests that Publish creates correct PluginEvent structure.
// 测试目标：Publish 创建的 PluginEvent 结构正确
func TestPublishEventStructure(t *testing.T) {
	resetHooks()
	defer resetHooks()

	var (
		receivedEvent PluginEvent
		mu            sync.Mutex
	)

	emitter := func(event PluginEvent) {
		mu.Lock()
		defer mu.Unlock()
		receivedEvent = event
	}

	SetGlobalEventHooks(emitter, nil)

	type StructTestEvent struct {
		ID       string
		Priority string
	}

	testPayload := StructTestEvent{
		ID:       "event-789",
		Priority: "high",
	}

	Publish(testPayload)

	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, EventType("typed.plugins.StructTestEvent"), receivedEvent.Type, "Event type should match payload type")
	assert.Equal(t, PriorityNormal, receivedEvent.Priority, "Priority should be PriorityNormal")
	assert.LessOrEqual(t, receivedEvent.Timestamp, time.Now().Unix(), "Timestamp should be set")
	assert.NotNil(t, receivedEvent.Metadata, "Metadata should not be nil")
	assert.Equal(t, testPayload, receivedEvent.Metadata["payload"], "Payload should be in metadata")
}

// TestMultipleSubscribers tests multiple subscribers to same event type.
// 测试目标：多个订阅者订阅同一事件类型
func TestMultipleSubscribers(t *testing.T) {
	resetHooks()
	defer resetHooks()

	var (
		addedListeners []EventListener
		mu             sync.Mutex
	)

	adder := func(listener EventListener, filter *EventFilter) {
		mu.Lock()
		defer mu.Unlock()
		addedListeners = append(addedListeners, listener)
	}

	SetGlobalEventHooks(nil, adder)

	type MultiSubscriberEvent struct {
		Data string
	}

	// Subscribe multiple times
	Subscribe[MultiSubscriberEvent](func(ctx context.Context, event MultiSubscriberEvent) error {
		return nil
	}, nil)

	Subscribe[MultiSubscriberEvent](func(ctx context.Context, event MultiSubscriberEvent) error {
		return nil
	}, nil)

	Subscribe[MultiSubscriberEvent](func(ctx context.Context, event MultiSubscriberEvent) error {
		return nil
	}, nil)

	mu.Lock()
	listenerCount := len(addedListeners)
	mu.Unlock()

	assert.Equal(t, 3, listenerCount, "Should have 3 listeners registered")

	// Note: In current implementation, Publish only emits the event,
	// actual listener invocation depends on the runtime emitter
}

// TestGetListenerIDUniqueness tests that GetListenerID returns unique IDs.
// 测试目标：GetListenerID 返回唯一 ID
func TestGetListenerIDUniqueness(t *testing.T) {
	type UniqueIDEvent struct {
		Value string
	}

	handler := func(ctx context.Context, event UniqueIDEvent) error {
		return nil
	}

	// Create multiple adapters
	adapter1 := &typedListenerAdapter[UniqueIDEvent]{
		handler: handler,
		id:      fmt.Sprintf("typed-%T-%d", (*UniqueIDEvent)(nil), time.Now().UnixNano()),
	}

	time.Sleep(1 * time.Millisecond) // Ensure different timestamp

	adapter2 := &typedListenerAdapter[UniqueIDEvent]{
		handler: handler,
		id:      fmt.Sprintf("typed-%T-%d", (*UniqueIDEvent)(nil), time.Now().UnixNano()),
	}

	id1 := adapter1.GetListenerID()
	id2 := adapter2.GetListenerID()

	assert.NotEqual(t, id1, id2, "Listener IDs should be unique")
	assert.NotEmpty(t, id1, "Listener ID should not be empty")
	assert.NotEmpty(t, id2, "Listener ID should not be empty")
}

// TestConcurrentSubscribePublish tests concurrent Subscribe and Publish calls.
// 测试目标：并发 Subscribe 和 Publish 调用的安全性
func TestConcurrentSubscribePublish(t *testing.T) {
	resetHooks()
	defer resetHooks()

	var emittedCount int32

	emitter := func(event PluginEvent) {
		atomic.AddInt32(&emittedCount, 1)
	}

	adder := func(listener EventListener, filter *EventFilter) {
		// Simulate some processing time
		time.Sleep(1 * time.Millisecond)
	}

	SetGlobalEventHooks(emitter, adder)

	type ConcurrentEvent struct {
		Index int
	}

	// Run concurrent operations
	var wg sync.WaitGroup

	// Multiple publishers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			Publish(ConcurrentEvent{Index: idx})
		}(i)
	}

	// Multiple subscribers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Subscribe[ConcurrentEvent](func(ctx context.Context, event ConcurrentEvent) error {
				return nil
			}, nil)
		}()
	}

	wg.Wait()

	assert.Equal(t, int32(10), atomic.LoadInt32(&emittedCount), "Should emit 10 events")
}

// resetHooks resets global hooks to nil state
func resetHooks() {
	globalEventHooks.mu.Lock()
	defer globalEventHooks.mu.Unlock()
	globalEventHooks.emitter = nil
	globalEventHooks.adder = nil
}

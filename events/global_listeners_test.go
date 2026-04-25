package events

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---- Global event bus tests ----

func TestInitGlobalEventBus_IdempotentOnRepeatCall(t *testing.T) {
	resetGlobalEventBusStateForTest(t)
	defer resetGlobalEventBusStateForTest(t)

	if err := InitGlobalEventBus(DefaultBusConfigs()); err != nil {
		t.Fatalf("first InitGlobalEventBus failed: %v", err)
	}
	first := GetGlobalEventBus()

	// Second call should be a no-op and return the same manager.
	if err := InitGlobalEventBus(DefaultBusConfigs()); err != nil {
		t.Fatalf("second InitGlobalEventBus failed: %v", err)
	}
	second := GetGlobalEventBus()

	if first != second {
		t.Error("expected identical manager on repeated InitGlobalEventBus calls")
	}
}

func TestSetGlobalEventBus_ReplacesManager(t *testing.T) {
	resetGlobalEventBusStateForTest(t)
	defer resetGlobalEventBusStateForTest(t)

	mgr1 := GetGlobalEventBus()
	mgr2, err := NewEventBusManager(DefaultBusConfigs())
	if err != nil {
		t.Fatalf("NewEventBusManager: %v", err)
	}
	defer mgr2.Close()

	SetGlobalEventBus(mgr2)
	if got := GetGlobalEventBus(); got != mgr2 {
		t.Errorf("expected mgr2 after SetGlobalEventBus, got %p (original was %p)", got, mgr1)
	}
}

func TestSetDefaultEventBusProvider_OverridesGlobal(t *testing.T) {
	resetGlobalEventBusStateForTest(t)
	defer func() {
		ClearDefaultEventBusProvider()
		resetGlobalEventBusStateForTest(t)
	}()

	custom, err := NewEventBusManager(DefaultBusConfigs())
	if err != nil {
		t.Fatalf("NewEventBusManager: %v", err)
	}
	defer custom.Close()

	SetDefaultEventBusProvider(func() *EventBusManager { return custom })

	if got := GetGlobalEventBus(); got != custom {
		t.Error("expected custom manager from provider, got different manager")
	}
}

func TestClearDefaultEventBusProvider_FallsBackToGlobal(t *testing.T) {
	resetGlobalEventBusStateForTest(t)
	defer resetGlobalEventBusStateForTest(t)

	custom, err := NewEventBusManager(DefaultBusConfigs())
	if err != nil {
		t.Fatalf("NewEventBusManager: %v", err)
	}
	defer custom.Close()

	SetDefaultEventBusProvider(func() *EventBusManager { return custom })
	ClearDefaultEventBusProvider()

	// Should now auto-create the global manager, not return the custom one.
	got := GetGlobalEventBus()
	if got == custom {
		t.Error("expected global fallback manager after ClearDefaultEventBusProvider, got custom manager")
	}
}

func TestPublishEventWithManager_NilManagerReturnsError(t *testing.T) {
	event := NewLynxEvent(EventPluginStarted, "test", "test")
	if err := PublishEventWithManager(nil, event); err == nil {
		t.Error("expected error when manager is nil")
	}
}

func TestSubscribeWithManager_NilManagerReturnsError(t *testing.T) {
	if err := SubscribeWithManager(nil, BusTypePlugin, func(LynxEvent) {}); err == nil {
		t.Error("expected error when manager is nil")
	}
}

func TestSubscribeToWithManager_NilManagerReturnsError(t *testing.T) {
	if err := SubscribeToWithManager(nil, EventPluginStarted, func(LynxEvent) {}); err == nil {
		t.Error("expected error when manager is nil")
	}
}

func TestCloseGlobalEventBus_NoManagerIsNoOp(t *testing.T) {
	resetGlobalEventBusStateForTest(t)
	defer resetGlobalEventBusStateForTest(t)

	if err := CloseGlobalEventBus(); err != nil {
		t.Errorf("CloseGlobalEventBus with no manager should not error, got: %v", err)
	}
}

func TestCloseGlobalEventBus_ClosesManager(t *testing.T) {
	resetGlobalEventBusStateForTest(t)
	defer resetGlobalEventBusStateForTest(t)

	if err := InitGlobalEventBus(DefaultBusConfigs()); err != nil {
		t.Fatalf("InitGlobalEventBus: %v", err)
	}
	if err := CloseGlobalEventBus(); err != nil {
		t.Errorf("CloseGlobalEventBus returned error: %v", err)
	}
}

// ---- EventListenerManager tests ----

func TestEventListenerManager_AddAndRemoveListener(t *testing.T) {
	mgr, err := NewEventBusManager(DefaultBusConfigs())
	if err != nil {
		t.Fatalf("NewEventBusManager: %v", err)
	}
	defer mgr.Close()

	lm := NewEventListenerManagerWithEventBus(mgr)

	added := lm.AddListener("l1", nil, func(LynxEvent) {}, BusTypePlugin)
	if added != nil {
		t.Fatalf("AddListener failed: %v", added)
	}

	// Duplicate ID must fail.
	if err := lm.AddListener("l1", nil, func(LynxEvent) {}, BusTypePlugin); err == nil {
		t.Error("expected error on duplicate listener ID")
	}

	if err := lm.RemoveListener("l1"); err != nil {
		t.Errorf("RemoveListener failed: %v", err)
	}

	// Removing again should fail.
	if err := lm.RemoveListener("l1"); err == nil {
		t.Error("expected error when removing non-existent listener")
	}
}

func TestEventListenerManager_ListenerReceivesEvents(t *testing.T) {
	mgr, err := NewEventBusManager(DefaultBusConfigs())
	if err != nil {
		t.Fatalf("NewEventBusManager: %v", err)
	}
	defer mgr.Close()

	lm := NewEventListenerManagerWithEventBus(mgr)

	var count atomic.Int32
	if err := lm.AddListener("counter", nil, func(LynxEvent) { count.Add(1) }, BusTypePlugin); err != nil {
		t.Fatalf("AddListener: %v", err)
	}
	defer lm.RemoveListener("counter") //nolint:errcheck

	event := NewLynxEvent(EventPluginStarted, "test", "test")
	_ = PublishEventWithManager(mgr, event)

	if !waitUntil(time.Second, func() bool { return count.Load() > 0 }) {
		t.Errorf("listener did not receive published event within 1s (count=%d)", count.Load())
	}
}

func TestEventListenerManager_AddListenerWithContext_CleansUpOnCancel(t *testing.T) {
	mgr, err := NewEventBusManager(DefaultBusConfigs())
	if err != nil {
		t.Fatalf("NewEventBusManager: %v", err)
	}
	defer mgr.Close()

	lm := NewEventListenerManagerWithEventBus(mgr)

	// Use a manually cancellable context via a channel trick – build one that
	// we can cancel by closing the done channel.
	ctx, cancel := newManualContext()

	if err := lm.AddListenerWithContext(ctx, "ctx-listener", nil, func(LynxEvent) {}, BusTypePlugin); err != nil {
		t.Fatalf("AddListenerWithContext: %v", err)
	}

	// Listener should exist now.
	lm.mu.RLock()
	_, exists := lm.listeners["ctx-listener"]
	lm.mu.RUnlock()
	if !exists {
		t.Fatal("listener not found after AddListenerWithContext")
	}

	// Cancel the context – the goroutine should remove the listener.
	cancel()
	if !waitUntil(time.Second, func() bool {
		lm.mu.RLock()
		_, exists = lm.listeners["ctx-listener"]
		lm.mu.RUnlock()
		return !exists
	}) {
		t.Error("listener was not cleaned up after context cancellation")
	}
}

func TestEventListenerManager_NilEventBus_ReturnsError(t *testing.T) {
	lm := NewEventListenerManagerWithEventBus(nil)
	err := lm.AddListener("x", nil, func(LynxEvent) {}, BusTypePlugin)
	if err == nil {
		t.Error("expected error when event bus manager is nil")
	}
}

func TestEventListenerManager_FilteredListener(t *testing.T) {
	mgr, err := NewEventBusManager(DefaultBusConfigs())
	if err != nil {
		t.Fatalf("NewEventBusManager: %v", err)
	}
	defer mgr.Close()

	lm := NewEventListenerManagerWithEventBus(mgr)

	filter := &EventFilter{EventTypes: []EventType{EventPluginStarted}}
	var received atomic.Int32
	if err := lm.AddListener("filtered", filter, func(LynxEvent) { received.Add(1) }, BusTypePlugin); err != nil {
		t.Fatalf("AddListener: %v", err)
	}
	defer lm.RemoveListener("filtered") //nolint:errcheck

	// Publish a matching event.
	_ = PublishEventWithManager(mgr, NewLynxEvent(EventPluginStarted, "src", "comp"))

	if !waitUntil(time.Second, func() bool { return received.Load() > 0 }) {
		t.Error("filtered listener did not receive matching event")
	}
}

func waitUntil(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return condition()
}

func TestGetGlobalBusStatus_ReturnsMap(t *testing.T) {
	resetGlobalEventBusStateForTest(t)
	defer resetGlobalEventBusStateForTest(t)

	status := GetGlobalBusStatus()
	if status == nil {
		t.Error("expected non-nil bus status map")
	}
}

func TestGetGlobalConfigs_ReturnsConfigs(t *testing.T) {
	resetGlobalEventBusStateForTest(t)
	defer resetGlobalEventBusStateForTest(t)

	cfgs := GetGlobalConfigs()
	// DefaultBusConfigs sets non-zero MaxQueue; just sanity-check it round-trips.
	if cfgs.Plugin.MaxQueue == 0 && cfgs.System.MaxQueue == 0 {
		t.Error("expected non-zero MaxQueue in default bus configs")
	}
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

// manualContext is a minimal context.Context that can be cancelled by calling
// the returned CancelFunc, without importing "context" from a test-only file.
type manualContext struct {
	done chan struct{}
	once sync.Once
}

func newManualContext() (*manualContext, func()) {
	mc := &manualContext{done: make(chan struct{})}
	return mc, func() { mc.once.Do(func() { close(mc.done) }) }
}

func (m *manualContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (m *manualContext) Done() <-chan struct{}       { return m.done }
func (m *manualContext) Err() error {
	select {
	case <-m.done:
		return &canceledError{}
	default:
		return nil
	}
}
func (m *manualContext) Value(any) any { return nil }

type canceledError struct{}

func (canceledError) Error() string { return "context canceled" }

package events

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestManagerCloseDoesNotBlockOnReentrantHandler verifies that Close returns
// promptly even while a handler is blocked calling back into the manager.
// Before the fix, Close held manager.mu across bus.Close, so the handler's
// PublishEvent blocked on mu.RLock, the bus waited on the handler, and Close
// took CloseTimeout per bus.
func TestManagerCloseDoesNotBlockOnReentrantHandler(t *testing.T) {
	configs := DefaultBusConfigs()
	manager, err := NewEventBusManager(configs)
	if err != nil {
		t.Fatalf("NewEventBusManager: %v", err)
	}

	inHandler := make(chan struct{})
	release := make(chan struct{})
	var publishErr atomic.Value
	handlerDone := make(chan struct{})

	if err := manager.Subscribe(BusTypePlugin, func(LynxEvent) {
		close(inHandler)
		<-release
		e := manager.PublishEvent(NewLynxEvent(EventSystemError, "p", "s"))
		if e != nil {
			publishErr.Store(e)
		}
		close(handlerDone)
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := manager.PublishEvent(NewLynxEvent(EventPluginInitialized, "p", "s")); err != nil {
		t.Fatalf("PublishEvent: %v", err)
	}
	select {
	case <-inHandler:
	case <-time.After(3 * time.Second):
		t.Fatal("handler was not invoked")
	}

	closeDone := make(chan error, 1)
	go func() { closeDone <- manager.Close() }()
	// Let Close mark the manager closed before the handler re-enters it.
	time.Sleep(50 * time.Millisecond)
	close(release)

	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not return within 3s while a handler re-entered the manager")
	}
	<-handlerDone
	if e, _ := publishErr.Load().(error); !errors.Is(e, ErrManagerClosed) {
		t.Fatalf("expected handler's PublishEvent to fail with ErrManagerClosed, got %v", e)
	}
}

// TestManagerClosedOperationsReturnError verifies that manager operations fail
// fast with ErrManagerClosed after Close, and that Close is idempotent.
func TestManagerClosedOperationsReturnError(t *testing.T) {
	manager, err := NewEventBusManager(DefaultBusConfigs())
	if err != nil {
		t.Fatalf("NewEventBusManager: %v", err)
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if bus := manager.GetBus(BusTypePlugin); bus != nil {
		t.Fatal("GetBus should return nil after Close")
	}
	if err := manager.PublishEvent(NewLynxEvent(EventPluginInitialized, "p", "s")); !errors.Is(err, ErrManagerClosed) {
		t.Fatalf("PublishEvent after Close: want ErrManagerClosed, got %v", err)
	}
	if err := manager.Subscribe(BusTypePlugin, func(LynxEvent) {}); !errors.Is(err, ErrManagerClosed) {
		t.Fatalf("Subscribe after Close: want ErrManagerClosed, got %v", err)
	}
	if err := manager.Pause(BusTypePlugin); !errors.Is(err, ErrManagerClosed) {
		t.Fatalf("Pause after Close: want ErrManagerClosed, got %v", err)
	}
}

// TestSystemBusPauseResumeDoesNotDeadlock verifies that pausing/resuming the
// system bus, whose notifications are routed back to itself, does not
// self-deadlock, and that both notifications are still delivered.
func TestSystemBusPauseResumeDoesNotDeadlock(t *testing.T) {
	manager, err := NewEventBusManager(DefaultBusConfigs())
	if err != nil {
		t.Fatalf("NewEventBusManager: %v", err)
	}
	defer manager.Close()

	sysBus := manager.GetBus(BusTypeSystem)
	if sysBus == nil {
		t.Fatal("system bus not found")
	}
	// Sanity: the notification really is routed to the system bus.
	probe := NewLynxEvent(EventSystemError, "system", "event-bus")
	if bt := manager.classifier.GetBusType(probe); bt != BusTypeSystem {
		t.Fatalf("EventSystemError routed to bus %d, want %d", bt, BusTypeSystem)
	}

	var mu sync.Mutex
	statuses := map[string]int{}
	seen := make(chan struct{}, 16)
	if err := manager.Subscribe(BusTypeSystem, func(ev LynxEvent) {
		if ev.EventType != EventSystemError {
			return
		}
		mu.Lock()
		statuses[ev.Status]++
		mu.Unlock()
		seen <- struct{}{}
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		sysBus.Pause()
		sysBus.Resume()
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Pause/Resume on the system bus deadlocked")
	}
	if sysBus.IsPaused() {
		t.Fatal("system bus should be resumed")
	}

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		ok := statuses["paused"] >= 1 && statuses["resumed"] >= 1
		mu.Unlock()
		if ok {
			break
		}
		select {
		case <-seen:
		case <-deadline:
			mu.Lock()
			defer mu.Unlock()
			t.Fatalf("expected paused and resumed notifications, got %v", statuses)
		}
	}
}

// TestDuplicateEventIDsAreDeliveredToEverySubscriber pins the decided
// semantics: the bus performs no EventID deduplication. Re-publishing an event
// with the same EventID is delivered again, and every subscriber on the bus
// receives every delivery.
func TestDuplicateEventIDsAreDeliveredToEverySubscriber(t *testing.T) {
	manager, err := NewEventBusManager(DefaultBusConfigs())
	if err != nil {
		t.Fatalf("NewEventBusManager: %v", err)
	}
	defer manager.Close()

	var a, b atomic.Int32
	if err := manager.SubscribeTo(EventPluginInitialized, func(LynxEvent) { a.Add(1) }); err != nil {
		t.Fatalf("SubscribeTo: %v", err)
	}
	if err := manager.SubscribeTo(EventPluginInitialized, func(LynxEvent) { b.Add(1) }); err != nil {
		t.Fatalf("SubscribeTo: %v", err)
	}

	ev := NewLynxEvent(EventPluginInitialized, "p", "s")
	if ev.EventID == "" {
		t.Fatal("expected a generated EventID")
	}
	for i := 0; i < 3; i++ {
		if err := manager.PublishEvent(ev); err != nil {
			t.Fatalf("PublishEvent: %v", err)
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if a.Load() == 3 && b.Load() == 3 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected each subscriber to receive 3 deliveries, got a=%d b=%d", a.Load(), b.Load())
}

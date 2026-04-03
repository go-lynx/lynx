package plugins

import "testing"

type runtimeTestAdapter struct {
	listeners       map[string]func(Event[PluginEvent])
	pluginListeners map[string]pluginEventHandler
	history         []PluginEvent
}

type pluginEventHandler struct {
	pluginName string
	handler    func(Event[PluginEvent])
}

func newRuntimeTestAdapter() *runtimeTestAdapter {
	return &runtimeTestAdapter{
		listeners:       make(map[string]func(Event[PluginEvent])),
		pluginListeners: make(map[string]pluginEventHandler),
		history:         make([]PluginEvent, 0),
	}
}

func (a *runtimeTestAdapter) PublishEvent(event PluginEvent) error {
	a.history = append(a.history, event)
	wrapped := NewEvent(event)
	for _, handler := range a.listeners {
		handler(wrapped)
	}
	for _, entry := range a.pluginListeners {
		if event.PluginID == entry.pluginName {
			entry.handler(wrapped)
		}
	}
	return nil
}

func (a *runtimeTestAdapter) Subscribe(eventType EventType, handler func(PluginEvent)) error {
	return nil
}

func (a *runtimeTestAdapter) SubscribeTo(eventType EventType, handler func(PluginEvent)) error {
	return nil
}

func (a *runtimeTestAdapter) AddTypedListener(id string, filter *EventFilter, handler func(Event[PluginEvent]), bus string) error {
	a.listeners[id] = handler
	return nil
}

func (a *runtimeTestAdapter) RemoveListener(id string) error {
	delete(a.listeners, id)
	delete(a.pluginListeners, id)
	return nil
}

func (a *runtimeTestAdapter) AddTypedPluginListener(pluginName string, id string, filter *EventFilter, handler func(Event[PluginEvent])) error {
	a.pluginListeners[id] = pluginEventHandler{
		pluginName: pluginName,
		handler:    handler,
	}
	return nil
}

func (a *runtimeTestAdapter) GetEventHistory(filter *EventFilter) []PluginEvent {
	return append([]PluginEvent(nil), a.history...)
}

type legacyRuntimeListener struct {
	id       string
	received []PluginEvent
}

func (l *legacyRuntimeListener) HandleEvent(event PluginEvent) {
	l.received = append(l.received, event)
}

func (l *legacyRuntimeListener) GetListenerID() string {
	return l.id
}

type typedRuntimeListener struct {
	id       string
	received []Event[PluginEvent]
}

func (l *typedRuntimeListener) HandleTypedEvent(event Event[PluginEvent]) {
	l.received = append(l.received, event)
}

func (l *typedRuntimeListener) GetListenerID() string {
	return l.id
}

func TestUnifiedRuntime_AddListener_UsesTypedAdapterPath(t *testing.T) {
	runtime := NewUnifiedRuntime()
	adapter := newRuntimeTestAdapter()
	runtime.SetEventBusAdapter(adapter)

	listener := &legacyRuntimeListener{id: "legacy-listener"}
	runtime.AddListener(listener, nil)
	runtime.EmitEvent(PluginEvent{Type: EventPluginStarted, PluginID: "demo"})

	if len(listener.received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(listener.received))
	}
	if listener.received[0].PluginID != "demo" {
		t.Fatalf("expected plugin id demo, got %q", listener.received[0].PluginID)
	}
}

func TestUnifiedRuntime_TypedPluginListenerAndHistory(t *testing.T) {
	runtime := NewUnifiedRuntime()
	adapter := newRuntimeTestAdapter()
	runtime.SetEventBusAdapter(adapter)

	listener := &typedRuntimeListener{id: "typed-listener"}
	runtime.AddTypedPluginListener("target-plugin", listener, nil)

	runtime.EmitTypedEvent(NewEvent(PluginEvent{Type: EventPluginStarted, PluginID: "target-plugin"}))
	runtime.EmitTypedEvent(NewEvent(PluginEvent{Type: EventPluginStarted, PluginID: "other-plugin"}))

	if len(listener.received) != 1 {
		t.Fatalf("expected 1 typed plugin event, got %d", len(listener.received))
	}
	if listener.received[0].Unwrap().PluginID != "target-plugin" {
		t.Fatalf("unexpected plugin id %q", listener.received[0].Unwrap().PluginID)
	}

	history := runtime.GetTypedEventHistory(EventFilter{})
	if len(history) != 2 {
		t.Fatalf("expected 2 history events, got %d", len(history))
	}
}

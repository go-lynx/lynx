package events

import (
	"strings"
	"testing"

	"github.com/go-lynx/lynx/plugins"
)

func TestPluginEventBusAdapter_NilManagerErrors(t *testing.T) {
	adapter := NewPluginEventBusAdapterWithListenerManager(nil, nil)

	if err := adapter.PublishEvent(plugins.PluginEvent{}); err == nil || !strings.Contains(err.Error(), "event manager not initialized") {
		t.Fatalf("PublishEvent error = %v, want event manager not initialized", err)
	}
	if err := adapter.Subscribe(plugins.EventPluginStarted, func(plugins.PluginEvent) {}); err == nil || !strings.Contains(err.Error(), "event manager not initialized") {
		t.Fatalf("Subscribe error = %v, want event manager not initialized", err)
	}
	if err := adapter.SubscribeTo(plugins.EventPluginStarted, func(plugins.PluginEvent) {}); err == nil || !strings.Contains(err.Error(), "event manager not initialized") {
		t.Fatalf("SubscribeTo error = %v, want event manager not initialized", err)
	}
}

func TestPluginEventBusAdapter_NilHandlerErrors(t *testing.T) {
	manager, err := NewEventBusManager(DefaultBusConfigs())
	if err != nil {
		t.Fatalf("NewEventBusManager: %v", err)
	}
	defer manager.Close()

	adapter := NewPluginEventBusAdapter(manager)
	if err := adapter.Subscribe(plugins.EventPluginStarted, nil); err == nil || !strings.Contains(err.Error(), "event handler cannot be nil") {
		t.Fatalf("Subscribe error = %v, want event handler cannot be nil", err)
	}
	if err := adapter.SubscribeTo(plugins.EventPluginStarted, nil); err == nil || !strings.Contains(err.Error(), "event handler cannot be nil") {
		t.Fatalf("SubscribeTo error = %v, want event handler cannot be nil", err)
	}
}

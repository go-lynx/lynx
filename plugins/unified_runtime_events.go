package plugins

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// EmitEvent publishes an event.
func (r *UnifiedRuntime) EmitEvent(event PluginEvent) {
	if r.isClosed() || event.Type == "" {
		return
	}
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}

	adapter := r.getEventAdapter()
	if adapter == nil {
		return
	}
	if err := adapter.PublishEvent(event); err != nil {
		if logger := r.GetLogger(); logger != nil {
			logger.Log(log.LevelError, "msg", "failed to publish event", "error", err, "event_type", event.Type, "plugin_id", event.PluginID)
		}
	}
}

// EmitPluginEvent publishes a plugin event.
func (r *UnifiedRuntime) EmitPluginEvent(pluginName string, eventType string, data map[string]any) {
	r.EmitEvent(PluginEvent{
		Type:      EventType(eventType),
		PluginID:  pluginName,
		Metadata:  data,
		Timestamp: time.Now().Unix(),
	})
}

// AddListener adds an event listener.
func (r *UnifiedRuntime) AddListener(listener EventListener, filter *EventFilter) {
	if listener == nil {
		return
	}

	adapter := r.getEventAdapter()
	if adapter == nil {
		return
	}

	id := listener.GetListenerID()
	if id == "" {
		id = fmt.Sprintf("listener-%d", time.Now().UnixNano())
	}

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

	if filter != nil && len(filter.Types) > 0 {
		for _, t := range filter.Types {
			_ = adapter.SubscribeTo(t, func(pe PluginEvent) {
				listener.HandleEvent(pe)
			})
		}
	}
}

// RemoveListener removes an event listener.
func (r *UnifiedRuntime) RemoveListener(listener EventListener) {
	if listener == nil {
		return
	}
	id := listener.GetListenerID()
	if id == "" {
		return
	}
	adapter := r.getEventAdapter()
	if adapter == nil {
		return
	}
	type removeListenerIface interface {
		RemoveListener(id string) error
	}
	if rl, ok := adapter.(removeListenerIface); ok {
		_ = rl.RemoveListener(id)
	}
}

// AddPluginListener adds a plugin-specific event listener.
func (r *UnifiedRuntime) AddPluginListener(pluginName string, listener EventListener, filter *EventFilter) {
	if listener == nil {
		return
	}
	adapter := r.getEventAdapter()
	if adapter == nil {
		return
	}
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

// GetEventHistory returns event history.
func (r *UnifiedRuntime) GetEventHistory(filter EventFilter) []PluginEvent {
	adapter := r.getEventAdapter()
	if adapter == nil {
		return nil
	}
	type historyIface interface {
		GetEventHistory(filter *EventFilter) []PluginEvent
	}
	if hi, ok := adapter.(historyIface); ok {
		return hi.GetEventHistory(&filter)
	}
	return nil
}

// GetPluginEventHistory returns plugin event history.
func (r *UnifiedRuntime) GetPluginEventHistory(pluginName string, filter EventFilter) []PluginEvent {
	adapter := r.getEventAdapter()
	if adapter == nil {
		return nil
	}
	type pluginHistoryIface interface {
		GetPluginEventHistory(pluginName string, filter *EventFilter) []PluginEvent
	}
	if phi, ok := adapter.(pluginHistoryIface); ok {
		return phi.GetPluginEventHistory(pluginName, &filter)
	}
	return nil
}

// SetEventDispatchMode sets event dispatch mode via the adapter.
func (r *UnifiedRuntime) SetEventDispatchMode(mode string) error {
	adapter := r.getEventAdapter()
	if adapter == nil {
		return nil
	}
	if configurable, ok := adapter.(interface{ SetDispatchMode(string) error }); ok {
		return configurable.SetDispatchMode(mode)
	}
	return nil
}

// SetEventWorkerPoolSize sets event worker pool size via the adapter.
func (r *UnifiedRuntime) SetEventWorkerPoolSize(size int) {
	adapter := r.getEventAdapter()
	if adapter == nil {
		return
	}
	if configurable, ok := adapter.(interface{ SetWorkerPoolSize(int) }); ok {
		configurable.SetWorkerPoolSize(size)
	}
}

// SetEventTimeout sets event timeout via the adapter.
func (r *UnifiedRuntime) SetEventTimeout(timeout time.Duration) {
	adapter := r.getEventAdapter()
	if adapter == nil {
		return
	}
	if configurable, ok := adapter.(interface{ SetEventTimeout(time.Duration) }); ok {
		configurable.SetEventTimeout(timeout)
	}
}

// GetEventStats returns event stats from the adapter plus runtime state.
func (r *UnifiedRuntime) GetEventStats() map[string]any {
	stats := map[string]any{
		"runtime_closed": r.isClosed(),
	}

	adapter := r.getEventAdapter()
	if adapter == nil {
		return stats
	}
	if statsProvider, ok := adapter.(interface{ GetStats() map[string]any }); ok {
		for k, v := range statsProvider.GetStats() {
			stats[k] = v
		}
	}
	return stats
}

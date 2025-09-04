package events

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

var (
	globalManager *EventBusManager
	globalOnce    sync.Once
	globalMu      sync.RWMutex
)

// InitGlobalEventBus initializes the global event bus manager
func InitGlobalEventBus(configs BusConfigs) error {
	var initErr error
	globalOnce.Do(func() {
		manager, err := NewEventBusManager(configs)
		if err != nil {
			initErr = err
			return
		}
		globalManager = manager
	})
	return initErr
}

// GetGlobalEventBus returns the global event bus manager
func GetGlobalEventBus() *EventBusManager {
	globalMu.RLock()
	defer globalMu.RUnlock()

	if globalManager == nil {
		// Initialize with default configs if not already initialized
		// Note: This will panic if initialization fails, but it's better than silent failure
		if err := InitGlobalEventBus(DefaultBusConfigs()); err != nil {
			panic(fmt.Sprintf("failed to initialize global event bus: %v", err))
		}
	}

	return globalManager
}

// SetGlobalEventBus sets the global event bus manager
func SetGlobalEventBus(manager *EventBusManager) {
	globalMu.Lock()
	defer globalMu.Unlock()

	globalManager = manager
}

// PublishEvent publishes an event to the global event bus
func PublishEvent(event LynxEvent) error {
	return GetGlobalEventBus().PublishEvent(event)
}

// Subscribe subscribes to events on a specific bus
func Subscribe(busType BusType, handler func(LynxEvent)) error {
	return GetGlobalEventBus().Subscribe(busType, handler)
}

// SubscribeTo subscribes to a specific event type
func SubscribeTo(eventType EventType, handler func(LynxEvent)) error {
	return GetGlobalEventBus().SubscribeTo(eventType, handler)
}

// CloseGlobalEventBus closes the global event bus manager
func CloseGlobalEventBus() error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalManager != nil {
		// Emit system shutdown event before closing
		shutdownEvent := LynxEvent{
			EventType: EventSystemShutdown,
			Priority:  PriorityHigh,
			Source:    "event-system",
			Category:  "system",
			PluginID:  "system",
			Status:    "shutdown",
			Timestamp: time.Now().Unix(),
			Metadata: map[string]any{
				"reason": "event_system_close",
			},
		}

		// Publish shutdown event to all buses with timeout
		shutdownTimeout := time.After(500 * time.Millisecond)
		shutdownDone := make(chan struct{})

		go func() {
			for busType := range globalManager.buses {
				if bus := globalManager.GetBus(busType); bus != nil {
					select {
					case <-shutdownTimeout:
						return
					default:
						bus.Publish(shutdownEvent)
					}
				}
			}
			close(shutdownDone)
		}()

		// Wait for shutdown event to be processed or timeout
		select {
		case <-shutdownDone:
			// Shutdown event processed successfully
		case <-shutdownTimeout:
			// Timeout reached, proceed with closing
		}

		return globalManager.Close()
	}
	return nil
}

// SetGlobalLogger sets the logger for the global event bus manager
func SetGlobalLogger(logger log.Logger) {
	GetGlobalEventBus().SetLogger(logger)
}

// GetGlobalBusStatus returns the status of all buses in the global manager
func GetGlobalBusStatus() map[BusType]BusStatus {
	return GetGlobalEventBus().GetBusStatus()
}

// GetGlobalClassifier returns the event classifier from the global manager
func GetGlobalClassifier() *EventClassifier {
	return GetGlobalEventBus().GetClassifier()
}

// GetGlobalConfigs returns the bus configurations from the global manager
func GetGlobalConfigs() BusConfigs {
	return GetGlobalEventBus().GetConfigs()
}

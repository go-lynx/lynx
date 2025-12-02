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
	// First check without lock (fast path)
	globalMu.RLock()
	manager := globalManager
	globalMu.RUnlock()

	if manager != nil {
		return manager
	}

	// Double-checked locking pattern: acquire write lock for initialization
	globalMu.Lock()
	defer globalMu.Unlock()

	// Check again after acquiring write lock (another goroutine might have initialized it)
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
		shutdownEvent := NewLynxEvent(EventSystemShutdown, "system", "event-system").
			WithPriority(PriorityHigh).
			WithCategory("system").
			WithStatus("shutdown").
			WithMetadata("reason", "event_system_close")

		// Publish shutdown event to all buses with timeout
		shutdownTimeout := time.After(500 * time.Millisecond)
		shutdownDone := make(chan struct{})

		go func() {
			defer func() {
				// Always close done channel to prevent goroutine leak
				select {
				case shutdownDone <- struct{}{}:
				default:
					close(shutdownDone)
				}
				if r := recover(); r != nil {
					// Log panic but don't crash
					fmt.Printf("[lynx-error] panic in shutdown event goroutine: %v\n", r)
				}
			}()
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

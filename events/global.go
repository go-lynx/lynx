package events

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

var (
	globalManager *EventBusManager
	globalMu      sync.RWMutex
	globalInitErr error

	defaultEventBusProvider func() *EventBusManager
)

// SetDefaultEventBusProvider wires compatibility global access to an external owner such as LynxApp.
func SetDefaultEventBusProvider(provider func() *EventBusManager) {
	globalMu.Lock()
	defer globalMu.Unlock()
	defaultEventBusProvider = provider
}

// InitGlobalEventBus initializes the global event bus manager
func InitGlobalEventBus(configs BusConfigs) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalManager != nil {
		return nil
	}

	manager, err := NewEventBusManager(configs)
	if err != nil {
		globalInitErr = err
		return err
	}
	globalManager = manager
	globalInitErr = nil
	return nil
}

// GetGlobalEventBus returns the global event bus manager
func GetGlobalEventBus() *EventBusManager {
	globalMu.RLock()
	provider := defaultEventBusProvider
	globalMu.RUnlock()
	if provider != nil {
		if manager := provider(); manager != nil {
			return manager
		}
	}

	manager, err := ensureGlobalEventBus()
	if err != nil {
		log.NewHelper(log.DefaultLogger).Warnf("failed to initialize global event bus, using fallback manager: %v", err)
		return newFallbackEventBusManager()
	}
	return manager
}

// SetGlobalEventBus sets the global event bus manager
func SetGlobalEventBus(manager *EventBusManager) {
	globalMu.Lock()
	defer globalMu.Unlock()

	globalManager = manager
	globalInitErr = nil
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

func ensureGlobalEventBus() (*EventBusManager, error) {
	globalMu.RLock()
	manager := globalManager
	initErr := globalInitErr
	globalMu.RUnlock()

	if manager != nil {
		return manager, nil
	}

	globalMu.Lock()
	defer globalMu.Unlock()

	if globalManager != nil {
		return globalManager, nil
	}

	manager, err := NewEventBusManager(DefaultBusConfigs())
	if err != nil {
		if initErr != nil {
			err = fmt.Errorf("%w; latest retry failed: %v", initErr, err)
		}
		globalInitErr = err
		return nil, err
	}
	globalManager = manager
	globalInitErr = nil
	return globalManager, nil
}

func newFallbackEventBusManager() *EventBusManager {
	manager := &EventBusManager{
		buses:      make(map[BusType]*LynxEventBus),
		classifier: NewEventClassifier(),
		configs:    DefaultBusConfigs(),
		logger:     log.DefaultLogger,
	}
	manager.initBuses()
	return manager
}

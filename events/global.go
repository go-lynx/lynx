package events

import (
	"context"
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
	globalFallbackOnce      sync.Once
	globalFallbackManager   *EventBusManager
)

// SetDefaultEventBusProvider wires compatibility global access to an external owner such as LynxApp.
func SetDefaultEventBusProvider(provider func() *EventBusManager) {
	globalMu.Lock()
	defer globalMu.Unlock()
	defaultEventBusProvider = provider
}

// ClearDefaultEventBusProvider removes the compatibility provider used for
// process-wide default event bus lookups.
func ClearDefaultEventBusProvider() {
	SetDefaultEventBusProvider(nil)
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
//
// Deprecated: prefer passing an explicit *EventBusManager and using
// PublishEventWithManager, SubscribeWithManager, or SubscribeToWithManager.
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
		log.NewHelper(log.DefaultLogger).Warnf("failed to initialize global event bus, using shared fallback manager: %v", err)
		return getOrCreateFallbackEventBusManager()
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
//
// Deprecated: prefer PublishEventWithManager with an explicit *EventBusManager.
func PublishEvent(event LynxEvent) error {
	return GetGlobalEventBus().PublishEvent(event)
}

// PublishEventWithManager publishes an event via the provided event bus manager.
func PublishEventWithManager(manager *EventBusManager, event LynxEvent) error {
	if manager == nil {
		return fmt.Errorf("event bus manager is nil")
	}
	return manager.PublishEvent(event)
}

// Subscribe subscribes to events on a specific bus
//
// Deprecated: prefer SubscribeWithManager with an explicit *EventBusManager.
func Subscribe(busType BusType, handler func(LynxEvent)) error {
	return GetGlobalEventBus().Subscribe(busType, handler)
}

// SubscribeWithManager subscribes to events on a specific bus via the provided manager.
func SubscribeWithManager(manager *EventBusManager, busType BusType, handler func(LynxEvent)) error {
	if manager == nil {
		return fmt.Errorf("event bus manager is nil")
	}
	return manager.Subscribe(busType, handler)
}

// SubscribeTo subscribes to a specific event type
//
// Deprecated: prefer SubscribeToWithManager with an explicit *EventBusManager.
func SubscribeTo(eventType EventType, handler func(LynxEvent)) error {
	return GetGlobalEventBus().SubscribeTo(eventType, handler)
}

// SubscribeToWithManager subscribes to a specific event type via the provided manager.
func SubscribeToWithManager(manager *EventBusManager, eventType EventType, handler func(LynxEvent)) error {
	if manager == nil {
		return fmt.Errorf("event bus manager is nil")
	}
	return manager.SubscribeTo(eventType, handler)
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

		// Publish shutdown event to all buses with timeout.
		// shutdownDone is a close-only notification channel: the goroutine calls
		// close(shutdownDone) exactly once (via sync.Once) to signal completion.
		// A context prevents two receivers from racing over one timer channel.
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancelShutdown()
		shutdownDone := make(chan struct{})
		var shutdownOnce sync.Once
		manager := globalManager

		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[lynx-error] panic in shutdown event goroutine: %v\n", r)
				}
				// Signal completion exactly once by closing the channel.
				shutdownOnce.Do(func() { close(shutdownDone) })
			}()
			for busType := range manager.buses {
				if bus := manager.GetBus(busType); bus != nil {
					select {
					case <-shutdownCtx.Done():
						return
					default:
						bus.Publish(shutdownEvent)
					}
				}
			}
		}()

		// Wait for shutdown event to be processed or timeout.
		select {
		case <-shutdownDone:
			// Shutdown event published to all buses.
		case <-shutdownCtx.Done():
			// Timeout reached, proceed with closing.
		}

		return manager.Close()
	}

	if globalFallbackManager != nil {
		return globalFallbackManager.Close()
	}
	return nil
}

// SetGlobalLogger sets the logger for the global event bus manager
//
// Deprecated: prefer calling SetLogger on an explicit *EventBusManager.
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

func getOrCreateFallbackEventBusManager() *EventBusManager {
	globalFallbackOnce.Do(func() {
		manager := &EventBusManager{
			buses:      make(map[BusType]*LynxEventBus),
			classifier: NewEventClassifier(),
			configs:    DefaultBusConfigs(),
			logger:     log.DefaultLogger,
		}
		manager.initBuses()
		globalFallbackManager = manager
	})
	return globalFallbackManager
}

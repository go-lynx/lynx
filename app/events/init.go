package events

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
)

// Init initializes the global event bus system
func Init(configs BusConfigs) error {
	// Initialize global event bus manager
	if err := InitGlobalEventBus(configs); err != nil {
		return fmt.Errorf("failed to initialize global event bus: %w", err)
	}

	// Setup plugin event bus adapter
	SetupPluginEventBusAdapter()

	return nil
}

// InitWithLogger initializes the global event bus system with a logger
func InitWithLogger(configs BusConfigs, logger log.Logger) error {
	// Initialize global event bus manager
	if err := InitGlobalEventBus(configs); err != nil {
		return fmt.Errorf("failed to initialize global event bus: %w", err)
	}

	// Set logger for all buses
	SetGlobalLogger(logger)

	// Setup plugin event bus adapter
	SetupPluginEventBusAdapter()

	return nil
}

// InitWithDefaultConfig initializes the global event bus system with default configuration
func InitWithDefaultConfig() error {
	return Init(DefaultBusConfigs())
}

// InitWithDefaultConfigAndLogger initializes the global event bus system with default configuration and logger
func InitWithDefaultConfigAndLogger(logger log.Logger) error {
	return InitWithLogger(DefaultBusConfigs(), logger)
}

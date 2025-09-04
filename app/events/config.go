package events

import (
	"fmt"
	"time"
)

// ErrorCallback is called when events are dropped or failed
type ErrorCallback func(event LynxEvent, reason string, err error)

// BusConfig represents configuration for a single event bus
type BusConfig struct {
	MaxQueue      int           `yaml:"max_queue" json:"max_queue"`
	FlushInterval time.Duration `yaml:"flush_interval" json:"flush_interval"`
	Priority      Priority      `yaml:"priority" json:"priority"`
	EnableHistory bool          `yaml:"enable_history" json:"enable_history"`
	EnableMetrics bool          `yaml:"enable_metrics" json:"enable_metrics"`
	// MaxRetries controls panic-based retry attempts in handlers
	MaxRetries int `yaml:"max_retries" json:"max_retries"`
	// HistorySize controls the capacity of in-memory history when enabled
	HistorySize int `yaml:"history_size" json:"history_size"`
	// BatchSize controls how many events are aggregated per dispatch cycle
	BatchSize int `yaml:"batch_size" json:"batch_size"`
	// WorkerCount controls number of consumer workers (reserved for future)
	WorkerCount int `yaml:"worker_count" json:"worker_count"`

	// Degradation settings
	EnableDegradation    bool            `yaml:"enable_degradation" json:"enable_degradation"`
	DegradationThreshold int             `yaml:"degradation_threshold" json:"degradation_threshold"` // Queue usage percentage
	DegradationMode      DegradationMode `yaml:"degradation_mode" json:"degradation_mode"`

	// Throttling settings
	EnableThrottling bool `yaml:"enable_throttling" json:"enable_throttling"`
	ThrottleRate     int  `yaml:"throttle_rate" json:"throttle_rate"`   // Events per second
	ThrottleBurst    int  `yaml:"throttle_burst" json:"throttle_burst"` // Burst size

	// Error handling
	ErrorCallback ErrorCallback `yaml:"-" json:"-"` // Not serializable
}

// DegradationMode represents the degradation strategy
type DegradationMode string

const (
	DegradationModeDrop     DegradationMode = "drop"     // Drop new events
	DegradationModePause    DegradationMode = "pause"    // Pause processing
	DegradationModeThrottle DegradationMode = "throttle" // Throttle processing
)

// DefaultBusConfig returns default configuration for a bus
func DefaultBusConfig() BusConfig {
	return BusConfig{
		MaxQueue:             10000,
		FlushInterval:        100 * time.Microsecond,
		Priority:             PriorityNormal,
		EnableHistory:        true,
		EnableMetrics:        true,
		MaxRetries:           0,
		HistorySize:          1000,
		BatchSize:            32,
		WorkerCount:          1,
		EnableDegradation:    true,
		DegradationThreshold: 90, // 90% queue usage
		DegradationMode:      DegradationModeDrop,
		EnableThrottling:     false,
		ThrottleRate:         1000, // 1000 events per second
		ThrottleBurst:        100,  // 100 events burst
	}
}

// BusConfigs represents configuration for all event buses
type BusConfigs struct {
	Plugin   BusConfig `yaml:"plugin" json:"plugin"`
	System   BusConfig `yaml:"system" json:"system"`
	Business BusConfig `yaml:"business" json:"business"`
	Health   BusConfig `yaml:"health" json:"health"`
	Config   BusConfig `yaml:"config" json:"config"`
	Resource BusConfig `yaml:"resource" json:"resource"`
	Security BusConfig `yaml:"security" json:"security"`
	Metrics  BusConfig `yaml:"metrics" json:"metrics"`
}

// DefaultBusConfigs returns default configuration for all buses
func DefaultBusConfigs() BusConfigs {
	return BusConfigs{
		Plugin: BusConfig{
			MaxQueue:      10000,
			FlushInterval: 100 * time.Microsecond,
			Priority:      PriorityHigh,
			EnableHistory: true,
			EnableMetrics: true,
			MaxRetries:    1,
			HistorySize:   2000,
			BatchSize:     32,
			WorkerCount:   1,
		},
		System: BusConfig{
			MaxQueue:      5000,
			FlushInterval: 200 * time.Microsecond,
			Priority:      PriorityHigh,
			EnableHistory: false,
			EnableMetrics: true,
			MaxRetries:    1,
			HistorySize:   500,
			BatchSize:     16,
			WorkerCount:   1,
		},
		Business: BusConfig{
			MaxQueue:      20000,
			FlushInterval: 50 * time.Microsecond,
			Priority:      PriorityNormal,
			EnableHistory: true,
			EnableMetrics: true,
			MaxRetries:    0,
			HistorySize:   2000,
			BatchSize:     64,
			WorkerCount:   1,
		},
		Health: BusConfig{
			MaxQueue:      1000,
			FlushInterval: 500 * time.Microsecond,
			Priority:      PriorityCritical,
			EnableHistory: true,
			EnableMetrics: true,
			MaxRetries:    0,
			HistorySize:   200,
			BatchSize:     8,
			WorkerCount:   1,
		},
		Config: BusConfig{
			MaxQueue:      2000,
			FlushInterval: 300 * time.Microsecond,
			Priority:      PriorityHigh,
			EnableHistory: true,
			EnableMetrics: true,
			MaxRetries:    0,
			HistorySize:   500,
			BatchSize:     16,
			WorkerCount:   1,
		},
		Resource: BusConfig{
			MaxQueue:      3000,
			FlushInterval: 250 * time.Microsecond,
			Priority:      PriorityHigh,
			EnableHistory: true,
			EnableMetrics: true,
			MaxRetries:    0,
			HistorySize:   500,
			BatchSize:     16,
			WorkerCount:   1,
		},
		Security: BusConfig{
			MaxQueue:      1000,
			FlushInterval: 100 * time.Microsecond,
			Priority:      PriorityCritical,
			EnableHistory: true,
			EnableMetrics: true,
			MaxRetries:    1,
			HistorySize:   500,
			BatchSize:     8,
			WorkerCount:   1,
		},
		Metrics: BusConfig{
			MaxQueue:      5000,
			FlushInterval: 1 * time.Millisecond,
			Priority:      PriorityLow,
			EnableHistory: false,
			EnableMetrics: true,
			MaxRetries:    0,
			HistorySize:   200,
			BatchSize:     16,
			WorkerCount:   1,
		},
	}
}

// GetBusConfig returns configuration for a specific bus type
func (c *BusConfigs) GetBusConfig(busType BusType) BusConfig {
	switch busType {
	case BusTypePlugin:
		return c.Plugin
	case BusTypeSystem:
		return c.System
	case BusTypeBusiness:
		return c.Business
	case BusTypeHealth:
		return c.Health
	case BusTypeConfig:
		return c.Config
	case BusTypeResource:
		return c.Resource
	case BusTypeSecurity:
		return c.Security
	case BusTypeMetrics:
		return c.Metrics
	default:
		return DefaultBusConfig()
	}
}

// Validate validates the bus configurations
func (c *BusConfigs) Validate() error {
	// Validate each bus configuration
	buses := []struct {
		name   string
		config BusConfig
	}{
		{"plugin", c.Plugin},
		{"system", c.System},
		{"business", c.Business},
		{"health", c.Health},
		{"config", c.Config},
		{"resource", c.Resource},
		{"security", c.Security},
		{"metrics", c.Metrics},
	}

	for _, bus := range buses {
		if err := c.validateBusConfig(bus.name, bus.config); err != nil {
			return err
		}
	}

	return nil
}

// validateBusConfig validates a single bus configuration
func (c *BusConfigs) validateBusConfig(name string, config BusConfig) error {
	if config.MaxQueue <= 0 {
		return fmt.Errorf("bus %s: max_queue must be positive", name)
	}
	if config.FlushInterval <= 0 {
		return fmt.Errorf("bus %s: flush_interval must be positive", name)
	}
	if config.Priority < PriorityLow || config.Priority > PriorityCritical {
		return fmt.Errorf("bus %s: invalid priority value", name)
	}
	if config.MaxRetries < 0 {
		return fmt.Errorf("bus %s: max_retries must be >= 0", name)
	}
	if config.EnableHistory && config.HistorySize <= 0 {
		return fmt.Errorf("bus %s: history_size must be positive when enable_history is true", name)
	}
	if config.BatchSize <= 0 {
		return fmt.Errorf("bus %s: batch_size must be >= 1", name)
	}
	if config.WorkerCount <= 0 {
		return fmt.Errorf("bus %s: worker_count must be >= 1", name)
	}
	if config.EnableThrottling {
		if config.ThrottleRate <= 0 {
			return fmt.Errorf("bus %s: throttle_rate must be positive when throttling is enabled", name)
		}
		if config.ThrottleBurst <= 0 {
			return fmt.Errorf("bus %s: throttle_burst must be positive when throttling is enabled", name)
		}
	}
	return nil
}

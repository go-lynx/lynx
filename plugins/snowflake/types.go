package snowflake

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/redis/go-redis/v9"

	pb "github.com/go-lynx/lynx/plugins/snowflake/conf"
)

// PlugSnowflake represents a Snowflake ID generator plugin instance
type PlugSnowflake struct {
	// Inherits from base plugin
	*plugins.BasePlugin
	// Snowflake configuration
	conf *pb.Snowflake
	// Redis client for worker ID registration
	redisClient redis.UniversalClient
	// Worker ID manager
	workerManager *WorkerIDManager
	// ID generator
	generator *Generator
	// Metrics collection
	metrics *Metrics
	// Security manager
	securityManager *SecurityManager
	// Shutdown channel
	shutdownCh chan struct{}
	// Ensure shutdown channel is closed only once
	shutdownOnce sync.Once
	// Wait group for goroutines
	wg sync.WaitGroup
	// Mutex for thread safety
	mu sync.RWMutex
	// Plugin runtime
	runtime plugins.Runtime
	// Logger instance
	logger log.Logger
}

// WorkerIDManager manages worker ID registration and heartbeat
// Uses Redis INCR for lock-free worker ID allocation
type WorkerIDManager struct {
	redisClient       redis.UniversalClient
	datacenterID      int64
	keyPrefix         string
	ttl               time.Duration
	heartbeatInterval time.Duration
	// Worker state
	workerID int64
	// Heartbeat lifecycle management
	heartbeatCtx     context.Context
	heartbeatCancel  context.CancelFunc
	heartbeatRunning bool
	// Registration info preserved for heartbeat
	registerTime time.Time
	instanceID   string
	localIP      string // Local IP address for troubleshooting
	// Health state - used to stop ID generation when heartbeat fails
	healthy int32 // atomic: 1=healthy, 0=unhealthy
	// Mutex for state management
	mu sync.RWMutex
}

// Generator generates snowflake IDs
type Generator struct {
	// Configuration
	datacenterID int64
	workerID     int64
	customEpoch  int64
	workerIDBits int64
	sequenceBits int64

	// Bit shifts
	timestampShift  int64
	datacenterShift int64
	workerShift     int64

	// Bit masks
	maxDatacenterID int64
	maxWorkerID     int64
	maxSequence     int64

	// State
	lastTimestamp int64
	sequence      int64

	// Statistics
	generatedCount     int64
	clockBackwardCount int64

	// Clock drift protection
	enableClockDriftProtection bool
	maxClockDrift              time.Duration
	clockDriftAction           string
	lastClockCheck             time.Time

	// Sequence cache for performance
	enableSequenceCache bool
	sequenceCache       []int64
	cacheIndex          int
	cacheSize           int

	// Shutdown state
	isShuttingDown bool

	// Metrics collection
	metrics *Metrics

	// Mutex for thread safety
	mu sync.Mutex
}

// Metrics collects detailed metrics for the snowflake generator
type Metrics struct {
	// ID generation metrics
	IDsGenerated      int64
	ClockDriftEvents  int64
	WorkerIDConflicts int64
	SequenceOverflows int64

	// Performance metrics
	GenerationLatency time.Duration
	AverageLatency    time.Duration
	P95Latency        time.Duration
	P99Latency        time.Duration
	MaxLatency        time.Duration
	MinLatency        time.Duration

	// Cache metrics
	CacheHitRate float64
	CacheHits    int64
	CacheMisses  int64
	CacheRefills int64

	// Throughput metrics
	IDGenerationRate   float64 // IDs per second
	PeakGenerationRate float64 // Peak IDs per second

	// Error metrics
	GenerationErrors int64
	RedisErrors      int64
	TimeoutErrors    int64
	ValidationErrors int64

	// Connection metrics
	RedisConnectionPool int
	ActiveConnections   int
	IdleConnections     int

	// Timing metrics
	StartTime          time.Time
	LastGenerationTime time.Time
	UptimeDuration     time.Duration

	// Latency histogram for detailed analysis
	LatencyHistogram map[string]int64 // e.g., "0-1ms": count, "1-5ms": count

	mu sync.RWMutex
}

// ClockDriftError represents a clock drift error
type ClockDriftError struct {
	CurrentTime   time.Time
	LastTimestamp time.Time
	Drift         time.Duration
}

func (e *ClockDriftError) Error() string {
	return fmt.Sprintf("clock drift detected: current=%v, last=%v, drift=%v",
		e.CurrentTime, e.LastTimestamp, e.Drift)
}

// WorkerIDConflictError represents a worker ID conflict error
type WorkerIDConflictError struct {
	WorkerID     int64
	DatacenterID int64
	ConflictWith string
}

func (e *WorkerIDConflictError) Error() string {
	return fmt.Sprintf("worker ID conflict: worker_id=%d, datacenter_id=%d, conflict_with=%s",
		e.WorkerID, e.DatacenterID, e.ConflictWith)
}

// SID represents a generated snowflake ID with metadata
type SID struct {
	ID           int64     `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	DatacenterID int64     `json:"datacenter_id"`
	WorkerID     int64     `json:"worker_id"`
	Sequence     int64     `json:"sequence"`
}

// IDComponents represents the components of a snowflake ID
type IDComponents struct {
	Timestamp    int64 `json:"timestamp"`
	DatacenterID int64 `json:"datacenter_id"`
	WorkerID     int64 `json:"worker_id"`
	Sequence     int64 `json:"sequence"`
}

// GeneratorStats represents statistics about the generator
type GeneratorStats struct {
	WorkerID           int64 `json:"worker_id"`
	DatacenterID       int64 `json:"datacenter_id"`
	GeneratedCount     int64 `json:"generated_count"`
	ClockBackwardCount int64 `json:"clock_backward_count"`
	LastGeneratedTime  int64 `json:"last_generated_time"`
}

// Constants for default configuration
const (
	DefaultDatacenterID     = 1
	DefaultWorkerID         = 1
	DefaultTimestampBits    = 41
	DefaultDatacenterBits   = 5
	DefaultWorkerBits       = 5
	DefaultSequenceBits     = 12
	DefaultEpoch            = 1609459200000 // 2021-01-01 00:00:00 UTC in milliseconds
	DefaultMaxClockBackward = 5000          // 5 seconds in milliseconds

	DefaultRedisKeyPrefix    = "snowflake:"
	DefaultWorkerIDTTL       = 30 * time.Second
	DefaultHeartbeatInterval = 10 * time.Second
)

const (
	// DefaultWorkerIDBits Default A bit of allocation
	DefaultWorkerIDBits = 5

	// DefaultMaxClockDrift Default timing
	DefaultMaxClockDrift      = 5 * time.Second
	DefaultClockCheckInterval = 1 * time.Second

	// DefaultSequenceCacheSize Default cache size
	DefaultSequenceCacheSize = 1000

	// WorkerIDLockKey Redis key patterns
	WorkerIDLockKey     = "lynx:snowflake:lock:worker_id"
	WorkerIDRegistryKey = "lynx:snowflake:registry"

	// ClockDriftActionWait Clock drift actions
	ClockDriftActionWait   = "wait"
	ClockDriftActionError  = "error"
	ClockDriftActionIgnore = "ignore"
)

// NewSnowflakePlugin creates a new snowflake plugin instance
func NewSnowflakePlugin() *PlugSnowflake {
	return &PlugSnowflake{
		shutdownCh: make(chan struct{}),
	}
}

// Plugin interface implementation

// ID returns the plugin ID
func (p *PlugSnowflake) ID() string {
	return "snowflake"
}

// Description returns the plugin description
func (p *PlugSnowflake) Description() string {
	return "Snowflake ID generator plugin for distributed unique ID generation"
}

// Weight returns the plugin weight for loading order
func (p *PlugSnowflake) Weight() int {
	return 100
}

// UpdateConfiguration updates the plugin configuration
func (p *PlugSnowflake) UpdateConfiguration(config interface{}) error {
	if conf, ok := config.(*pb.Snowflake); ok {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.conf = conf
		return nil
	}
	return fmt.Errorf("invalid configuration type for snowflake plugin")
}

// Lifecycle interface implementation

// Initialize initializes the plugin
func (p *PlugSnowflake) Initialize(plugin plugins.Plugin, runtime plugins.Runtime) error {
	p.runtime = runtime
	p.logger = runtime.GetLogger()

	// Get configuration
	conf := &pb.Snowflake{}
	config := runtime.GetConfig()
	if config != nil {
		// Load protobuf configuration using the config prefix
		if err := config.Value(ConfPrefix).Scan(conf); err != nil {
			// If config loading fails, use default values and log warning
			log.NewHelper(p.logger).Warnf("failed to load snowflake configuration: %v, using defaults", err)
			conf = &pb.Snowflake{
				DatacenterId:               0,
				WorkerId:                   0,
				AutoRegisterWorkerId:       true,
				RedisKeyPrefix:             DefaultRedisKeyPrefix,
				EnableClockDriftProtection: true,
				ClockDriftAction:           "wait",
				EnableSequenceCache:        true,
				SequenceCacheSize:          1000,
				EnableMetrics:              true,
				RedisPluginName:            "redis",
				RedisDb:                    0,
				CustomEpoch:                DefaultEpoch,
				WorkerIdBits:               DefaultWorkerBits,
				SequenceBits:               DefaultSequenceBits,
			}
		}
	}
	p.conf = conf

	// Initialize Redis client if auto registration is enabled
	if conf.AutoRegisterWorkerId {
		redisPluginName := conf.RedisPluginName
		if redisPluginName == "" {
			redisPluginName = "redis"
		}

		// Try to get Redis client from shared resources
		if redisResource, err := runtime.GetSharedResource(redisPluginName); err == nil {
			if redisClient, ok := redisResource.(redis.UniversalClient); ok {
				p.redisClient = redisClient
				log.NewHelper(p.logger).Infof("successfully connected to Redis plugin: %s", redisPluginName)
			} else {
				log.NewHelper(p.logger).Warnf("Redis resource is not UniversalClient type, disabling auto worker ID registration")
				conf.AutoRegisterWorkerId = false
			}
		} else {
			log.NewHelper(p.logger).Warnf("failed to get Redis client from plugin %s: %v, disabling auto worker ID registration", redisPluginName, err)
			conf.AutoRegisterWorkerId = false
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Initialize worker manager with proper configuration
	keyPrefix := conf.RedisKeyPrefix
	if keyPrefix == "" {
		keyPrefix = DefaultRedisKeyPrefix
	}

	ttl := DefaultWorkerIDTTL
	if conf.WorkerIdTtl != nil {
		ttl = conf.WorkerIdTtl.AsDuration()
	}

	heartbeatInterval := DefaultHeartbeatInterval
	if conf.HeartbeatInterval != nil {
		heartbeatInterval = conf.HeartbeatInterval.AsDuration()
	}

	p.workerManager = &WorkerIDManager{
		redisClient:       p.redisClient,
		workerID:          int64(conf.WorkerId),
		datacenterID:      int64(conf.DatacenterId),
		keyPrefix:         keyPrefix,
		ttl:               ttl,
		heartbeatInterval: heartbeatInterval,
	}

	// Initialize generator with proper configuration
	generatorConfig := &GeneratorConfig{
		CustomEpoch:                conf.CustomEpoch,
		DatacenterIDBits:           DefaultDatacenterBits, // Fixed to 5 bits for datacenter ID (0-31)
		WorkerIDBits:               int(conf.WorkerIdBits),
		SequenceBits:               int(conf.SequenceBits),
		EnableClockDriftProtection: conf.EnableClockDriftProtection,
		MaxClockDrift:              time.Duration(5 * time.Second), // Default value
		ClockDriftAction:           conf.ClockDriftAction,
		EnableSequenceCache:        conf.EnableSequenceCache,
		SequenceCacheSize:          int(conf.SequenceCacheSize),
	}

	// Set default values if not specified
	if generatorConfig.CustomEpoch == 0 {
		generatorConfig.CustomEpoch = DefaultEpoch
	}
	if generatorConfig.WorkerIDBits == 0 {
		generatorConfig.WorkerIDBits = DefaultWorkerBits
	}
	if generatorConfig.SequenceBits == 0 {
		generatorConfig.SequenceBits = DefaultSequenceBits
	}
	if generatorConfig.SequenceCacheSize == 0 {
		generatorConfig.SequenceCacheSize = 1000
	}

	// Handle clock drift configuration
	if conf.MaxClockDrift != nil {
		generatorConfig.MaxClockDrift = conf.MaxClockDrift.AsDuration()
	}

	var err error
	p.generator, err = NewSnowflakeGeneratorCore(int64(conf.DatacenterId), int64(conf.WorkerId), generatorConfig)
	if err != nil {
		return fmt.Errorf("failed to create snowflake generator: %w", err)
	}

	// Initialize metrics
	p.metrics = &Metrics{
		StartTime: time.Now(),
	}

	return nil
}

// Start starts the plugin
func (p *PlugSnowflake) Start(plugin plugins.Plugin) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Note: Worker manager heartbeat is already started in RegisterWorkerID
	// No additional goroutine needed here as heartbeat runs in workerManager

	return nil
}

// Stop stops the plugin
func (p *PlugSnowflake) Stop(plugin plugins.Plugin) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Signal shutdown - use sync.Once to ensure idempotent closure
	p.shutdownOnce.Do(func() {
		close(p.shutdownCh)
	})

	// Stop worker manager heartbeat and unregister
	if p.workerManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := p.workerManager.UnregisterWorkerID(ctx); err != nil {
			log.NewHelper(p.logger).Warnf("failed to unregister worker ID during stop: %v", err)
		}
	}

	// Shutdown generator
	if p.generator != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.generator.Shutdown(ctx); err != nil {
			log.NewHelper(p.logger).Warnf("failed to shutdown generator: %v", err)
		}
	}

	// Wait for goroutines to finish
	p.wg.Wait()

	return nil
}

// Status returns the plugin status
func (p *PlugSnowflake) Status(plugin plugins.Plugin) plugins.PluginStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.generator != nil {
		return plugins.StatusActive
	}
	return plugins.StatusInactive
}

// LifecycleSteps interface implementation

// InitializeResources initializes plugin resources
func (p *PlugSnowflake) InitializeResources(rt plugins.Runtime) error {
	// Initialize resources
	return nil
}

// StartupTasks performs startup tasks
func (p *PlugSnowflake) StartupTasks() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Auto-register worker ID if enabled and not already registered
	if p.conf != nil && p.conf.AutoRegisterWorkerId && p.workerManager != nil && p.redisClient != nil {
		// Check if worker ID is already set (manual configuration)
		if p.conf.WorkerId > 0 {
			// Try to register the specific worker ID
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := p.workerManager.RegisterSpecificWorkerID(ctx, int64(p.conf.WorkerId)); err != nil {
				log.NewHelper(p.logger).Warnf("failed to register specific worker ID %d: %v, trying auto-register", p.conf.WorkerId, err)
				// Fall back to auto-register
				maxWorkerID := int64((1 << p.conf.WorkerIdBits) - 1)
				if maxWorkerID == 0 {
					maxWorkerID = 31 // Default max worker ID
				}
				workerID, err := p.workerManager.RegisterWorkerID(ctx, maxWorkerID)
				if err != nil {
					return fmt.Errorf("failed to auto-register worker ID: %w", err)
				}
				// Update generator with new worker ID (thread-safe)
				if p.generator != nil {
					p.generator.mu.Lock()
					p.generator.workerID = workerID
					p.generator.mu.Unlock()
				}
				log.NewHelper(p.logger).Infof("auto-registered worker ID: %d", workerID)
			} else {
				log.NewHelper(p.logger).Infof("registered specific worker ID: %d", p.conf.WorkerId)
			}
		} else {
			// Auto-register worker ID
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			maxWorkerID := int64((1 << p.conf.WorkerIdBits) - 1)
			if maxWorkerID == 0 {
				maxWorkerID = 31 // Default max worker ID
			}
			workerID, err := p.workerManager.RegisterWorkerID(ctx, maxWorkerID)
			if err != nil {
				return fmt.Errorf("failed to auto-register worker ID: %w", err)
			}
			// Update generator with new worker ID (thread-safe)
			if p.generator != nil {
				p.generator.mu.Lock()
				p.generator.workerID = workerID
				p.generator.mu.Unlock()
			}
			log.NewHelper(p.logger).Infof("auto-registered worker ID: %d", workerID)
		}
	}

	return nil
}

// CleanupTasks performs cleanup tasks
func (p *PlugSnowflake) CleanupTasks() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Unregister worker ID if registered
	if p.workerManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := p.workerManager.UnregisterWorkerID(ctx); err != nil {
			log.NewHelper(p.logger).Warnf("failed to unregister worker ID during cleanup: %v", err)
			// Don't return error, as this is cleanup
		} else {
			log.NewHelper(p.logger).Infof("unregistered worker ID during cleanup")
		}
	}

	// Shutdown generator
	if p.generator != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.generator.Shutdown(ctx); err != nil {
			log.NewHelper(p.logger).Warnf("failed to shutdown generator: %v", err)
		}
	}

	return nil
}

// CheckHealth checks plugin health
func (p *PlugSnowflake) CheckHealth() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.generator == nil {
		return fmt.Errorf("snowflake generator not initialized")
	}

	return nil
}

// GetHealth implements the HealthCheck interface
func (p *PlugSnowflake) GetHealth() plugins.HealthReport {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := "healthy"
	details := make(map[string]any)
	message := "Snowflake ID generator is operating normally"

	// Check generator status
	if p.generator == nil {
		status = "unhealthy"
		message = "Snowflake generator not initialized"
		details["generator_status"] = "not_initialized"
	} else {
		details["generator_status"] = "initialized"
		details["worker_id"] = p.generator.workerID
		details["datacenter_id"] = p.generator.datacenterID
		details["custom_epoch"] = p.generator.customEpoch
		details["generated_count"] = atomic.LoadInt64(&p.generator.generatedCount)
		details["clock_backward_count"] = atomic.LoadInt64(&p.generator.clockBackwardCount)
		details["is_shutting_down"] = p.generator.isShuttingDown

		// Check for clock drift issues
		if p.generator.clockBackwardCount > 0 {
			status = "degraded"
			message = "Clock backward events detected"
		}
	}

	// Check Redis connection status
	if p.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := p.redisClient.Ping(ctx).Err(); err != nil {
			if status == "healthy" {
				status = "degraded"
			}
			message = "Redis connection issues detected"
			details["redis_status"] = "unhealthy"
			details["redis_error"] = err.Error()
		} else {
			details["redis_status"] = "healthy"
		}
	} else {
		details["redis_status"] = "not_configured"
	}

	// Check worker manager status
	if p.workerManager != nil {
		details["worker_manager_status"] = "active"
		details["worker_manager_worker_id"] = p.workerManager.workerID
		details["worker_manager_datacenter_id"] = p.workerManager.datacenterID
		details["worker_manager_key_prefix"] = p.workerManager.keyPrefix
		details["worker_manager_ttl"] = p.workerManager.ttl.String()
		details["worker_manager_heartbeat_interval"] = p.workerManager.heartbeatInterval.String()
	} else {
		details["worker_manager_status"] = "not_configured"
	}

	// Add metrics information if available
	if p.metrics != nil {
		p.metrics.mu.RLock()
		details["metrics"] = map[string]any{
			"ids_generated":        p.metrics.IDsGenerated,
			"clock_drift_events":   p.metrics.ClockDriftEvents,
			"worker_id_conflicts":  p.metrics.WorkerIDConflicts,
			"sequence_overflows":   p.metrics.SequenceOverflows,
			"generation_errors":    p.metrics.GenerationErrors,
			"redis_errors":         p.metrics.RedisErrors,
			"timeout_errors":       p.metrics.TimeoutErrors,
			"validation_errors":    p.metrics.ValidationErrors,
			"id_generation_rate":   p.metrics.IDGenerationRate,
			"peak_generation_rate": p.metrics.PeakGenerationRate,
			"uptime_duration":      p.metrics.UptimeDuration.String(),
			"last_generation_time": p.metrics.LastGenerationTime.Format(time.RFC3339),
		}
		p.metrics.mu.RUnlock()

		// Check for high error rates
		totalOperations := p.metrics.IDsGenerated + p.metrics.GenerationErrors
		if totalOperations > 0 {
			errorRate := float64(p.metrics.GenerationErrors) / float64(totalOperations)
			details["error_rate"] = errorRate

			if errorRate > 0.1 { // More than 10% error rate
				status = "degraded"
				message = "High error rate detected"
			}
		}
	}

	// Add configuration information
	if p.conf != nil {
		details["configuration"] = map[string]any{
			"datacenter_id":           p.conf.DatacenterId,
			"worker_id":               p.conf.WorkerId,
			"custom_epoch":            p.conf.CustomEpoch,
			"auto_register_worker_id": p.conf.AutoRegisterWorkerId,
			"redis_plugin_name":       p.conf.RedisPluginName,
			"redis_key_prefix":        p.conf.RedisKeyPrefix,
			"worker_id_ttl":           p.conf.WorkerIdTtl,
			"heartbeat_interval":      p.conf.HeartbeatInterval,
			"enable_metrics":          p.conf.EnableMetrics,
			"clock_drift_protection":  p.conf.EnableClockDriftProtection,
			"sequence_cache":          p.conf.EnableSequenceCache,
			"max_clock_drift":         p.conf.MaxClockDrift,
			"clock_check_interval":    p.conf.ClockCheckInterval,
			"clock_drift_action":      p.conf.ClockDriftAction,
			"sequence_cache_size":     p.conf.SequenceCacheSize,
			"redis_db":                p.conf.RedisDb,
			"worker_id_bits":          p.conf.WorkerIdBits,
			"sequence_bits":           p.conf.SequenceBits,
		}
	}

	return plugins.HealthReport{
		Status:    status,
		Details:   details,
		Timestamp: time.Now().Unix(),
		Message:   message,
	}
}

// DependencyAware interface implementation

// GetDependencies returns plugin dependencies
func (p *PlugSnowflake) GetDependencies() []plugins.Dependency {
	var deps []plugins.Dependency
	if p.conf.RedisPluginName != "" {
		deps = append(deps, plugins.Dependency{
			ID:          "redis",
			Name:        "Redis",
			Type:        plugins.DependencyTypeOptional,
			Required:    false,
			Description: "Redis client for worker ID management",
		})
	}
	return deps
}

// Snowflake specific methods

// GenerateID generates a new snowflake ID
// Note: Generator.GenerateID() is internally thread-safe with its own mutex,
// so we only need to protect the nil check here to avoid double locking overhead.
func (p *PlugSnowflake) GenerateID() (int64, error) {
	// Quick nil check with read lock
	p.mu.RLock()
	generator := p.generator
	workerManager := p.workerManager
	p.mu.RUnlock()

	if generator == nil {
		return 0, fmt.Errorf("snowflake generator not initialized")
	}

	// Check worker manager health to prevent ID duplication
	// when heartbeat fails and worker ID may have been taken by another instance
	if workerManager != nil && !workerManager.IsHealthy() {
		return 0, fmt.Errorf("worker ID registration unhealthy, cannot generate ID safely")
	}

	// Generator.GenerateID() has its own mutex protection
	return generator.GenerateID()
}

// GenerateIDWithMetadata generates a new snowflake ID with metadata
func (p *PlugSnowflake) GenerateIDWithMetadata() (int64, *SID, error) {
	// Quick nil check with read lock
	p.mu.RLock()
	generator := p.generator
	workerManager := p.workerManager
	p.mu.RUnlock()

	if generator == nil {
		return 0, nil, fmt.Errorf("snowflake generator not initialized")
	}

	// Check worker manager health to prevent ID duplication
	if workerManager != nil && !workerManager.IsHealthy() {
		return 0, nil, fmt.Errorf("worker ID registration unhealthy, cannot generate ID safely")
	}

	// Generator methods have their own mutex protection
	return generator.GenerateIDWithMetadata()
}

// ParseID parses a snowflake ID into its components
func (p *PlugSnowflake) ParseID(id int64) (*SID, error) {
	// Quick nil check with read lock
	p.mu.RLock()
	generator := p.generator
	p.mu.RUnlock()

	if generator == nil {
		return nil, fmt.Errorf("snowflake generator not initialized")
	}

	// ParseID is read-only and safe
	return generator.ParseID(id)
}

// GetGenerator returns the snowflake generator instance
func (p *PlugSnowflake) GetGenerator() *Generator {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.generator
}

package snowflake

import (
	"fmt"
	"sync"
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
	generator *SnowflakeGenerator
	// Metrics collection
	metrics *SnowflakeMetrics
	// Shutdown channel
	shutdownCh chan struct{}
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
type WorkerIDManager struct {
	redisClient     redis.UniversalClient
	workerID        int64
	datacenterID    int64
	keyPrefix       string
	ttl             time.Duration
	heartbeatInterval time.Duration
	shutdownCh      chan struct{}
	mu              sync.RWMutex
}

// SnowflakeGenerator generates snowflake IDs
type SnowflakeGenerator struct {
	// Configuration
	datacenterID    int64
	workerID        int64
	customEpoch     int64
	workerIDBits    int64
	sequenceBits    int64
	
	// Bit shifts
	timestampShift  int64
	datacenterShift int64
	workerShift     int64
	
	// Bit masks
	maxDatacenterID int64
	maxWorkerID     int64
	maxSequence     int64
	
	// State
	lastTimestamp   int64
	sequence        int64
	
	// Statistics
	generatedCount     int64
	clockBackwardCount int64
	
	// Clock drift protection
	enableClockDriftProtection bool
	maxClockDrift             time.Duration
	clockDriftAction          string
	lastClockCheck            time.Time
	
	// Sequence cache for performance
	enableSequenceCache bool
	sequenceCache       []int64
	cacheIndex          int
	cacheSize           int
	
	// Shutdown state
	isShuttingDown bool
	
	// Mutex for thread safety
	mu sync.Mutex
}

// SnowflakeMetrics collects metrics for the snowflake generator
type SnowflakeMetrics struct {
	// ID generation metrics
	IDsGenerated      int64
	ClockDriftEvents  int64
	WorkerIDConflicts int64
	SequenceOverflows int64
	
	// Performance metrics
	GenerationLatency time.Duration
	CacheHitRate      float64
	
	// Error metrics
	GenerationErrors int64
	RedisErrors      int64
	
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

// SnowflakeID represents a generated snowflake ID with metadata
type SnowflakeID struct {
	ID           int64     `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	DatacenterID int64     `json:"datacenter_id"`
	WorkerID     int64     `json:"worker_id"`
	Sequence     int64     `json:"sequence"`
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
	// Default configuration values
	DefaultDatacenterID     = 1
	DefaultWorkerID         = 1
	DefaultTimestampBits    = 41
	DefaultDatacenterBits   = 5
	DefaultWorkerBits       = 5
	DefaultSequenceBits     = 12
	DefaultEpoch            = 1609459200000 // 2021-01-01 00:00:00 UTC in milliseconds
	DefaultMaxClockBackward = 5000          // 5 seconds in milliseconds
	
	// Redis integration constants
	DefaultRedisKeyPrefix      = "snowflake"
	DefaultWorkerIDTTL         = 30 * time.Second
	DefaultHeartbeatInterval   = 10 * time.Second
)

const (
	// Default bit allocation
	DefaultWorkerIDBits = 10
	
	// Default timing
	DefaultMaxClockDrift      = 5 * time.Second
	DefaultClockCheckInterval = 1 * time.Second
	
	// Default cache size
	DefaultSequenceCacheSize = 1000
	
	// Redis key patterns
	WorkerIDLockKey       = "lynx:snowflake:lock:worker_id"
	WorkerIDRegistryKey   = "lynx:snowflake:registry"
	
	// Clock drift actions
	ClockDriftActionWait   = "wait"
	ClockDriftActionError  = "error"
	ClockDriftActionIgnore = "ignore"
)
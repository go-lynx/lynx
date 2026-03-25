package events

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	kelindarEvent "github.com/kelindar/event"
	ants "github.com/panjf2000/ants/v2"
)

// SimpleThrottler implements a basic rate limiter
type SimpleThrottler struct {
	rate     int       // events per second
	burst    int       // burst size
	tokens   int       // current tokens
	lastTime time.Time // last token refill time
	mu       sync.Mutex
}

// IsClosed returns whether the bus is closed
func (b *LynxEventBus) IsClosed() bool { return b.isClosed.Load() }

// SetLogger sets the logger for this bus
func (b *LynxEventBus) SetLogger(logger log.Logger) { b.logger = logger }

// GetLogger returns the logger for this bus
func (b *LynxEventBus) GetLogger() log.Logger { return b.logger }

// GetBusType returns the bus type
func (b *LynxEventBus) GetBusType() BusType { return b.busType }

// GetConfig returns the bus configuration
func (b *LynxEventBus) GetConfig() BusConfig { return b.config }

// GetHistory returns the event history (if enabled)
func (b *LynxEventBus) GetHistory() *EventHistory { return b.history }

// GetMetrics returns the event metrics (if enabled)
func (b *LynxEventBus) GetMetrics() *EventMetrics { return b.metrics }

// LynxEventBus represents a single event bus for a specific bus type
type LynxEventBus struct {
	manager *EventBusManager
	// kelindar/event dispatcher
	dispatcher *kelindarEvent.Dispatcher

	// Bus configuration
	config  BusConfig
	busType BusType

	// Lynx specific features
	history *EventHistory
	metrics *EventMetrics
	logger  log.Logger

	// State management
	done     chan struct{}
	mu       sync.RWMutex
	isClosed atomic.Bool
	wg       sync.WaitGroup
	paused   atomic.Bool

	// Backpressure queue (single shared queue)
	queue chan LynxEvent
	// Atomic counter for queue size to avoid expensive len() calls
	queueSize atomic.Int64
	// Backpressure queue & subscribers tracking
	subscriberCount atomic.Int64
	// per-type subscriber counts for SubscribeTo
	typeSubs map[EventType]int

	// Pause tracking
	pauseStartTime time.Time
	pauseDuration  time.Duration
	pauseCount     atomic.Int64

	// Degradation tracking
	isDegraded           atomic.Bool
	degradationStartTime time.Time
	// Performance optimization: sample degradation checks instead of checking every event
	lastDegradationCheck atomic.Int64 // Unix timestamp in nanoseconds

	// Throttling for degradation mode
	throttler *SimpleThrottler

	// Worker pool for parallel dispatching
	workerPool *ants.Pool

	// Retry bounding (limit concurrent scheduled retries)
	retrySem chan struct{}

	// Memory optimization
	bufferPool   *EventBufferPool
	metadataPool *MetadataPool

	// Event deduplication
	processedEvents sync.Map // map[string]time.Time - eventID -> processing timestamp
	dedupWindow     time.Duration
}

// NewLynxEventBus creates a new LynxEventBus with the given configuration
func NewLynxEventBus(config BusConfig, busType BusType, manager *EventBusManager) *LynxEventBus {
	bus := &LynxEventBus{
		manager:    manager,
		dispatcher: kelindarEvent.NewDispatcher(),
		config:     config,
		busType:    busType,
		done:       make(chan struct{}),
		typeSubs:   make(map[EventType]int),
	}

	// Initialize history if enabled
	if config.EnableHistory {
		size := config.HistorySize
		if size <= 0 {
			size = 1000
		}
		bus.history = NewEventHistory(size)
	}

	// Initialize metrics if enabled
	if config.EnableMetrics {
		bus.metrics = NewEventMetrics()
	}

	// initialize single shared queue (unified capacity pool)
	capTotal := max(config.MaxQueue, 1)
	bus.queue = make(chan LynxEvent, capTotal)

	// Initialize worker pool (for parallel handler execution)
	poolSize := max(config.WorkerCount, 1)
	maxBlock := max(poolSize*4, 64)
	if p, err := ants.NewPool(poolSize, ants.WithNonblocking(false), ants.WithMaxBlockingTasks(maxBlock)); err == nil {
		bus.workerPool = p
	} else {
		// fallback: will publish synchronously if pool not available
		if bus.logger != nil {
			log.NewHelper(bus.logger).Warnf("worker pool init failed, fallback to sync: err=%v", err)
		}
	}

	// Initialize throttler if enabled
	if config.EnableThrottling {
		bus.throttler = NewSimpleThrottler(config.ThrottleRate, config.ThrottleBurst)
	}

	// Initialize retry semaphore if configured
	if config.MaxConcurrentRetries > 0 {
		bus.retrySem = make(chan struct{}, config.MaxConcurrentRetries)
	}

	// Initialize memory pools
	bus.bufferPool = GetGlobalEventBufferPool()
	bus.metadataPool = GetGlobalMetadataPool()

	// Initialize deduplication (default: 5 minutes window)
	bus.dedupWindow = 5 * time.Minute

	// Start worker to drain queue and publish to dispatcher
	bus.wg.Add(1)
	go bus.run()

	// Start cleanup goroutine for processed events map
	bus.wg.Add(1)
	go bus.cleanupProcessedEvents()

	return bus
}

// min returns the minimum of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two ints
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

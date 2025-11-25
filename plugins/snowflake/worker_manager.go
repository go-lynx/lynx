package snowflake

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

// NewWorkerIDManager creates a new worker ID manager
// Uses Redis INCR for lock-free worker ID allocation
func NewWorkerIDManager(redisClient redis.UniversalClient, datacenterID int64, config *WorkerManagerConfig) *WorkerIDManager {
	if config == nil {
		config = DefaultWorkerManagerConfig()
	}

	mgr := &WorkerIDManager{
		redisClient:       redisClient,
		datacenterID:      datacenterID,
		keyPrefix:         config.KeyPrefix,
		ttl:               config.TTL,
		heartbeatInterval: config.HeartbeatInterval,
		workerID:          -1, // Not assigned yet
		heartbeatCtx:      nil,
		heartbeatCancel:   nil,
		heartbeatRunning:  false,
	}
	atomic.StoreInt32(&mgr.healthy, 1) // Initially healthy
	return mgr
}

// RegisterWorkerID registers a worker ID using Redis INCR atomic operation
// This is lock-free and highly efficient for multi-instance deployment
func (w *WorkerIDManager) RegisterWorkerID(ctx context.Context, maxWorkerID int64) (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.workerID != -1 {
		return w.workerID, nil // Already registered
	}

	counterKey := w.getCounterKey()
	maxAttempts := maxWorkerID + 1 // Try each possible worker ID at most once

	for attempt := int64(0); attempt < maxAttempts; attempt++ {
		// Atomic increment to get a unique sequence number
		seq, err := w.redisClient.Incr(ctx, counterKey).Result()
		if err != nil {
			return -1, fmt.Errorf("failed to increment worker counter: %w", err)
		}

		// Calculate worker ID from sequence (modulo operation)
		candidateWorkerID := (seq - 1) % (maxWorkerID + 1)

		// Try to claim this worker ID using SetNX (atomic, no lock needed)
		success, err := w.tryClaimWorkerID(ctx, candidateWorkerID)
		if err != nil {
			return -1, fmt.Errorf("failed to claim worker ID %d: %w", candidateWorkerID, err)
		}

		if success {
			w.workerID = candidateWorkerID
			atomic.StoreInt32(&w.healthy, 1)
			w.startHeartbeatLocked()
			log.Infof("Successfully registered worker ID %d (datacenter: %d)", candidateWorkerID, w.datacenterID)
			return candidateWorkerID, nil
		}

		// This worker ID is taken, try next one
		log.Debugf("Worker ID %d is already taken, trying next...", candidateWorkerID)
	}

	return -1, fmt.Errorf("no available worker ID found after %d attempts (max: %d)", maxAttempts, maxWorkerID)
}

// tryClaimWorkerID attempts to claim a specific worker ID using SetNX (atomic operation)
func (w *WorkerIDManager) tryClaimWorkerID(ctx context.Context, workerID int64) (bool, error) {
	key := w.getWorkerKey(workerID)

	now := time.Now()
	w.instanceID = w.generateInstanceID()
	workerInfo := WorkerInfo{
		WorkerID:      workerID,
		DatacenterID:  w.datacenterID,
		RegisterTime:  now,
		LastHeartbeat: now,
		InstanceID:    w.instanceID,
	}

	// SetNX is atomic - only succeeds if key doesn't exist
	success, err := w.redisClient.SetNX(ctx, key, workerInfo.String(), w.ttl).Result()
	if err != nil {
		return false, err
	}

	if success {
		w.registerTime = now
		// Add to registry set for tracking
		registryKey := w.getRegistryKey()
		_ = w.redisClient.SAdd(ctx, registryKey, fmt.Sprintf("%d:%d", w.datacenterID, workerID))
	}

	return success, nil
}

// RegisterSpecificWorkerID registers a specific worker ID
func (w *WorkerIDManager) RegisterSpecificWorkerID(ctx context.Context, workerID int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.workerID != -1 {
		if w.workerID == workerID {
			return nil // Already registered with this ID
		}
		return fmt.Errorf("already registered with worker ID %d", w.workerID)
	}

	success, err := w.tryClaimWorkerID(ctx, workerID)
	if err != nil {
		return fmt.Errorf("failed to register worker ID %d: %w", workerID, err)
	}
	if !success {
		return &WorkerIDConflictError{
			WorkerID:     workerID,
			DatacenterID: w.datacenterID,
			ConflictWith: "another instance",
		}
	}

	w.workerID = workerID
	atomic.StoreInt32(&w.healthy, 1)
	w.startHeartbeatLocked()
	return nil
}

// IsHealthy returns whether the worker manager is healthy (heartbeat is working)
func (w *WorkerIDManager) IsHealthy() bool {
	return atomic.LoadInt32(&w.healthy) == 1
}

// startHeartbeatLocked starts the heartbeat if not running.
// Caller must hold w.mu.
func (w *WorkerIDManager) startHeartbeatLocked() {
	if w.heartbeatRunning {
		return
	}
	// Cancel any previous context just in case
	if w.heartbeatCancel != nil {
		w.heartbeatCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.heartbeatCtx = ctx
	w.heartbeatCancel = cancel
	w.heartbeatRunning = true
	go w.heartbeatLoop(ctx)
}

// heartbeatLoop starts the heartbeat process with context cancellation.
func (w *WorkerIDManager) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(w.heartbeatInterval)
	defer ticker.Stop()

	consecutiveFailures := 0
	maxConsecutiveFailures := 3

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.sendHeartbeat(); err != nil {
				consecutiveFailures++
				log.Warnf("snowflake worker heartbeat failed (attempt %d/%d): %v",
					consecutiveFailures, maxConsecutiveFailures, err)

				// Mark as unhealthy after first failure to prevent ID generation
				if consecutiveFailures >= 1 {
					atomic.StoreInt32(&w.healthy, 0)
				}

				// If too many failures, try to re-register
				if consecutiveFailures >= maxConsecutiveFailures {
					log.Errorf("snowflake worker heartbeat failed %d times, attempting re-registration",
						consecutiveFailures)

					// Try to re-register the same worker ID
					if reregErr := w.tryReRegister(ctx); reregErr != nil {
						log.Errorf("failed to re-register worker ID: %v", reregErr)
					} else {
						log.Infof("successfully re-registered worker ID %d", w.workerID)
						atomic.StoreInt32(&w.healthy, 1)
						consecutiveFailures = 0
					}
				}
			} else {
				// Reset failure counter and mark healthy on success
				if consecutiveFailures > 0 {
					log.Infof("snowflake worker heartbeat recovered after %d failures", consecutiveFailures)
					consecutiveFailures = 0
				}
				atomic.StoreInt32(&w.healthy, 1)
			}
		}
	}
}

// tryReRegister attempts to re-register the current worker ID
func (w *WorkerIDManager) tryReRegister(ctx context.Context) error {
	w.mu.Lock()
	workerID := w.workerID
	w.mu.Unlock()

	if workerID == -1 {
		return fmt.Errorf("no worker ID to re-register")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	key := w.getWorkerKey(workerID)

	now := time.Now()
	workerInfo := WorkerInfo{
		WorkerID:      workerID,
		DatacenterID:  w.datacenterID,
		RegisterTime:  w.registerTime,
		LastHeartbeat: now,
		InstanceID:    w.instanceID,
	}

	// Use SET with XX option - only set if exists, or use plain SET to reclaim
	// This will overwrite any stale data
	return w.redisClient.Set(timeoutCtx, key, workerInfo.String(), w.ttl).Err()
}

// sendHeartbeat sends a heartbeat to maintain worker ID registration
func (w *WorkerIDManager) sendHeartbeat() error {
	w.mu.RLock()
	workerID := w.workerID
	registerTime := w.registerTime
	instanceID := w.instanceID
	w.mu.RUnlock()

	if workerID == -1 {
		return fmt.Errorf("worker ID not registered")
	}

	parent := w.heartbeatCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()

	key := w.getWorkerKey(workerID)

	// Verify we still own this worker ID before updating
	// Use Lua script for atomic check-and-update
	script := `
		local current = redis.call('GET', KEYS[1])
		if current then
			local parts = {}
			for part in string.gmatch(current, "[^:]+") do
				table.insert(parts, part)
			end
			-- Check if instance ID matches (parts[5] onwards)
			local currentInstanceID = table.concat(parts, ":", 5)
			if currentInstanceID ~= ARGV[2] then
				return 0  -- Someone else owns this worker ID
			end
		end
		-- Update with new heartbeat
		redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[3])
		return 1
	`

	workerInfo := WorkerInfo{
		WorkerID:      workerID,
		DatacenterID:  w.datacenterID,
		RegisterTime:  registerTime,
		LastHeartbeat: time.Now(),
		InstanceID:    instanceID,
	}

	result, err := w.redisClient.Eval(ctx, script, []string{key},
		workerInfo.String(), instanceID, int64(w.ttl.Seconds())).Result()
	if err != nil {
		return fmt.Errorf("heartbeat script failed: %w", err)
	}

	if result.(int64) == 0 {
		return fmt.Errorf("worker ID %d was taken by another instance", workerID)
	}

	return nil
}

// UnregisterWorkerID unregisters the worker ID
func (w *WorkerIDManager) UnregisterWorkerID(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.workerID == -1 {
		return nil // Not registered
	}

	// Mark as unhealthy first
	atomic.StoreInt32(&w.healthy, 0)

	// Stop heartbeat
	if w.heartbeatCancel != nil {
		w.heartbeatCancel()
		w.heartbeatCancel = nil
		w.heartbeatCtx = nil
		w.heartbeatRunning = false
	}

	key := w.getWorkerKey(w.workerID)
	registryKey := w.getRegistryKey()

	// Remove from registry
	_ = w.redisClient.SRem(ctx, registryKey, fmt.Sprintf("%d:%d", w.datacenterID, w.workerID))

	// Remove worker key
	_ = w.redisClient.Del(ctx, key)

	w.workerID = -1
	return nil
}

// GetWorkerID returns the current worker ID
func (w *WorkerIDManager) GetWorkerID() int64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.workerID
}

// GetRegisteredWorkers returns all registered workers
func (w *WorkerIDManager) GetRegisteredWorkers(ctx context.Context) ([]WorkerInfo, error) {
	registryKey := w.getRegistryKey()

	members, err := w.redisClient.SMembers(ctx, registryKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get registry members: %w", err)
	}

	var workers []WorkerInfo
	for _, member := range members {
		parts := strings.Split(member, ":")
		if len(parts) != 2 {
			continue
		}

		datacenterID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}

		workerID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}

		// Get worker info
		key := w.getWorkerKeyForDatacenter(datacenterID, workerID)
		infoStr, err := w.redisClient.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		workerInfo, err := ParseWorkerInfo(infoStr)
		if err != nil {
			continue
		}

		workers = append(workers, *workerInfo)
	}

	return workers, nil
}

// Helper methods
func (w *WorkerIDManager) getWorkerKey(workerID int64) string {
	return fmt.Sprintf("%sdc:%d:worker:%d", w.keyPrefix, w.datacenterID, workerID)
}

func (w *WorkerIDManager) getWorkerKeyForDatacenter(datacenterID, workerID int64) string {
	return fmt.Sprintf("%sdc:%d:worker:%d", w.keyPrefix, datacenterID, workerID)
}

func (w *WorkerIDManager) getCounterKey() string {
	return fmt.Sprintf("%sdc:%d:counter", w.keyPrefix, w.datacenterID)
}

func (w *WorkerIDManager) getRegistryKey() string {
	return fmt.Sprintf("%sregistry", w.keyPrefix)
}

func (w *WorkerIDManager) generateInstanceID() string {
	return fmt.Sprintf("instance-%d-%d-%d", time.Now().UnixNano(), w.datacenterID, time.Now().UnixMicro()%10000)
}

// WorkerInfo represents information about a registered worker
type WorkerInfo struct {
	WorkerID      int64     `json:"worker_id"`
	DatacenterID  int64     `json:"datacenter_id"`
	RegisterTime  time.Time `json:"register_time"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	InstanceID    string    `json:"instance_id"`
}

// String returns a string representation of WorkerInfo
func (wi *WorkerInfo) String() string {
	return fmt.Sprintf("%d:%d:%d:%d:%s",
		wi.WorkerID,
		wi.DatacenterID,
		wi.RegisterTime.Unix(),
		wi.LastHeartbeat.Unix(),
		wi.InstanceID)
}

// ParseWorkerInfo parses a WorkerInfo from string
func ParseWorkerInfo(s string) (*WorkerInfo, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 5 {
		return nil, fmt.Errorf("invalid worker info format")
	}

	workerID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid worker ID: %w", err)
	}

	datacenterID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid datacenter ID: %w", err)
	}

	registerTime, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid register time: %w", err)
	}

	lastHeartbeat, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid last heartbeat: %w", err)
	}

	instanceID := strings.Join(parts[4:], ":")

	return &WorkerInfo{
		WorkerID:      workerID,
		DatacenterID:  datacenterID,
		RegisterTime:  time.Unix(registerTime, 0),
		LastHeartbeat: time.Unix(lastHeartbeat, 0),
		InstanceID:    instanceID,
	}, nil
}

// WorkerManagerConfig holds configuration for the worker manager
type WorkerManagerConfig struct {
	KeyPrefix         string
	TTL               time.Duration
	HeartbeatInterval time.Duration
}

// DefaultWorkerManagerConfig returns default worker manager configuration
func DefaultWorkerManagerConfig() *WorkerManagerConfig {
	return &WorkerManagerConfig{
		KeyPrefix:         DefaultRedisKeyPrefix,
		TTL:               DefaultWorkerIDTTL,
		HeartbeatInterval: DefaultHeartbeatInterval,
	}
}

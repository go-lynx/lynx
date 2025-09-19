package snowflake

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewWorkerIDManager creates a new worker ID manager
func NewWorkerIDManager(redisClient redis.UniversalClient, datacenterID int64, config *WorkerManagerConfig) *WorkerIDManager {
	if config == nil {
		config = DefaultWorkerManagerConfig()
	}

	return &WorkerIDManager{
		redisClient:       redisClient,
		datacenterID:      datacenterID,
		keyPrefix:         config.KeyPrefix,
		ttl:               config.TTL,
		heartbeatInterval: config.HeartbeatInterval,
		shutdownCh:        make(chan struct{}),
		workerID:          -1, // Not assigned yet
	}
}

// RegisterWorkerID registers a worker ID automatically
func (w *WorkerIDManager) RegisterWorkerID(ctx context.Context, maxWorkerID int64) (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Try to register a worker ID
	for workerID := int64(0); workerID <= maxWorkerID; workerID++ {
		if success, err := w.tryRegisterWorkerID(ctx, workerID); err != nil {
			return -1, fmt.Errorf("failed to register worker ID %d: %w", workerID, err)
		} else if success {
			w.workerID = workerID
			// Start heartbeat
			go w.startHeartbeat()
			return workerID, nil
		}
	}

	return -1, fmt.Errorf("no available worker ID found (max: %d)", maxWorkerID)
}

// RegisterSpecificWorkerID registers a specific worker ID
func (w *WorkerIDManager) RegisterSpecificWorkerID(ctx context.Context, workerID int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	success, err := w.tryRegisterWorkerID(ctx, workerID)
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
	// Start heartbeat
	go w.startHeartbeat()
	return nil
}

// tryRegisterWorkerID attempts to register a specific worker ID
func (w *WorkerIDManager) tryRegisterWorkerID(ctx context.Context, workerID int64) (bool, error) {
	key := w.getWorkerKey(workerID)
	lockKey := w.getLockKey(workerID)
	registryKey := w.getRegistryKey()

	// Use distributed lock to prevent race conditions
	lockValue := fmt.Sprintf("%d:%d:%d", w.datacenterID, workerID, time.Now().UnixNano())
	
	// Try to acquire lock
	acquired, err := w.redisClient.SetNX(ctx, lockKey, lockValue, 10*time.Second).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !acquired {
		return false, nil // Lock not acquired, worker ID is being registered by another instance
	}

	// Ensure lock is released
	defer func() {
		w.redisClient.Del(ctx, lockKey)
	}()

	// Check if worker ID is already registered
	exists, err := w.redisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check worker ID existence: %w", err)
	}
	if exists > 0 {
		return false, nil // Worker ID already registered
	}

	// Register worker ID
	workerInfo := WorkerInfo{
		WorkerID:     workerID,
		DatacenterID: w.datacenterID,
		RegisterTime: time.Now(),
		LastHeartbeat: time.Now(),
		InstanceID:   w.generateInstanceID(),
	}

	// Set worker key with TTL
	err = w.redisClient.Set(ctx, key, workerInfo.String(), w.ttl).Err()
	if err != nil {
		return false, fmt.Errorf("failed to set worker key: %w", err)
	}

	// Add to registry
	err = w.redisClient.SAdd(ctx, registryKey, fmt.Sprintf("%d:%d", w.datacenterID, workerID)).Err()
	if err != nil {
		// Cleanup on failure
		w.redisClient.Del(ctx, key)
		return false, fmt.Errorf("failed to add to registry: %w", err)
	}

	return true, nil
}

// startHeartbeat starts the heartbeat process
func (w *WorkerIDManager) startHeartbeat() {
	ticker := time.NewTicker(w.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.shutdownCh:
			return
		case <-ticker.C:
			if err := w.sendHeartbeat(); err != nil {
				// Log error but continue
				fmt.Printf("heartbeat failed: %v\n", err)
			}
		}
	}
}

// sendHeartbeat sends a heartbeat to maintain worker ID registration
func (w *WorkerIDManager) sendHeartbeat() error {
	w.mu.RLock()
	workerID := w.workerID
	w.mu.RUnlock()

	if workerID == -1 {
		return fmt.Errorf("worker ID not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := w.getWorkerKey(workerID)

	// Update TTL and last heartbeat time
	workerInfo := WorkerInfo{
		WorkerID:      workerID,
		DatacenterID:  w.datacenterID,
		RegisterTime:  time.Now(), // This should be preserved, but for simplicity we use current time
		LastHeartbeat: time.Now(),
		InstanceID:    w.generateInstanceID(),
	}

	return w.redisClient.Set(ctx, key, workerInfo.String(), w.ttl).Err()
}

// UnregisterWorkerID unregisters the worker ID
func (w *WorkerIDManager) UnregisterWorkerID(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.workerID == -1 {
		return nil // Not registered
	}

	// Stop heartbeat
	close(w.shutdownCh)

	key := w.getWorkerKey(w.workerID)
	registryKey := w.getRegistryKey()

	// Remove from registry
	err := w.redisClient.SRem(ctx, registryKey, fmt.Sprintf("%d:%d", w.datacenterID, w.workerID)).Err()
	if err != nil {
		return fmt.Errorf("failed to remove from registry: %w", err)
	}

	// Remove worker key
	err = w.redisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to remove worker key: %w", err)
	}

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
	return fmt.Sprintf("%s:dc:%d:worker:%d", w.keyPrefix, w.datacenterID, workerID)
}

func (w *WorkerIDManager) getWorkerKeyForDatacenter(datacenterID, workerID int64) string {
	return fmt.Sprintf("%s:dc:%d:worker:%d", w.keyPrefix, datacenterID, workerID)
}

func (w *WorkerIDManager) getLockKey(workerID int64) string {
	return fmt.Sprintf("%s:lock:dc:%d:worker:%d", w.keyPrefix, w.datacenterID, workerID)
}

func (w *WorkerIDManager) getRegistryKey() string {
	return fmt.Sprintf("%s:registry", w.keyPrefix)
}

func (w *WorkerIDManager) generateInstanceID() string {
	return fmt.Sprintf("instance-%d-%d", time.Now().UnixNano(), w.datacenterID)
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
func (w *WorkerInfo) String() string {
	return fmt.Sprintf("%d:%d:%d:%d:%s", 
		w.WorkerID, 
		w.DatacenterID, 
		w.RegisterTime.Unix(), 
		w.LastHeartbeat.Unix(), 
		w.InstanceID)
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
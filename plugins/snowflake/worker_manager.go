package snowflake

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

// getLocalIP returns the local IP address of the machine
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return "unknown"
}

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
		localIP:           getLocalIP(), // Get local IP for troubleshooting
	}
	atomic.StoreInt32(&mgr.healthy, 1) // Initially healthy
	return mgr
}

// RegisterWorkerID registers a worker ID
// Flow: INCR to get workerID -> if exceeds max, reset to 0 -> SetNX to verify -> retry until full cycle
// Heartbeat maintains key TTL to ensure worker ID exclusivity during instance lifetime
func (w *WorkerIDManager) RegisterWorkerID(ctx context.Context, maxWorkerID int64) (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.workerID != -1 {
		return w.workerID, nil // Already registered
	}

	counterKey := w.getCounterKey()
	totalWorkerIDs := maxWorkerID + 1 // Total available worker IDs (0 to maxWorkerID)
	maxRetries := int(totalWorkerIDs) // Try each worker ID at most once (full cycle)

	// Loop to try acquiring an available worker ID
	for retryCount := 0; retryCount < maxRetries; retryCount++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		default:
		}

		// 1. Atomic INCR with auto-reset using Lua script
		// If counter exceeds max, reset to 1 and return 1
		// This prevents race condition when multiple instances try to reset simultaneously
		result, err := w.redisClient.Eval(ctx, LuaScriptIncrWithReset, []string{counterKey}, totalWorkerIDs).Result()
		if err != nil {
			return -1, fmt.Errorf("failed to execute INCR script: %w", err)
		}
		seq := result.(int64)

		// 2. workerID = seq - 1 (0-based)
		workerID := seq - 1

		// 4. SetNX to verify this worker ID is available
		now := time.Now()
		w.instanceID = w.generateInstanceID()
		workerInfo := WorkerInfo{
			WorkerID:      workerID,
			DatacenterID:  w.datacenterID,
			IP:            w.localIP,
			RegisterTime:  now.Unix(),
			LastHeartbeat: now.Unix(),
			InstanceID:    w.instanceID,
		}

		key := w.getWorkerKey(workerID)
		success, err := w.redisClient.SetNX(ctx, key, workerInfo.String(), w.ttl).Result()
		if err != nil {
			return -1, fmt.Errorf("failed to SetNX worker ID %d: %w", workerID, err)
		}

		if success {
			// Registration successful
			w.workerID = workerID
			w.registerTime = now

			// Add to registry set (for monitoring)
			registryKey := w.getRegistryKey()
			_ = w.redisClient.SAdd(ctx, registryKey, fmt.Sprintf("%d:%d", w.datacenterID, workerID))

			atomic.StoreInt32(&w.healthy, 1)
			w.startHeartbeatLocked() // Start heartbeat to maintain key TTL

			log.Infof("successfully registered worker ID %d (datacenter: %d, attempts: %d)", workerID, w.datacenterID, retryCount+1)
			return workerID, nil
		}

		// SetNX failed, this worker ID is already taken
		log.Debugf("worker ID %d is taken, attempt %d/%d", workerID, retryCount+1, maxRetries)

		// Backoff sleep: random 10-50ms to prevent retry storm
		backoff := time.Duration(10+rand.Intn(40)) * time.Millisecond
		time.Sleep(backoff)
	}

	// All worker IDs are taken after a full cycle
	atomic.StoreInt32(&w.healthy, 0)
	return -1, fmt.Errorf("all %d worker IDs are occupied, registration failed", totalWorkerIDs)
}

// RegisterSpecificWorkerID registers a specific worker ID
// Uses SetNX to verify worker ID availability, returns error if already taken
func (w *WorkerIDManager) RegisterSpecificWorkerID(ctx context.Context, workerID int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.workerID != -1 {
		if w.workerID == workerID {
			return nil // Already registered with this ID
		}
		return fmt.Errorf("already registered with worker ID %d", w.workerID)
	}

	now := time.Now()
	w.instanceID = w.generateInstanceID()
	workerInfo := WorkerInfo{
		WorkerID:      workerID,
		DatacenterID:  w.datacenterID,
		IP:            w.localIP,
		RegisterTime:  now.Unix(),
		LastHeartbeat: now.Unix(),
		InstanceID:    w.instanceID,
	}

	// SetNX to verify worker ID availability
	key := w.getWorkerKey(workerID)
	success, err := w.redisClient.SetNX(ctx, key, workerInfo.String(), w.ttl).Result()
	if err != nil {
		return fmt.Errorf("failed to SetNX worker ID %d: %w", workerID, err)
	}
	if !success {
		atomic.StoreInt32(&w.healthy, 0) // Mark as unhealthy
		return &WorkerIDConflictError{
			WorkerID:     workerID,
			DatacenterID: w.datacenterID,
			ConflictWith: "another instance",
		}
	}

	w.workerID = workerID
	w.registerTime = now

	registryKey := w.getRegistryKey()
	_ = w.redisClient.SAdd(ctx, registryKey, fmt.Sprintf("%d:%d", w.datacenterID, workerID))

	atomic.StoreInt32(&w.healthy, 1)
	w.startHeartbeatLocked()

	log.Infof("successfully registered specific worker ID %d (datacenter: %d)", workerID, w.datacenterID)
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
		IP:            w.localIP,
		RegisterTime:  w.registerTime.Unix(),
		LastHeartbeat: now.Unix(),
		InstanceID:    w.instanceID,
	}

	// Use SET with XX option - only set if exists, or use plain SET to reclaim
	// This will overwrite any stale data
	return w.redisClient.Set(timeoutCtx, key, workerInfo.String(), w.ttl).Err()
}

// sendHeartbeat sends heartbeat to maintain worker ID key TTL
// Uses Lua script to atomically verify instanceID and refresh TTL
func (w *WorkerIDManager) sendHeartbeat() error {
	w.mu.RLock()
	workerID := w.workerID
	registerTime := w.registerTime
	instanceID := w.instanceID
	localIP := w.localIP
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

	workerInfo := WorkerInfo{
		WorkerID:      workerID,
		DatacenterID:  w.datacenterID,
		IP:            localIP,
		RegisterTime:  registerTime.Unix(),
		LastHeartbeat: time.Now().Unix(),
		InstanceID:    instanceID,
	}

	result, err := w.redisClient.Eval(ctx, LuaScriptHeartbeat, []string{key},
		workerInfo.String(), instanceID, int64(w.ttl.Seconds())).Result()
	if err != nil {
		return fmt.Errorf("heartbeat script execution failed: %w", err)
	}

	code := result.(int64)
	switch code {
	case 1:
		return nil // Success
	case 0:
		return fmt.Errorf("worker ID %d was taken by another instance", workerID)
	case -1:
		return fmt.Errorf("worker ID %d key has expired", workerID)
	case -2:
		return fmt.Errorf("worker ID %d has invalid JSON format", workerID)
	default:
		return fmt.Errorf("heartbeat returned unknown status: %d", code)
	}
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
	WorkerID      int64  `json:"worker_id"`
	DatacenterID  int64  `json:"datacenter_id"`
	IP            string `json:"ip"`
	RegisterTime  int64  `json:"register_time"`
	LastHeartbeat int64  `json:"last_heartbeat"`
	InstanceID    string `json:"instance_id"`
}

// String returns JSON representation of WorkerInfo
func (wi *WorkerInfo) String() string {
	data, _ := json.Marshal(wi)
	return string(data)
}

// ParseWorkerInfo parses a WorkerInfo from JSON string
func ParseWorkerInfo(s string) (*WorkerInfo, error) {
	var info WorkerInfo
	if err := json.Unmarshal([]byte(s), &info); err != nil {
		return nil, fmt.Errorf("invalid worker info JSON: %w", err)
	}
	return &info, nil
}

// GetRegisterTime returns register time as time.Time
func (wi *WorkerInfo) GetRegisterTime() time.Time {
	return time.Unix(wi.RegisterTime, 0)
}

// GetLastHeartbeat returns last heartbeat as time.Time
func (wi *WorkerInfo) GetLastHeartbeat() time.Time {
	return time.Unix(wi.LastHeartbeat, 0)
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

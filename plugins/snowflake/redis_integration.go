package snowflake

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"

	pb "github.com/go-lynx/lynx/plugins/snowflake/conf"
)

// RedisIntegration handles integration with Redis plugin
type RedisIntegration struct {
	client redis.UniversalClient
	config *RedisIntegrationConfig
}

// RedisIntegrationConfig holds Redis integration configuration
type RedisIntegrationConfig struct {
	RedisPluginName string // Name of the Redis plugin instance to use
	Database        int    // Redis database number for snowflake
	KeyPrefix       string // Key prefix for snowflake keys
}

// Validate validates the Redis integration configuration
func (c *RedisIntegrationConfig) Validate() error {
	// Validate Redis plugin name
	if c.RedisPluginName == "" {
		return fmt.Errorf("redis plugin name cannot be empty")
	}

	// Validate plugin name format
	if len(c.RedisPluginName) > 100 {
		return fmt.Errorf("redis plugin name is too long (>100 chars): %s", c.RedisPluginName)
	}

	// Check for invalid characters in plugin name
	if strings.ContainsAny(c.RedisPluginName, " \t\n\r/\\") {
		return fmt.Errorf("redis plugin name contains invalid characters: %s", c.RedisPluginName)
	}

	// Validate database number
	if c.Database < 0 || c.Database > 15 {
		return fmt.Errorf("redis database number must be between 0 and 15, got %d", c.Database)
	}

	// Validate key prefix
	if c.KeyPrefix == "" {
		return fmt.Errorf("key prefix cannot be empty")
	}

	// Check key prefix length
	if len(c.KeyPrefix) > 50 {
		return fmt.Errorf("key prefix is too long (>50 chars): %s", c.KeyPrefix)
	}

	// Check for invalid characters in key prefix
	if strings.ContainsAny(c.KeyPrefix, " \t\n\r") {
		return fmt.Errorf("key prefix cannot contain whitespace characters: %s", c.KeyPrefix)
	}

	// Check for Redis key pattern conflicts
	if strings.Contains(c.KeyPrefix, "*") || strings.Contains(c.KeyPrefix, "?") {
		return fmt.Errorf("key prefix cannot contain Redis pattern characters (* or ?): %s", c.KeyPrefix)
	}

	// Ensure key prefix ends with separator for clarity
	if !strings.HasSuffix(c.KeyPrefix, ":") && !strings.HasSuffix(c.KeyPrefix, "_") {
		return fmt.Errorf("key prefix should end with ':' or '_' separator: %s", c.KeyPrefix)
	}

	return nil
}

// NewRedisIntegration creates a new Redis integration
func NewRedisIntegration(config *RedisIntegrationConfig) (*RedisIntegration, error) {
	if config == nil {
		config = DefaultRedisIntegrationConfig()
	}

	// Get Redis client from Redis plugin
	client, err := getRedisClientFromPlugin(config.RedisPluginName, config.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis client: %w", err)
	}

	return &RedisIntegration{
		client: client,
		config: config,
	}, nil
}

// GetClient returns the Redis client
func (r *RedisIntegration) GetClient() redis.UniversalClient {
	return r.client
}

// TestConnection tests the Redis connection
func (r *RedisIntegration) TestConnection(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// CreateWorkerManager creates a worker ID manager using this Redis integration
func (r *RedisIntegration) CreateWorkerManager(datacenterID int64, config *WorkerManagerConfig) *WorkerIDManager {
	if config == nil {
		config = DefaultWorkerManagerConfig()
	}

	// Use the key prefix from Redis integration config if not specified
	if config.KeyPrefix == "" {
		config.KeyPrefix = r.config.KeyPrefix
	}

	return NewWorkerIDManager(r.client, datacenterID, config)
}

// getRedisClientFromPlugin gets Redis client from the Redis plugin
func getRedisClientFromPlugin(pluginName string, database int) (redis.UniversalClient, error) {
	// Try to get Redis plugin instance from the global plugin registry
	// This implementation integrates with the Lynx plugin system

	// Method 1: Try to get from global registry (if available)
	if client := tryGetFromGlobalRegistry(pluginName, database); client != nil {
		return client, nil
	}

	// Method 2: Try to create from configuration with enhanced options
	if client := tryCreateFromConfig(pluginName, database); client != nil {
		return client, nil
	}

	// Method 3: Try to create with cluster support
	if client := tryCreateClusterClient(pluginName, database); client != nil {
		return client, nil
	}

	return nil, fmt.Errorf("redis plugin '%s' not found or not initialized", pluginName)
}

// tryGetFromGlobalRegistry attempts to get Redis client from global plugin registry
func tryGetFromGlobalRegistry(pluginName string, database int) redis.UniversalClient {
	// This is a placeholder implementation
	// In real scenario, you would use the actual plugin registry
	// For example: return plugins.GetRedisClient(pluginName, database)

	// Try to get from global context if available
	// This is a placeholder implementation
	// In real scenario, you would use the actual plugin registry

	return nil
}

// tryCreateFromConfig attempts to create Redis client from configuration
func tryCreateFromConfig(pluginName string, database int) redis.UniversalClient {
	// Enhanced Redis client configuration with connection pool settings
	log.Warn("Creating Redis client from default config - not recommended for production")

	// Create Redis client with enhanced connection pool settings
	client := redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           database,
		PoolSize:     10,              // Connection pool size
		MinIdleConns: 5,               // Minimum idle connections
		MaxRetries:   3,               // Maximum retry attempts
		DialTimeout:  5 * time.Second, // Connection timeout
		ReadTimeout:  3 * time.Second, // Read timeout
		WriteTimeout: 3 * time.Second, // Write timeout
		PoolTimeout:  4 * time.Second, // Pool timeout
	})

	// Test the connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Errorf("Failed to connect to Redis: %v", err)
		err := client.Close()
		if err != nil {
			log.Error(err)
			return nil
		}
		return nil
	}

	log.Info("Successfully connected to Redis with enhanced configuration")
	return client
}

// tryCreateClusterClient attempts to create Redis cluster client
func tryCreateClusterClient(pluginName string, database int) redis.UniversalClient {
	// Try to create Redis cluster client for high availability
	log.Info("Attempting to create Redis cluster client")

	// Default cluster addresses - in production, these should come from configuration
	clusterAddresses := []string{
		"localhost:7000",
		"localhost:7001",
		"localhost:7002",
	}

	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        clusterAddresses,
		Password:     "",
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	})

	// Test cluster connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Warnf("Failed to connect to Redis cluster: %v", err)
		err := client.Close()
		if err != nil {
			log.Error(err)
			return nil
		}
		return nil
	}

	log.Info("Successfully connected to Redis cluster")
	return client
}

// DefaultRedisIntegrationConfig returns default Redis integration configuration
func DefaultRedisIntegrationConfig() *RedisIntegrationConfig {
	return &RedisIntegrationConfig{
		RedisPluginName: "default", // Default Redis plugin name
		Database:        0,         // Default Redis database
		KeyPrefix:       DefaultRedisKeyPrefix,
	}
}

// RedisSnowflakePlugin represents a snowflake plugin with Redis integration
type RedisSnowflakePlugin struct {
	*PlugSnowflake
	redisIntegration *RedisIntegration
	workerManager    *WorkerIDManager
}

// NewRedisSnowflakePlugin creates a new snowflake plugin with Redis integration
func NewRedisSnowflakePlugin(config *pb.Snowflake, redisConfig *RedisIntegrationConfig) (*RedisSnowflakePlugin, error) {
	// Create Redis integration
	redisIntegration, err := NewRedisIntegration(redisConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis integration: %w", err)
	}

	// Test Redis connection
	ctx := context.Background()
	if err := redisIntegration.TestConnection(ctx); err != nil {
		return nil, fmt.Errorf("redis connection test failed: %w", err)
	}

	// Create base snowflake plugin
	basePlugin := NewSnowflakePlugin()

	// Set the configuration directly (UpdateConfiguration is the available method)
	if err := basePlugin.UpdateConfiguration(config); err != nil {
		return nil, fmt.Errorf("failed to configure snowflake plugin: %w", err)
	}

	// Create worker manager if Redis integration is enabled
	var workerManager *WorkerIDManager
	if config.GetAutoRegisterWorkerId() && config.GetRedisPluginName() != "" {
		workerManagerConfig := &WorkerManagerConfig{
			KeyPrefix:         config.GetRedisKeyPrefix(),
			TTL:               config.GetWorkerIdTtl().AsDuration(),
			HeartbeatInterval: config.GetHeartbeatInterval().AsDuration(),
		}
		workerManager = redisIntegration.CreateWorkerManager(int64(config.GetDatacenterId()), workerManagerConfig)

		// Auto-register worker ID if enabled
		if config.GetAutoRegisterWorkerId() {
			maxWorkerID := int64((1 << 10) - 1) // Default worker ID bits
			workerID, err := workerManager.RegisterWorkerID(ctx, maxWorkerID)
			if err != nil {
				return nil, fmt.Errorf("failed to auto-register worker ID: %w", err)
			}

			log.Infof("Auto-registered worker ID: %d", workerID)
		}
	}

	return &RedisSnowflakePlugin{
		PlugSnowflake:    basePlugin,
		redisIntegration: redisIntegration,
		workerManager:    workerManager,
	}, nil
}

// GetWorkerManager returns the worker manager
func (r *RedisSnowflakePlugin) GetWorkerManager() *WorkerIDManager {
	return r.workerManager
}

// GetRedisIntegration returns the Redis integration
func (r *RedisSnowflakePlugin) GetRedisIntegration() *RedisIntegration {
	return r.redisIntegration
}

// Shutdown gracefully shuts down the plugin
func (r *RedisSnowflakePlugin) Shutdown(ctx context.Context) error {
	if r.workerManager != nil {
		if err := r.workerManager.UnregisterWorkerID(ctx); err != nil {
			log.Errorf("Failed to unregister worker ID: %v", err)
			return err
		}
	}
	return nil
}

// RegisterWorkerID manually registers a specific worker ID
func (r *RedisSnowflakePlugin) RegisterWorkerID(ctx context.Context, workerID int64) error {
	if r.workerManager == nil {
		return fmt.Errorf("worker manager not initialized")
	}

	if err := r.workerManager.RegisterSpecificWorkerID(ctx, workerID); err != nil {
		return err
	}

	// Update the generator's worker ID
	r.PlugSnowflake.conf.WorkerId = int32(workerID)
	return nil
}

// GetRegisteredWorkers returns all registered workers
func (r *RedisSnowflakePlugin) GetRegisteredWorkers(ctx context.Context) ([]WorkerInfo, error) {
	if r.workerManager == nil {
		return nil, fmt.Errorf("worker manager not initialized")
	}

	return r.workerManager.GetRegisteredWorkers(ctx)
}

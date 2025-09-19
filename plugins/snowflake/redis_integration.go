package snowflake

import (
	"context"
	"fmt"

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
	// Try to get Redis plugin instance
	// This is a simplified implementation - in real scenario, you would use
	// the actual plugin registry to get the Redis plugin instance
	
	// For now, we'll try to get it from the global context or configuration
	// This assumes the Redis plugin is already initialized and available
	
	// Method 1: Try to get from global registry (if available)
	if client := tryGetFromGlobalRegistry(pluginName, database); client != nil {
		return client, nil
	}

	// Method 2: Try to create from configuration
	if client := tryCreateFromConfig(pluginName, database); client != nil {
		return client, nil
	}

	return nil, fmt.Errorf("Redis plugin '%s' not found or not initialized", pluginName)
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
	// This is a placeholder implementation
	// In real scenario, you would read the Redis plugin configuration
	// and create a client based on that configuration
	
	// For development/testing, you could create a default client
	// But this should not be used in production
	
	log.Warn("Creating Redis client from default config - not recommended for production")
	
	// Create a simple Redis client for development
	// This should be replaced with proper plugin integration
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       database,
	})
	
	// Test the connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Errorf("Failed to connect to Redis: %v", err)
		return nil
	}
	
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
		return nil, fmt.Errorf("Redis connection test failed: %w", err)
	}

	// Create base snowflake plugin
	basePlugin := NewSnowflakePlugin()
	
	// Configure the plugin with the provided config
	if err := basePlugin.Configure(config); err != nil {
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
			
			fmt.Printf("Auto-registered worker ID: %d\n", workerID)
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
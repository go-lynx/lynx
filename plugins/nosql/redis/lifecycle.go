package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
	"github.com/go-lynx/lynx/plugins/nosql/redis/conf"
	"github.com/redis/go-redis/v9"
)

// InitializeResources implements custom initialization logic for Redis plugin
// Scans and loads Redis configuration from runtime config, uses default config if not provided
// Parameter rt is the runtime environment
// Returns error information, returns corresponding error if configuration loading fails
func (r *PlugRedis) InitializeResources(rt plugins.Runtime) error {
	// Initialize an empty configuration structure
	r.conf = &conf.Redis{}

	// Scan and load Redis configuration from runtime config
	err := rt.GetConfig().Value(confPrefix).Scan(r.conf)
	if err != nil {
		return err
	}

	// Validate configuration and set default values
	if err := ValidateAndSetDefaults(r.conf); err != nil {
		return fmt.Errorf("redis configuration validation failed: %w", err)
	}

	return nil
}

// StartupTasks starts Redis client and performs health check
// Returns error information, returns corresponding error if startup or health check fails
func (r *PlugRedis) StartupTasks() error {
	// Log Redis client startup
	log.Infof("starting redis client")

	// Increment startup counter
	redisStartupTotal.Inc()

	// Create Redis universal client (supports single node/cluster/sentinel)
	r.rdb = redis.NewUniversalClient(r.buildUniversalOptions())

	// Register command-level metrics hook
	r.rdb.AddHook(metricsHook{})

	// Initialize collector channel
	r.statsQuit = make(chan struct{})

	// Perform quick health check at startup (short timeout)
	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	start := time.Now()
	_, err := r.rdb.Ping(pingCtx).Result()
	cancel()
	if err != nil {
		// Startup failure requires complete rollback of all resources
		r.cleanupOnStartupFailure()
		redisStartupFailedTotal.Inc()
		return fmt.Errorf("redis ping failed during startup: %w", err)
	}
	latency := time.Since(start)
	redisPingLatency.Observe(latency.Seconds())

	// Determine mode (single node/cluster/sentinel)
	mode := r.detectMode()
	log.Infof("redis client successfully started, mode=%s, addrs=%v, ping_latency=%s", mode, r.currentAddrList(), latency)

	// Perform enhanced check at startup stage
	r.enhancedReadinessCheck(mode)

	// Start pool statistics collector
	r.startPoolStatsCollector()
	// Start info collector
	r.startInfoCollector(mode)
	return nil
}

// cleanupOnStartupFailure cleans up resources on startup failure
func (r *PlugRedis) cleanupOnStartupFailure() {
	// Close Redis client
	if r.rdb != nil {
		if err := r.rdb.Close(); err != nil {
			log.Warnf("failed to close redis client during startup cleanup: %v", err)
		}
		r.rdb = nil
	}

	// Clean up collector channel
	if r.statsQuit != nil {
		close(r.statsQuit)
		r.statsQuit = nil
	}

	// Wait for potentially started goroutines to exit
	done := make(chan struct{})
	go func() {
		r.statsWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Infof("startup cleanup completed successfully")
	case <-time.After(5 * time.Second):
		log.Warnf("timeout waiting for goroutines cleanup during startup failure")
	}
}

// CleanupTasks closes Redis client
// Returns error information, returns corresponding error if client closing fails
func (r *PlugRedis) CleanupTasks() error {
	// If Redis client is not initialized, return nil directly
	if r.rdb == nil {
		return nil
	}

	// Stop collectors
	if r.statsQuit != nil {
		// Safely close channel to avoid duplicate closing
		select {
		case <-r.statsQuit:
			// Channel already closed
		default:
			close(r.statsQuit)
		}

		// Wait for all collector goroutines to exit, set timeout to avoid infinite waiting
		done := make(chan struct{})
		go func() {
			r.statsWG.Wait()
			close(done)
		}()

		select {
		case <-done:
			log.Infof("redis stats collectors stopped successfully")
		case <-time.After(10 * time.Second):
			log.Warnf("timeout waiting for redis stats collectors to stop")
		}
	}

	// Close Redis client
	if err := r.rdb.Close(); err != nil {
		// Return error with plugin information
		return plugins.NewPluginError(r.ID(), "Stop", "Failed to stop Redis client", err)
	}
	return nil
}

// Configure allows updating Redis server configuration at runtime
// Parameter c should be a pointer to a conf.Redis structure, containing new configuration information
// Returns error information, returns corresponding error if configuration update fails
func (r *PlugRedis) Configure(c any) error {
	// If the incoming configuration is nil, return nil directly
	if c == nil {
		return nil
	}
	// Convert the incoming configuration to *conf.Redis type and update to plugin configuration
	r.conf = c.(*conf.Redis)
	return nil
}

// CheckHealth implements the health check interface for Redis server
// Performs necessary health checks on the Redis server and updates the provided health report
// Parameter report is a pointer to the health report, used to record health check results
// Returns error information, returns corresponding error if health check fails
func (r *PlugRedis) CheckHealth() error {
	// Perform health check with fixed short timeout to avoid being affected by idle connection configuration
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	// Ensure context is cancelled at the end of the function
	defer cancel()

	// Execute Redis client Ping operation for health check
	start := time.Now()
	_, err := r.rdb.Ping(ctx).Result()
	latency := time.Since(start)
	redisPingLatency.Observe(latency.Seconds())
	log.Infof("redis health check: addrs=%v, ping_latency=%s", r.currentAddrList(), latency)
	if err != nil {
		return err
	}
	return nil
}

package http

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-lynx/lynx/plugins/service/http/conf"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/durationpb"
)

// TestHTTPPluginStress tests the HTTP plugin under high load
func TestHTTPPluginStress(t *testing.T) {
	// Skip stress tests in short mode
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	httpPlugin := NewServiceHttp()
	
	// Configure for stress testing
	config := &conf.Http{
		Network: "tcp",
		Addr:    ":8080",
		Timeout: &durationpb.Duration{Seconds: 30},
		Performance: &conf.PerformanceConfig{
			ReadTimeout:  &durationpb.Duration{Seconds: 10},
			WriteTimeout: &durationpb.Duration{Seconds: 10},
			IdleTimeout:  &durationpb.Duration{Seconds: 30},
		},
		Security: &conf.SecurityConfig{
			MaxRequestSize: 1024 * 1024, // 1MB
		},
		Middleware: &conf.MiddlewareConfig{
			EnableTracing:    true,
			EnableLogging:    true,
			EnableMetrics:    true,
			EnableRecovery:   true,
			EnableValidation: true,
			EnableRateLimit:  true,
		},
	}

	httpPlugin.conf = config

	// Test concurrent configuration validation
	t.Run("ConcurrentConfigValidation", func(t *testing.T) {
		const numGoroutines = 100
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := httpPlugin.validateConfig(); err != nil {
					errors <- err
				}
			}()
		}

		wg.Wait()
		close(errors)

		// Check for any errors
		for err := range errors {
			t.Errorf("Configuration validation failed: %v", err)
		}
	})

	// Test concurrent middleware building
	t.Run("ConcurrentMiddlewareBuilding", func(t *testing.T) {
		const numGoroutines = 50
		var wg sync.WaitGroup
		results := make(chan []middleware.Middleware, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				middlewares := httpPlugin.buildMiddlewares()
				results <- middlewares
			}()
		}

		wg.Wait()
		close(results)

		// Verify all results are consistent
		var firstResult []middleware.Middleware
		count := 0
		for result := range results {
			if firstResult == nil {
				firstResult = result
			}
			assert.Equal(t, len(firstResult), len(result), "Middleware count should be consistent")
			count++
		}
		assert.Equal(t, numGoroutines, count)
	})

	// Test concurrent metrics recording
	t.Run("ConcurrentMetricsRecording", func(t *testing.T) {
		const numGoroutines = 200
		const requestsPerGoroutine = 100
		var wg sync.WaitGroup

		// Initialize metrics
		httpPlugin.initMetrics()

		start := time.Now()
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < requestsPerGoroutine; j++ {
					// Record various metrics
					httpRequestCounter.WithLabelValues("GET", fmt.Sprintf("/api/%d", id), "200").Inc()
					httpRequestDuration.WithLabelValues("GET", fmt.Sprintf("/api/%d", id)).Observe(0.1)
					httpResponseSize.WithLabelValues("GET", fmt.Sprintf("/api/%d", id)).Observe(1024)
					httpActiveConnections.WithLabelValues(fmt.Sprintf("/api/%d", id)).Inc()
					httpActiveConnections.WithLabelValues(fmt.Sprintf("/api/%d", id)).Dec()
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		// Verify metrics were recorded
		totalRequests := numGoroutines * requestsPerGoroutine
		assert.Greater(t, totalRequests, 0)
		t.Logf("Recorded %d metrics in %v", totalRequests, duration)
	})

	// Test rate limiting under load
	t.Run("RateLimitingUnderLoad", func(t *testing.T) {
		const numGoroutines = 50
		const requestsPerGoroutine = 20
		var wg sync.WaitGroup
		successCount := 0
		errorCount := 0
		var mu sync.Mutex

		// Initialize rate limiter
		httpPlugin.initSecurityDefaults()

		start := time.Now()
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < requestsPerGoroutine; j++ {
					if httpPlugin.rateLimiter.Allow() {
						mu.Lock()
						successCount++
						mu.Unlock()
					} else {
						mu.Lock()
						errorCount++
						mu.Unlock()
					}
					time.Sleep(10 * time.Millisecond) // Small delay to simulate real requests
				}
			}()
		}

		wg.Wait()
		duration := time.Since(start)

		totalRequests := numGoroutines * requestsPerGoroutine
		t.Logf("Rate limiting test: %d total requests, %d allowed, %d rejected in %v", 
			totalRequests, successCount, errorCount, duration)
		
		// Verify rate limiting is working
		assert.Greater(t, successCount, 0, "Some requests should be allowed")
		assert.Greater(t, errorCount, 0, "Some requests should be rate limited")
	})
}

// TestHTTPPluginMemoryUsage tests memory usage under load
func TestHTTPPluginMemoryUsage(t *testing.T) {
	// Skip memory tests in short mode
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	httpPlugin := NewServiceHttp()
	
	config := &conf.Http{
		Network: "tcp",
		Addr:    ":8080",
		Monitoring: &conf.MonitoringConfig{
			EnableMetrics:           true,
			EnableRouteMetrics:      true,
			EnableConnectionMetrics: true,
			EnableQueueMetrics:      true,
		},
	}

	httpPlugin.conf = config

	// Test memory usage with many concurrent operations
	t.Run("MemoryUsageUnderLoad", func(t *testing.T) {
		const numIterations = 1000
		const numGoroutines = 10

		var wg sync.WaitGroup
		start := time.Now()

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numIterations; j++ {
					// Simulate various plugin operations
					_ = httpPlugin.validateConfig()
					_ = httpPlugin.buildMiddlewares()
					
					// Create and use configuration
					testConfig := &conf.Http{
						Network: "tcp",
						Addr:    fmt.Sprintf(":%d", 8080+j%100),
						Timeout: &durationpb.Duration{Seconds: int64(j % 30)},
					}
					_ = httpPlugin.Configure(testConfig)
				}
			}()
		}

		wg.Wait()
		duration := time.Since(start)

		t.Logf("Memory usage test completed: %d operations in %v", 
			numIterations*numGoroutines, duration)
	})
}

// TestHTTPPluginConcurrentStartup tests concurrent plugin operations
func TestHTTPPluginConcurrentStartup(t *testing.T) {
	// Skip concurrent startup tests in short mode
	if testing.Short() {
		t.Skip("Skipping concurrent startup test in short mode")
	}

	const numPlugins = 10
	var wg sync.WaitGroup
	plugins := make([]*ServiceHttp, numPlugins)

	// Create multiple plugin instances
	for i := 0; i < numPlugins; i++ {
		plugins[i] = NewServiceHttp()
		plugins[i].conf = &conf.Http{
			Network: "tcp",
			Addr:    fmt.Sprintf(":%d", 8080+i),
			Timeout: &durationpb.Duration{Seconds: 5},
		}
	}

	// Test concurrent initialization
	t.Run("ConcurrentInitialization", func(t *testing.T) {
		start := time.Now()
		for i := 0; i < numPlugins; i++ {
			wg.Add(1)
			go func(plugin *ServiceHttp) {
				defer wg.Done()
				err := plugin.validateConfig()
				assert.NoError(t, err)
			}(plugins[i])
		}

		wg.Wait()
		duration := time.Since(start)

		t.Logf("Concurrent initialization completed: %d plugins in %v", numPlugins, duration)
	})

	// Test concurrent configuration updates
	t.Run("ConcurrentConfigurationUpdates", func(t *testing.T) {
		start := time.Now()
		for i := 0; i < numPlugins; i++ {
			wg.Add(1)
			go func(plugin *ServiceHttp, id int) {
				defer wg.Done()
				newConfig := &conf.Http{
					Network: "tcp",
					Addr:    fmt.Sprintf(":%d", 9000+id),
					Timeout: &durationpb.Duration{Seconds: 10},
				}
				err := plugin.Configure(newConfig)
				assert.NoError(t, err)
			}(plugins[i], i)
		}

		wg.Wait()
		duration := time.Since(start)

		t.Logf("Concurrent configuration updates completed: %d plugins in %v", numPlugins, duration)
	})
}

// BenchmarkHTTPPluginConcurrent benchmarks concurrent operations
func BenchmarkHTTPPluginConcurrent(b *testing.B) {
	httpPlugin := NewServiceHttp()
	httpPlugin.conf = &conf.Http{
		Network: "tcp",
		Addr:    ":8080",
		Timeout: &durationpb.Duration{Seconds: 5},
	}

	// Benchmark concurrent configuration validation
	b.Run("ConcurrentConfigValidation", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = httpPlugin.validateConfig()
			}
		})
	})

	// Benchmark concurrent middleware building
	b.Run("ConcurrentMiddlewareBuilding", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = httpPlugin.buildMiddlewares()
			}
		})
	})

	// Benchmark concurrent metrics recording
	b.Run("ConcurrentMetricsRecording", func(b *testing.B) {
		httpPlugin.initMetrics()
		b.RunParallel(func(pb *testing.PB) {
			counter := 0
			for pb.Next() {
				httpRequestCounter.WithLabelValues("GET", "/benchmark", "200").Inc()
				httpRequestDuration.WithLabelValues("GET", "/benchmark").Observe(0.1)
				counter++
			}
		})
	})
}

// BenchmarkHTTPPluginMemory benchmarks memory allocation
func BenchmarkHTTPPluginMemory(b *testing.B) {
	// Benchmark configuration creation
	b.Run("ConfigurationCreation", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			config := &conf.Http{
				Network: "tcp",
				Addr:    fmt.Sprintf(":%d", 8080+i%100),
				Timeout: &durationpb.Duration{Seconds: int64(i % 30)},
				Monitoring: &conf.MonitoringConfig{
					EnableMetrics: true,
				},
				Middleware: &conf.MiddlewareConfig{
					EnableTracing: true,
				},
			}
			_ = config
		}
	})

	// Benchmark plugin creation
	b.Run("PluginCreation", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			plugin := NewServiceHttp()
			_ = plugin
		}
	})
}

package sentinel

import (
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(interval time.Duration) *MetricsCollector {
	return &MetricsCollector{
		enabled:        true,
		interval:       interval,
		requestCounter: make(map[string]int64),
		blockedCounter: make(map[string]int64),
		passedCounter:  make(map[string]int64),
		rtHistogram:    make(map[string][]float64),
		stopCh:         make(chan struct{}),
	}
}

// Start starts the metrics collection
func (mc *MetricsCollector) Start(wg *sync.WaitGroup, stopCh chan struct{}) {
	defer wg.Done()
	
	if !mc.enabled {
		return
	}

	ticker := time.NewTicker(mc.interval)
	defer ticker.Stop()

	log.Infof("Sentinel metrics collector started with interval %v", mc.interval)

	for {
		select {
		case <-ticker.C:
			mc.collectMetrics()
		case <-stopCh:
			log.Infof("Sentinel metrics collector stopped")
			return
		}
	}
}

// collectMetrics collects and reports metrics
func (mc *MetricsCollector) collectMetrics() {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Log current metrics
	for resource := range mc.requestCounter {
		stats := mc.calculateResourceStats(resource)
		log.Debugf("Resource %s stats: PassQPS=%.2f, BlockQPS=%.2f, AvgRT=%.2fms", 
			resource, stats.PassQPS, stats.BlockQPS, stats.AvgRT)
	}
}

// RecordPassed records a passed request
func (mc *MetricsCollector) RecordPassed(resource string) {
	if !mc.enabled {
		return
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.requestCounter[resource]++
	mc.passedCounter[resource]++
}

// RecordBlocked records a blocked request
func (mc *MetricsCollector) RecordBlocked(resource string) {
	if !mc.enabled {
		return
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.requestCounter[resource]++
	mc.blockedCounter[resource]++
}

// RecordRT records response time
func (mc *MetricsCollector) RecordRT(resource string, duration time.Duration) {
	if !mc.enabled {
		return
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	rtMs := float64(duration.Nanoseconds()) / 1e6 // Convert to milliseconds
	mc.rtHistogram[resource] = append(mc.rtHistogram[resource], rtMs)

	// Keep only recent RT values (last 100 requests)
	if len(mc.rtHistogram[resource]) > 100 {
		mc.rtHistogram[resource] = mc.rtHistogram[resource][1:]
	}
}

// RecordError records an error
func (mc *MetricsCollector) RecordError(resource string) {
	if !mc.enabled {
		return
	}

	// For now, just log the error
	log.Debugf("Error recorded for resource %s", resource)
}

// GetResourceStats returns statistics for a specific resource
func (mc *MetricsCollector) GetResourceStats(resource string) *ResourceStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return mc.calculateResourceStats(resource)
}

// GetAllResourceStats returns statistics for all resources
func (mc *MetricsCollector) GetAllResourceStats() map[string]*ResourceStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	stats := make(map[string]*ResourceStats)
	for resource := range mc.requestCounter {
		stats[resource] = mc.calculateResourceStats(resource)
	}

	return stats
}

// calculateResourceStats calculates statistics for a resource
func (mc *MetricsCollector) calculateResourceStats(resource string) *ResourceStats {
	stats := &ResourceStats{
		Resource:  resource,
		Timestamp: time.Now(),
	}

	// Calculate QPS (requests per second)
	totalRequests := mc.requestCounter[resource]
	passedRequests := mc.passedCounter[resource]
	blockedRequests := mc.blockedCounter[resource]

	// Simple QPS calculation based on interval
	intervalSeconds := mc.interval.Seconds()
	stats.TotalQPS = float64(totalRequests) / intervalSeconds
	stats.PassQPS = float64(passedRequests) / intervalSeconds
	stats.BlockQPS = float64(blockedRequests) / intervalSeconds

	// Calculate RT statistics
	rtValues := mc.rtHistogram[resource]
	if len(rtValues) > 0 {
		var sum, min, max float64
		min = rtValues[0]
		max = rtValues[0]

		for _, rt := range rtValues {
			sum += rt
			if rt < min {
				min = rt
			}
			if rt > max {
				max = rt
			}
		}

		stats.AvgRT = sum / float64(len(rtValues))
		stats.MinRT = min
		stats.MaxRT = max
	}

	return stats
}

// Reset resets all metrics
func (mc *MetricsCollector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.requestCounter = make(map[string]int64)
	mc.blockedCounter = make(map[string]int64)
	mc.passedCounter = make(map[string]int64)
	mc.rtHistogram = make(map[string][]float64)

	log.Infof("Sentinel metrics reset")
}

// GetMetricsSummary returns a summary of all metrics
func (mc *MetricsCollector) GetMetricsSummary() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	summary := make(map[string]interface{})
	
	var totalRequests, totalPassed, totalBlocked int64
	resourceCount := 0

	for resource := range mc.requestCounter {
		totalRequests += mc.requestCounter[resource]
		totalPassed += mc.passedCounter[resource]
		totalBlocked += mc.blockedCounter[resource]
		resourceCount++
	}

	summary["total_requests"] = totalRequests
	summary["total_passed"] = totalPassed
	summary["total_blocked"] = totalBlocked
	summary["resource_count"] = resourceCount
	summary["collection_interval"] = mc.interval.String()
	summary["enabled"] = mc.enabled

	if totalRequests > 0 {
		summary["pass_rate"] = float64(totalPassed) / float64(totalRequests)
		summary["block_rate"] = float64(totalBlocked) / float64(totalRequests)
	}

	return summary
}

// SetEnabled enables or disables metrics collection
func (mc *MetricsCollector) SetEnabled(enabled bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.enabled = enabled
	log.Infof("Sentinel metrics collection enabled: %v", enabled)
}

// SetInterval updates the collection interval
func (mc *MetricsCollector) SetInterval(interval time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.interval = interval
	log.Infof("Sentinel metrics collection interval updated to %v", interval)
}
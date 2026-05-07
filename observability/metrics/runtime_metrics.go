package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Runtime lifecycle metrics — registered with the default prometheus registry so
// they appear automatically in the unified /metrics endpoint (see handler.go).
var (
	// pluginStartupDuration tracks how long each plugin's Initialize step takes.
	// High values here indicate slow plugin boot, not slow requests.
	pluginStartupDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lynx_runtime_startup_duration_seconds",
		Help:    "Plugin initialization duration in seconds.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"plugin_id", "plugin_name"})

	// pluginCleanupDuration tracks how long CleanupResources takes per plugin.
	pluginCleanupDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lynx_plugin_cleanup_duration_seconds",
		Help:    "Plugin resource cleanup duration in seconds.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
	}, []string{"plugin_id"})

	// resourceCleanupPanicsTotal counts panics recovered inside cleanupResourceGracefully.
	// A non-zero value means a resource's shutdown method panicked — investigate immediately.
	resourceCleanupPanicsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_resource_cleanup_panics_total",
		Help: "Total panics recovered during individual resource cleanup.",
	}, []string{"resource_name"})

	// resourceCleanupSlowTotal counts cleanup calls that exceeded 5 seconds.
	resourceCleanupSlowTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "lynx_resource_cleanup_slow_total",
		Help: "Total resource cleanup calls that exceeded 5 seconds.",
	}, []string{"resource_name"})
)

// RecordPluginStartupDuration records how long a plugin's Initialize step took.
func RecordPluginStartupDuration(pluginID, pluginName string, seconds float64) {
	pluginStartupDuration.WithLabelValues(pluginID, pluginName).Observe(seconds)
}

// RecordPluginCleanupDuration records how long CleanupResources took for a plugin.
func RecordPluginCleanupDuration(pluginID string, seconds float64) {
	pluginCleanupDuration.WithLabelValues(pluginID).Observe(seconds)
}

// RecordResourceCleanupPanic increments the panic counter for a named resource.
func RecordResourceCleanupPanic(resourceName string) {
	resourceCleanupPanicsTotal.WithLabelValues(resourceName).Inc()
}

// RecordResourceCleanupSlow increments the slow-cleanup counter for a named resource.
func RecordResourceCleanupSlow(resourceName string) {
	resourceCleanupSlowTotal.WithLabelValues(resourceName).Inc()
}

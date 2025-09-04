package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// HealthProvider abstracts health data providers for various plugins
// Labels returns key-value pairs that will be attached as additional labels to health metrics (besides standard labels).
type HealthProvider interface {
	IsHealthy() bool
	ErrorCount() int
	LastCheck() time.Time
	Labels() map[string]string
}

var (
	// Global registry (unified registration for all plugins)
	registry = prometheus.NewRegistry()
	// Additional aggregation sources (for integrating third-party/submodule registries that cannot directly depend on this package)
	extraGatherers []prometheus.Gatherer
)

// RegisterGatherer allows plugins/modules that don't import this package to inject their private registries during upper-level assembly
// For example: some plugins maintain independent *prometheus.Registry internally
func RegisterGatherer(g prometheus.Gatherer) {
	if g == nil {
		return
	}
	extraGatherers = append(extraGatherers, g)
}

// RegisterCollector registers a Collector to the global registry
func RegisterCollector(c prometheus.Collector) error {
	return registry.Register(c)
}

// MustRegister registers Collectors in batch (will panic if registration fails)
func MustRegister(cs ...prometheus.Collector) {
	registry.MustRegister(cs...)
}

// healthCollector adapts HealthProvider to three metrics:
// - plugin_health_status{plugin,instance,version,...} 0/1
// - plugin_health_error_count{plugin,instance,version,...}
// - plugin_health_last_check_timestamp_seconds{plugin,instance,version,...}
type healthCollector struct {
	plugin   string
	instance string
	version  string
	hp       HealthProvider

	statusDesc     *prometheus.Desc
	errorCountDesc *prometheus.Desc
	lastCheckDesc  *prometheus.Desc
}

// NewHealthCollector creates a health metrics collector
func NewHealthCollector(plugin, instance, version string, hp HealthProvider) prometheus.Collector {
	// Unified standard labels
	constLabels := prometheus.Labels{
		"plugin":   plugin,
		"instance": instance,
		"version":  version,
	}
	return &healthCollector{
		plugin:   plugin,
		instance: instance,
		version:  version,
		hp:       hp,
		statusDesc: prometheus.NewDesc(
			"plugin_health_status",
			"Health status of a plugin instance (1=healthy, 0=unhealthy)",
			nil, constLabels,
		),
		errorCountDesc: prometheus.NewDesc(
			"plugin_health_error_count",
			"Consecutive error count observed by health checker",
			nil, constLabels,
		),
		lastCheckDesc: prometheus.NewDesc(
			"plugin_health_last_check_timestamp_seconds",
			"Unix timestamp of the last health check",
			nil, constLabels,
		),
	}
}

func (c *healthCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.statusDesc
	ch <- c.errorCountDesc
	ch <- c.lastCheckDesc
}

func (c *healthCollector) Collect(ch chan<- prometheus.Metric) {
	healthy := 0.0
	if c.hp.IsHealthy() {
		healthy = 1.0
	}
	// Note: Additional labels are not expanded as dynamic labels in this version to avoid high cardinality; can be extended as Info metrics.
	ch <- prometheus.MustNewConstMetric(c.statusDesc, prometheus.GaugeValue, healthy)
	ch <- prometheus.MustNewConstMetric(c.errorCountDesc, prometheus.GaugeValue, float64(c.hp.ErrorCount()))
	ch <- prometheus.MustNewConstMetric(c.lastCheckDesc, prometheus.GaugeValue, float64(c.hp.LastCheck().Unix()))
}

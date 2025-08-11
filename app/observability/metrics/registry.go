package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// HealthProvider 抽象各插件健康数据的提供方
// Labels 返回的键值会作为额外的标签附加到健康指标上（除标准标签外）。
type HealthProvider interface {
	IsHealthy() bool
	ErrorCount() int
	LastCheck() time.Time
	Labels() map[string]string
}

var (
	// 全局注册表（各插件统一注册）
	registry = prometheus.NewRegistry()
	// 额外的聚合源（用于接入无法直接依赖本包的第三方/子模块注册表）
	extraGatherers []prometheus.Gatherer
)

// RegisterGatherer 允许在不引入该包的插件/模块，通过上层装配时注入其私有注册表
// 例如：某些插件内部维护了独立的 *prometheus.Registry
func RegisterGatherer(g prometheus.Gatherer) {
	if g == nil {
		return
	}
	extraGatherers = append(extraGatherers, g)
}

// RegisterCollector 向全局注册表注册一个 Collector
func RegisterCollector(c prometheus.Collector) error {
	return registry.Register(c)
}

// MustRegister 批量注册 Collector（注册失败将 panic）
func MustRegister(cs ...prometheus.Collector) {
	registry.MustRegister(cs...)
}

// healthCollector 将 HealthProvider 适配为三个指标：
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

// NewHealthCollector 创建健康指标采集器
func NewHealthCollector(plugin, instance, version string, hp HealthProvider) prometheus.Collector {
	// 统一标准标签
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
	// 注意：额外标签在此版本不展开为动态标签，避免高基数；可扩展为 Info 指标。
	ch <- prometheus.MustNewConstMetric(c.statusDesc, prometheus.GaugeValue, healthy)
	ch <- prometheus.MustNewConstMetric(c.errorCountDesc, prometheus.GaugeValue, float64(c.hp.ErrorCount()))
	ch <- prometheus.MustNewConstMetric(c.lastCheckDesc, prometheus.GaugeValue, float64(c.hp.LastCheck().Unix()))
}

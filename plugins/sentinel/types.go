package sentinel

import (
	"net/http"
	"sync"
	"time"

	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/go-lynx/lynx/plugins"
)

// FlowRule represents a flow control rule
type FlowRule struct {
	Resource               string  `yaml:"resource" json:"resource"`
	TokenCalculateStrategy int32   `yaml:"token_calculate_strategy" json:"token_calculate_strategy"`
	ControlBehavior        int32   `yaml:"control_behavior" json:"control_behavior"`
	Threshold              float64 `yaml:"threshold" json:"threshold"`
	RelationStrategy       int32   `yaml:"relation_strategy" json:"relation_strategy"`
	RefResource            string  `yaml:"ref_resource" json:"ref_resource"`
	MaxQueueingTimeMs      uint32  `yaml:"max_queueing_time_ms" json:"max_queueing_time_ms"`
	WarmUpPeriodSec        uint32  `yaml:"warm_up_period_sec" json:"warm_up_period_sec"`
	WarmUpColdFactor       uint32  `yaml:"warm_up_cold_factor" json:"warm_up_cold_factor"`
	StatIntervalInMs       uint32  `yaml:"stat_interval_in_ms" json:"stat_interval_in_ms"`
	LowMemUsageThreshold   int64   `yaml:"low_mem_usage_threshold" json:"low_mem_usage_threshold"`
	HighMemUsageThreshold  int64   `yaml:"high_mem_usage_threshold" json:"high_mem_usage_threshold"`
	MemLowWaterMarkBytes   int64   `yaml:"mem_low_water_mark_bytes" json:"mem_low_water_mark_bytes"`
	MemHighWaterMarkBytes  int64   `yaml:"mem_high_water_mark_bytes" json:"mem_high_water_mark_bytes"`
}

// CircuitBreakerRule represents a circuit breaker rule
type CircuitBreakerRule struct {
	Resource                     string  `yaml:"resource" json:"resource"`
	Strategy                     int32   `yaml:"strategy" json:"strategy"`
	RetryTimeoutMs               uint32  `yaml:"retry_timeout_ms" json:"retry_timeout_ms"`
	MinRequestAmount             uint64  `yaml:"min_request_amount" json:"min_request_amount"`
	StatIntervalMs               uint32  `yaml:"stat_interval_ms" json:"stat_interval_ms"`
	StatSlidingWindowBucketCount uint32  `yaml:"stat_sliding_window_bucket_count" json:"stat_sliding_window_bucket_count"`
	Threshold                    float64 `yaml:"threshold" json:"threshold"`
	ProbeNum                     uint64  `yaml:"probe_num" json:"probe_num"`
}

// SystemRule represents a system protection rule
type SystemRule struct {
	MetricType   int32   `yaml:"metric_type" json:"metric_type"`
	TriggerCount float64 `yaml:"trigger_count" json:"trigger_count"`
	Strategy     int32   `yaml:"strategy" json:"strategy"`
}

// SentinelConfig represents the configuration for Sentinel plugin
type SentinelConfig struct {
	Enabled     bool                   `yaml:"enabled" json:"enabled"`
	AppName     string                 `yaml:"app_name" json:"app_name"`
	LogLevel    string                 `yaml:"log_level" json:"log_level"`
	LogDir      string                 `yaml:"log_dir" json:"log_dir"`
	FlowRules   []FlowRule            `yaml:"flow_rules" json:"flow_rules"`
	CBRules     []CircuitBreakerRule  `yaml:"circuit_breaker_rules" json:"circuit_breaker_rules"`
	SystemRules []SystemRule          `yaml:"system_rules" json:"system_rules"`
	Metrics     MetricsConfig         `yaml:"metrics" json:"metrics"`
	Dashboard   DashboardConfig       `yaml:"dashboard" json:"dashboard"`
	DataSource  DataSourceConfig      `yaml:"data_source" json:"data_source"`
	WarmUp      WarmUpConfig          `yaml:"warm_up" json:"warm_up"`
	Advanced    AdvancedConfig        `yaml:"advanced" json:"advanced"`
}

// MetricsConfig represents metrics configuration
type MetricsConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Interval string `yaml:"interval" json:"interval"`
}

// DashboardConfig represents dashboard configuration
type DashboardConfig struct {
	Enabled bool  `yaml:"enabled" json:"enabled"`
	Port    int32 `yaml:"port" json:"port"`
}

// DataSourceConfig represents data source configuration
type DataSourceConfig struct {
	Type string `yaml:"type" json:"type"`
	File struct {
		FlowRulesPath           string `yaml:"flow_rules_path" json:"flow_rules_path"`
		CircuitBreakerRulesPath string `yaml:"circuit_breaker_rules_path" json:"circuit_breaker_rules_path"`
		SystemRulesPath         string `yaml:"system_rules_path" json:"system_rules_path"`
	} `yaml:"file" json:"file"`
}

// WarmUpConfig represents warm-up configuration
type WarmUpConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Duration string `yaml:"duration" json:"duration"`
}

// AdvancedConfig represents advanced configuration
type AdvancedConfig struct {
	StatIntervalMs uint32 `yaml:"stat_interval_ms" json:"stat_interval_ms"`
	MetricLogFlushIntervalSec uint32 `yaml:"metric_log_flush_interval_sec" json:"metric_log_flush_interval_sec"`
}

// PlugSentinel represents a Sentinel plugin instance
type PlugSentinel struct {
	// Inherits from base plugin
	*plugins.BasePlugin
	
	// Sentinel configuration
	conf *SentinelConfig
	
	// Sentinel initialized flag
	sentinelInitialized bool
	
	// Rule managers (these are not exposed as public types in sentinel-golang)
	// We'll manage rules through the public APIs instead
	
	// Metrics and monitoring
	metricsCollector *MetricsCollector
	dashboardServer  *DashboardServer
	
	// Internal state
	isInitialized bool
	mu            sync.RWMutex
	
	// Background tasks control
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// MetricsCollector handles metrics collection for Sentinel
type MetricsCollector struct {
	enabled          bool
	interval         time.Duration
	requestCounter   map[string]int64
	blockedCounter   map[string]int64
	passedCounter    map[string]int64
	rtHistogram      map[string][]float64
	mu               sync.RWMutex
	stopCh           chan struct{}
	wg               sync.WaitGroup
}

// DashboardServer provides a web dashboard for monitoring
type DashboardServer struct {
	enabled          bool
	port             int32
	server           *http.Server // HTTP server instance
	stopCh           chan struct{}
	metricsCollector *MetricsCollector
}

// ResourceStats represents statistics for a specific resource
type ResourceStats struct {
	Resource     string    `json:"resource"`
	PassQPS      float64   `json:"pass_qps"`
	BlockQPS     float64   `json:"block_qps"`
	TotalQPS     float64   `json:"total_qps"`
	AvgRT        float64   `json:"avg_rt"`
	MinRT        float64   `json:"min_rt"`
	MaxRT        float64   `json:"max_rt"`
	ExceptionQPS float64   `json:"exception_qps"`
	Timestamp    time.Time `json:"timestamp"`
}

// FlowControlResult represents the result of flow control check
type FlowControlResult struct {
	Allowed   bool          `json:"allowed"`
	Resource  string        `json:"resource"`
	Rule      string        `json:"rule,omitempty"`
	Reason    string        `json:"reason,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState struct {
	Resource    string                        `json:"resource"`
	State       circuitbreaker.State          `json:"state"`
	Rule        *circuitbreaker.Rule          `json:"rule"`
	LastChange  time.Time                     `json:"last_change"`
	ErrorCount  int64                         `json:"error_count"`
	RequestCount int64                        `json:"request_count"`
}

// SentinelMiddleware provides middleware integration for various frameworks
type SentinelMiddleware struct {
	plugin *PlugSentinel
}

// RequestContext holds context information for a request
type RequestContext struct {
	Resource    string
	StartTime   time.Time
	Entry       interface{} // Sentinel entry (using interface{} since SentinelEntry is not exported)
	Metadata    map[string]interface{}
}
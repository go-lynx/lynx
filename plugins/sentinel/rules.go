package sentinel

import (
	"fmt"

	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/core/system"
	"github.com/go-lynx/lynx/app/log"
)

// loadFlowRules loads flow control rules into Sentinel
func (s *PlugSentinel) loadFlowRules() error {
	var rules []*flow.Rule

	// Load configured flow rules
	for _, confRule := range s.conf.FlowRules {
		rule := &flow.Rule{
			Resource:               confRule.Resource,
			TokenCalculateStrategy: flow.TokenCalculateStrategy(confRule.TokenCalculateStrategy),
			ControlBehavior:        flow.ControlBehavior(confRule.ControlBehavior),
			Threshold:              confRule.Threshold,
			StatIntervalInMs:       confRule.StatIntervalInMs,
		}

		// Set warm-up period if specified
		if confRule.WarmUpPeriodSec > 0 {
			rule.WarmUpPeriodSec = confRule.WarmUpPeriodSec
		}

		// Set max queueing time if specified
		if confRule.MaxQueueingTimeMs > 0 {
			rule.MaxQueueingTimeMs = confRule.MaxQueueingTimeMs
		}

		rules = append(rules, rule)
	}

	// Add default rule if no specific rules are configured
	if len(rules) == 0 {
		defaultRule := &flow.Rule{
			Resource:               "default",
			TokenCalculateStrategy: flow.Direct,
			ControlBehavior:        flow.Reject,
			Threshold:              100.0, // Default QPS limit
			StatIntervalInMs:       1000,
		}
		rules = append(rules, defaultRule)
	}

	// Load rules into Sentinel
	if len(rules) > 0 {
		_, err := flow.LoadRules(rules)
		if err != nil {
			return fmt.Errorf("failed to load flow rules: %w", err)
		}
		log.Infof("Loaded %d flow control rules", len(rules))
	}

	return nil
}

// loadCircuitBreakerRules loads circuit breaker rules into Sentinel
func (s *PlugSentinel) loadCircuitBreakerRules() error {
	var rules []*circuitbreaker.Rule

	// Load configured circuit breaker rules
	for _, confRule := range s.conf.CBRules {

		rule := &circuitbreaker.Rule{
			Resource:         confRule.Resource,
			Strategy:         circuitbreaker.Strategy(confRule.Strategy),
			RetryTimeoutMs:   confRule.RetryTimeoutMs,
			MinRequestAmount: confRule.MinRequestAmount,
			StatIntervalMs:   confRule.StatIntervalMs,
			Threshold:        confRule.Threshold,
		}

		rules = append(rules, rule)
	}

	// Add default circuit breaker rule if no specific rules are configured
	if len(rules) == 0 {
		defaultRule := &circuitbreaker.Rule{
			Resource:         "default",
			Strategy:         circuitbreaker.ErrorRatio,
			RetryTimeoutMs:   5000, // 5 seconds default
			MinRequestAmount: 10,   // Default minimum request amount
			StatIntervalMs:   1000, // 1 second default
			Threshold:        0.5,  // 50% error ratio threshold
		}
		rules = append(rules, defaultRule)
	}

	// Load rules into Sentinel
	if len(rules) > 0 {
		_, err := circuitbreaker.LoadRules(rules)
		if err != nil {
			return fmt.Errorf("failed to load circuit breaker rules: %w", err)
		}
		log.Infof("Loaded %d circuit breaker rules", len(rules))
	}

	return nil
}

// loadSystemRules loads system protection rules into Sentinel
func (s *PlugSentinel) loadSystemRules() error {
	var rules []*system.Rule

	// Load configured system rules
	for _, confRule := range s.conf.SystemRules {
		rule := &system.Rule{
			MetricType:   system.MetricType(confRule.MetricType),
			TriggerCount: confRule.TriggerCount,
		}

		rules = append(rules, rule)
	}

	// Load rules into Sentinel
	if len(rules) > 0 {
		_, err := system.LoadRules(rules)
		if err != nil {
			return fmt.Errorf("failed to load system rules: %w", err)
		}
		log.Infof("Loaded %d system protection rules", len(rules))
	}

	return nil
}

// parseControlBehavior converts string control behavior to Sentinel enum
func (s *PlugSentinel) parseControlBehavior(behavior string) flow.ControlBehavior {
	switch behavior {
	case "reject":
		return flow.Reject
	case "throttling":
		return flow.Throttling
	default:
		return flow.Reject
	}
}

// parseCircuitBreakerStrategy converts string strategy to Sentinel enum
func (s *PlugSentinel) parseCircuitBreakerStrategy(strategy string) circuitbreaker.Strategy {
	switch strategy {
	case "slow_request_ratio":
		return circuitbreaker.SlowRequestRatio
	case "error_ratio":
		return circuitbreaker.ErrorRatio
	case "error_count":
		return circuitbreaker.ErrorCount
	default:
		return circuitbreaker.ErrorRatio
	}
}

// parseSystemMetricType converts string metric type to Sentinel enum
func (s *PlugSentinel) parseSystemMetricType(metricType string) system.MetricType {
	switch metricType {
	case "load":
		return system.Load
	case "cpu_usage":
		return system.CpuUsage
	case "inbound_qps":
		return system.InboundQPS
	case "concurrency":
		return system.Concurrency
	default:
		return system.Load
	}
}

// GetFlowRules returns current flow control rules
func (s *PlugSentinel) GetFlowRules() []flow.Rule {
	return flow.GetRules()
}

// GetCircuitBreakerRules returns current circuit breaker rules
func (s *PlugSentinel) GetCircuitBreakerRules() []circuitbreaker.Rule {
	return circuitbreaker.GetRules()
}

// GetSystemRules returns current system protection rules
func (s *PlugSentinel) GetSystemRules() []system.Rule {
	return system.GetRules()
}

// AddFlowRule adds a new flow control rule dynamically
func (s *PlugSentinel) AddFlowRule(resource string, qpsLimit float64, controlBehavior string) error {
	rule := &flow.Rule{
		Resource:               resource,
		TokenCalculateStrategy: flow.Direct,
		ControlBehavior:        s.parseControlBehavior(controlBehavior),
		Threshold:              qpsLimit,
		StatIntervalInMs:       1000,
	}

	currentRules := s.GetFlowRules()
	var rules []*flow.Rule
	for _, r := range currentRules {
		rules = append(rules, &r)
	}
	rules = append(rules, rule)
	_, err := flow.LoadRules(rules)
	if err != nil {
		return fmt.Errorf("failed to add flow rule: %w", err)
	}

	log.Infof("Added flow rule for resource %s with QPS limit %f", resource, qpsLimit)
	return nil
}

// RemoveFlowRule removes a flow control rule by resource name
func (s *PlugSentinel) RemoveFlowRule(resource string) error {
	currentRules := s.GetFlowRules()
	var newRules []*flow.Rule

	for _, rule := range currentRules {
		if rule.Resource != resource {
			newRules = append(newRules, &rule)
		}
	}

	_, err := flow.LoadRules(newRules)
	if err != nil {
		return fmt.Errorf("failed to remove flow rule: %w", err)
	}

	log.Infof("Removed flow rule for resource %s", resource)
	return nil
}

// AddCircuitBreakerRule adds a new circuit breaker rule dynamically
func (s *PlugSentinel) AddCircuitBreakerRule(resource string, strategy int32, threshold float64, minRequestAmount uint64) error {
	rule := &circuitbreaker.Rule{
		Resource:         resource,
		Strategy:         circuitbreaker.Strategy(strategy),
		RetryTimeoutMs:   5000, // 5 seconds default
		MinRequestAmount: minRequestAmount,
		StatIntervalMs:   1000, // 1 second default
		Threshold:        threshold,
	}

	currentRules := s.GetCircuitBreakerRules()
	var rules []*circuitbreaker.Rule
	for _, r := range currentRules {
		rules = append(rules, &r)
	}
	rules = append(rules, rule)
	_, err := circuitbreaker.LoadRules(rules)
	if err != nil {
		return fmt.Errorf("failed to add circuit breaker rule: %w", err)
	}

	log.Infof("Added circuit breaker rule for resource %s with strategy %s", resource, strategy)
	return nil
}

// RemoveCircuitBreakerRule removes a circuit breaker rule by resource name
func (s *PlugSentinel) RemoveCircuitBreakerRule(resource string) error {
	currentRules := s.GetCircuitBreakerRules()
	var newRules []*circuitbreaker.Rule

	for _, rule := range currentRules {
		if rule.Resource != resource {
			newRules = append(newRules, &rule)
		}
	}

	_, err := circuitbreaker.LoadRules(newRules)
	if err != nil {
		return fmt.Errorf("failed to remove circuit breaker rule: %w", err)
	}

	log.Infof("Removed circuit breaker rule for resource %s", resource)
	return nil
}
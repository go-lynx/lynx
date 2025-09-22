// Package grpc provides load balancing functionality for gRPC clients
package grpc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-kratos/kratos/v2/selector/p2c"
	"github.com/go-kratos/kratos/v2/selector/random"
	"github.com/go-kratos/kratos/v2/selector/wrr"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins/service/grpc/filter"
)

// LoadBalancerType defines the type of load balancing strategy
type LoadBalancerType string

const (
	// LoadBalancerRoundRobin uses round-robin load balancing
	LoadBalancerRoundRobin LoadBalancerType = "round_robin"
	// LoadBalancerRandom uses random load balancing
	LoadBalancerRandom LoadBalancerType = "random"
	// LoadBalancerWeightedRoundRobin uses weighted round-robin load balancing
	LoadBalancerWeightedRoundRobin LoadBalancerType = "weighted_round_robin"
	// LoadBalancerP2C uses power of two choices load balancing
	LoadBalancerP2C LoadBalancerType = "p2c"
	// LoadBalancerConsistentHash uses consistent hash load balancing
	LoadBalancerConsistentHash LoadBalancerType = "consistent_hash"
)

// LoadBalancerConfig contains configuration for load balancing
type LoadBalancerConfig struct {
	Strategy LoadBalancerType  `json:"strategy"`
	Filters  []string          `json:"filters"`
	Metadata map[string]string `json:"metadata"`
}

// LoadBalancer manages load balancing for gRPC services
type LoadBalancer struct {
	discovery registry.Discovery
	selectors map[string]selector.Selector
	configs   map[string]*LoadBalancerConfig
	mu        sync.RWMutex
	metrics   *ClientMetrics
}

// NewLoadBalancer creates a new load balancer with the given discovery
func NewLoadBalancer(discovery registry.Discovery, metrics *ClientMetrics) *LoadBalancer {
	return &LoadBalancer{
		discovery: discovery,
		selectors: make(map[string]selector.Selector),
		configs:   make(map[string]*LoadBalancerConfig),
		metrics:   metrics,
	}
}

// ConfigureService configures load balancing for a specific service
func (lb *LoadBalancer) ConfigureService(serviceName string, config *LoadBalancerConfig) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Store configuration
	lb.configs[serviceName] = config

	// Create selector based on strategy
	sel, err := lb.createSelector(config)
	if err != nil {
		return fmt.Errorf("failed to create selector for service %s: %w", serviceName, err)
	}

	// Close existing selector if any
	if existingSel, exists := lb.selectors[serviceName]; exists {
		if closer, ok := existingSel.(interface{ Close() error }); ok {
			err := closer.Close()
			if err != nil {
				return err
			}
		}
	}

	lb.selectors[serviceName] = sel
	return nil
}

// SelectNode selects a node for the given service using the configured load balancing strategy
func (lb *LoadBalancer) SelectNode(ctx context.Context, serviceName string) (selector.Node, func(context.Context, selector.DoneInfo), error) {
	lb.mu.RLock()
	sel, exists := lb.selectors[serviceName]
	lb.mu.RUnlock()

	if !exists {
		// Use default random selector if no specific configuration
		sel = random.NewBuilder().Build()
		lb.mu.Lock()
		lb.selectors[serviceName] = sel
		lb.configs[serviceName] = &LoadBalancerConfig{
			Strategy: LoadBalancerRandom,
		}
		lb.mu.Unlock()
	}

	// Get service instances from discovery
	instances, err := lb.discovery.GetService(ctx, serviceName)
	if err != nil {
		if lb.metrics != nil {
			lb.metrics.RecordLoadBalancerError(serviceName, "discovery_failed")
		}
		return nil, nil, fmt.Errorf("failed to get service instances for %s: %w", serviceName, err)
	}

	if len(instances) == 0 {
		if lb.metrics != nil {
			lb.metrics.RecordLoadBalancerError(serviceName, "no_instances")
		}
		return nil, nil, fmt.Errorf("no instances available for service %s", serviceName)
	}

	// Convert registry instances to selector nodes
	nodes := make([]selector.Node, 0, len(instances))
	for _, instance := range instances {
		n := &registryNode{instance: instance}
		nodes = append(nodes, n)
	}

	// Apply nodes to selector
	if applier, ok := sel.(interface{ Apply([]selector.Node) }); ok {
		applier.Apply(nodes)
	}

	// Apply the selector
	selectedNode, done, err := sel.Select(ctx)
	if err != nil {
		if lb.metrics != nil {
			lb.metrics.RecordLoadBalancerError(serviceName, "selection_failed")
		}
		return nil, nil, fmt.Errorf("failed to select node for service %s: %w", serviceName, err)
	}

	if lb.metrics != nil {
		lb.metrics.RecordLoadBalancerSelection(serviceName, selectedNode.Address())
	}

	return selectedNode, done, nil
}

// GetServiceConfig returns the load balancer configuration for a service
func (lb *LoadBalancer) GetServiceConfig(serviceName string) *LoadBalancerConfig {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if config, exists := lb.configs[serviceName]; exists {
		// Return a copy to prevent external modification
		configCopy := *config
		if config.Metadata != nil {
			configCopy.Metadata = make(map[string]string)
			for k, v := range config.Metadata {
				configCopy.Metadata[k] = v
			}
		}
		return &configCopy
	}
	return nil
}

// RemoveService removes load balancing configuration for a service
func (lb *LoadBalancer) RemoveService(serviceName string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if sel, exists := lb.selectors[serviceName]; exists {
		if closer, ok := sel.(interface{ Close() error }); ok {
			err := closer.Close()
			if err != nil {
				log.Error(err)
				return
			}
		}
		delete(lb.selectors, serviceName)
	}
	delete(lb.configs, serviceName)
}

// Close closes all selectors and cleans up resources
func (lb *LoadBalancer) Close() error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	var lastErr error
	for serviceName, sel := range lb.selectors {
		if closer, ok := sel.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				lastErr = err
			}
		}
		delete(lb.selectors, serviceName)
	}

	lb.configs = make(map[string]*LoadBalancerConfig)
	return lastErr
}

// GetStats returns statistics about the load balancer
func (lb *LoadBalancer) GetStats() map[string]interface{} {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	stats := map[string]interface{}{
		"services":       make(map[string]interface{}),
		"total_services": len(lb.configs),
	}

	services := make(map[string]interface{})
	for serviceName, config := range lb.configs {
		services[serviceName] = map[string]interface{}{
			"strategy": string(config.Strategy),
			"filters":  config.Filters,
			"metadata": config.Metadata,
		}
	}
	stats["services"] = services

	return stats
}

// createSelector creates a selector based on the load balancing strategy
func (lb *LoadBalancer) createSelector(config *LoadBalancerConfig) (selector.Selector, error) {
	var builder selector.Builder

	switch config.Strategy {
	case LoadBalancerRandom:
		builder = random.NewBuilder()
	case LoadBalancerRoundRobin:
		// Use weighted round-robin with equal weights as round-robin
		builder = wrr.NewBuilder()
	case LoadBalancerWeightedRoundRobin:
		builder = wrr.NewBuilder()
	case LoadBalancerP2C:
		builder = p2c.NewBuilder()
	default:
		// Default to random if strategy is unknown
		builder = random.NewBuilder()
	}

	// Apply filters if configured
	if len(config.Filters) > 0 {
		filters := make([]selector.NodeFilter, 0, len(config.Filters))
		for _, filterName := range config.Filters {
			nodeFilter := lb.createNodeFilter(filterName, config.Metadata)
			if nodeFilter != nil {
				filters = append(filters, nodeFilter)
			}
		}
		// Note: Filter application would need to be handled by the specific selector implementation
	}

	return builder.Build(), nil
}

// createNodeFilter creates a node filter based on the filter name
func (lb *LoadBalancer) createNodeFilter(filterName string, metadata map[string]string) selector.NodeFilter {
	switch filterName {
	case "version":
		if version, exists := metadata["version"]; exists {
			return filter.Version(version)
		}
	case "group":
		if group, exists := metadata["group"]; exists {
			return filter.Group(group)
		}
	case "healthy":
		return filter.Healthy()
	case "region":
		if region, exists := metadata["region"]; exists {
			return filter.Region(region)
		}
	}
	return nil
}

// registryNode implements the node.Node interface for registry instances
type registryNode struct {
	instance *registry.ServiceInstance
}

func (n *registryNode) Scheme() string {
	if len(n.instance.Endpoints) > 0 {
		// Extract scheme from endpoint (e.g., "grpc://" from "grpc://localhost:8080")
		for _, endpoint := range n.instance.Endpoints {
			if len(endpoint) > 0 {
				if idx := strings.Index(endpoint, "://"); idx > 0 {
					return endpoint[:idx]
				}
			}
		}
	}
	return "grpc" // Default to grpc
}

func (n *registryNode) Address() string {
	if len(n.instance.Endpoints) > 0 {
		return n.instance.Endpoints[0]
	}
	return ""
}

func (n *registryNode) ServiceName() string {
	return n.instance.Name
}

func (n *registryNode) InitialWeight() *int64 {
	// Try to get weight from metadata
	if weightStr, exists := n.instance.Metadata["weight"]; exists {
		if weight, err := strconv.ParseInt(weightStr, 10, 64); err == nil && weight > 0 {
			return &weight
		}
	}
	// Default weight
	defaultWeight := int64(100)
	return &defaultWeight
}

// Metadata returns the metadata of the node
func (n *registryNode) Metadata() map[string]string {
	return n.instance.Metadata
}

func (n *registryNode) Version() string {
	if version, exists := n.instance.Metadata["version"]; exists {
		return version
	}
	return ""
}

// parseWeight parses weight string to int64
func parseWeight(weightStr string) int64 {
	// Simple implementation - in production you might want more robust parsing
	switch weightStr {
	case "high":
		return 200
	case "medium":
		return 100
	case "low":
		return 50
	default:
		// Try to parse as number
		if len(weightStr) > 0 && weightStr[0] >= '0' && weightStr[0] <= '9' {
			weight := int64(0)
			for _, c := range weightStr {
				if c >= '0' && c <= '9' {
					weight = weight*10 + int64(c-'0')
				} else {
					break
				}
			}
			return weight
		}
		return 100 // Default weight
	}
}

// ConsistentHashSelector implements consistent hash load balancing
type ConsistentHashSelector struct {
	nodes []selector.Node
	hash  func(string) uint32
	mu    sync.RWMutex
}

// NewConsistentHashSelector creates a new consistent hash selector
func NewConsistentHashSelector(hashFunc func(string) uint32) *ConsistentHashSelector {
	if hashFunc == nil {
		hashFunc = defaultHash
	}
	return &ConsistentHashSelector{
		hash: hashFunc,
	}
}

func (s *ConsistentHashSelector) Select(ctx context.Context, opts ...selector.SelectOption) (selector.Node, func(context.Context, selector.DoneInfo), error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.nodes) == 0 {
		return nil, nil, fmt.Errorf("no nodes available")
	}

	// Simple consistent hash implementation
	// In production, you'd want a proper consistent hash ring
	key := fmt.Sprintf("%v", ctx.Value("hash_key"))
	if key == "" {
		key = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	hash := s.hash(key)
	index := int(hash) % len(s.nodes)
	selectedNode := s.nodes[index]

	done := func(ctx context.Context, di selector.DoneInfo) {
		// Record selection metrics or handle completion
	}

	return selectedNode, done, nil
}

func (s *ConsistentHashSelector) Apply(nodes []selector.Node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes = make([]selector.Node, len(nodes))
	copy(s.nodes, nodes)
}

// defaultHash provides a simple hash function
func defaultHash(key string) uint32 {
	hash := uint32(2166136261)
	for _, b := range []byte(key) {
		hash ^= uint32(b)
		hash *= 16777619
	}
	return hash
}

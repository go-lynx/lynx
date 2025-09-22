package sentinel

import (
	"context"
	"fmt"
	"time"

	"github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/go-lynx/lynx/app/log"
)

// Entry performs flow control and circuit breaker check for a resource
func (s *PlugSentinel) Entry(resource string, opts ...api.EntryOption) (interface{}, error) {
	if !s.sentinelInitialized {
		return nil, fmt.Errorf("sentinel plugin not initialized")
	}

	entry, err := api.Entry(resource, opts...)
	if err != nil {
		// Log blocked request
		log.Warnf("Request blocked by Sentinel for resource %s: %v", resource, err)
		
		// Update metrics
		if s.metricsCollector != nil {
			s.metricsCollector.RecordBlocked(resource)
		}
		
		return nil, err
	}

	// Update metrics
	if s.metricsCollector != nil {
		s.metricsCollector.RecordPassed(resource)
	}

	return entry, nil
}

// EntryWithContext performs flow control check with context
func (s *PlugSentinel) EntryWithContext(ctx context.Context, resource string, opts ...api.EntryOption) (interface{}, error) {
	// Check if context is already canceled
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}

	return s.Entry(resource, opts...)
}

// Execute executes a function with Sentinel protection
func (s *PlugSentinel) Execute(resource string, fn func() error, opts ...api.EntryOption) error {
	entry, err := s.Entry(resource, opts...)
	if err != nil {
		return err
	}
	
	// Type assert to get the actual entry
	sentinelEntry, ok := entry.(*base.SentinelEntry)
	if !ok {
		return fmt.Errorf("invalid entry type")
	}
	defer sentinelEntry.Exit()

	startTime := time.Now()
	err = fn()
	duration := time.Since(startTime)

	// Record execution time
	if s.metricsCollector != nil {
		s.metricsCollector.RecordRT(resource, duration)
	}

	// Record error if occurred
	if err != nil {
		if s.metricsCollector != nil {
			s.metricsCollector.RecordError(resource)
		}
		
		// Report error to Sentinel for circuit breaker
		api.TraceError(sentinelEntry, err)
	}

	return err
}

// ExecuteWithContext executes a function with Sentinel protection and context
func (s *PlugSentinel) ExecuteWithContext(ctx context.Context, resource string, fn func(context.Context) error, opts ...api.EntryOption) error {
	entry, err := s.EntryWithContext(ctx, resource, opts...)
	if err != nil {
		return err
	}
	
	// Type assert to get the actual entry
	sentinelEntry, ok := entry.(*base.SentinelEntry)
	if !ok {
		return fmt.Errorf("invalid entry type")
	}
	defer sentinelEntry.Exit()

	startTime := time.Now()
	err = fn(ctx)
	duration := time.Since(startTime)

	// Record execution time
	if s.metricsCollector != nil {
		s.metricsCollector.RecordRT(resource, duration)
	}

	// Record error if occurred
	if err != nil {
		if s.metricsCollector != nil {
			s.metricsCollector.RecordError(resource)
		}
		
		// Report error to Sentinel for circuit breaker
		api.TraceError(sentinelEntry, err)
	}

	return err
}

// CheckFlow checks if a request can pass flow control
func (s *PlugSentinel) CheckFlow(resource string) *FlowControlResult {
	result := &FlowControlResult{
		Resource:  resource,
		Timestamp: time.Now(),
	}

	entry, err := s.Entry(resource)
	if err != nil {
		result.Allowed = false
		result.Reason = err.Error()
		return result
	}

	// Type assert to get the actual entry
	if sentinelEntry, ok := entry.(*base.SentinelEntry); ok {
		sentinelEntry.Exit()
	}
	result.Allowed = true
	return result
}

// GetResourceStats returns statistics for a specific resource
func (s *PlugSentinel) GetResourceStats(resource string) *ResourceStats {
	if s.metricsCollector == nil {
		return nil
	}

	return s.metricsCollector.GetResourceStats(resource)
}

// GetAllResourceStats returns statistics for all resources
func (s *PlugSentinel) GetAllResourceStats() map[string]*ResourceStats {
	if s.metricsCollector == nil {
		return nil
	}

	return s.metricsCollector.GetAllResourceStats()
}

// GetCircuitBreakerState returns the state of circuit breaker for a resource
func (s *PlugSentinel) GetCircuitBreakerState(resource string) *CircuitBreakerState {
	// This would require accessing Sentinel's internal state
	// For now, return a basic implementation
	return &CircuitBreakerState{
		Resource:   resource,
		LastChange: time.Now(),
	}
}

// CreateMiddleware creates a middleware for integration with web frameworks
func (s *PlugSentinel) CreateMiddleware() *SentinelMiddleware {
	return &SentinelMiddleware{
		plugin: s,
	}
}

// SentinelMiddleware methods for different frameworks

// HTTPMiddleware creates HTTP middleware function
func (m *SentinelMiddleware) HTTPMiddleware(resourceExtractor func(interface{}) string) func(interface{}) interface{} {
	return func(next interface{}) interface{} {
		return func(ctx interface{}) {
			resource := resourceExtractor(ctx)
			if resource == "" {
				resource = "http_request"
			}

			err := m.plugin.Execute(resource, func() error {
				// Call next handler
				if nextFunc, ok := next.(func(interface{})); ok {
					nextFunc(ctx)
				}
				return nil
			})

			if err != nil {
				log.Errorf("Request blocked by Sentinel: %v", err)
				// Handle blocked request (return error response)
			}
		}
	}
}

// GRPCUnaryInterceptor creates gRPC unary interceptor
func (m *SentinelMiddleware) GRPCUnaryInterceptor() func(context.Context, interface{}, interface{}, interface{}) (interface{}, error) {
	return func(ctx context.Context, req interface{}, info interface{}, handler interface{}) (interface{}, error) {
		// Extract method name as resource
		resource := "grpc_unary"
		if methodInfo, ok := info.(interface{ FullMethod() string }); ok {
			resource = methodInfo.FullMethod()
		}

		var resp interface{}
		err := m.plugin.ExecuteWithContext(ctx, resource, func(ctx context.Context) error {
			var err error
			if handlerFunc, ok := handler.(func(context.Context, interface{}) (interface{}, error)); ok {
				resp, err = handlerFunc(ctx, req)
			}
			return err
		})

		return resp, err
	}
}

// GRPCStreamInterceptor creates gRPC stream interceptor
func (m *SentinelMiddleware) GRPCStreamInterceptor() func(interface{}, interface{}, interface{}, interface{}) error {
	return func(srv interface{}, stream interface{}, info interface{}, handler interface{}) error {
		// Extract method name as resource
		resource := "grpc_stream"
		if methodInfo, ok := info.(interface{ FullMethod() string }); ok {
			resource = methodInfo.FullMethod()
		}

		return m.plugin.Execute(resource, func() error {
			if handlerFunc, ok := handler.(func(interface{}, interface{}) error); ok {
				return handlerFunc(srv, stream)
			}
			return nil
		})
	}
}

// Utility functions for common use cases

// ProtectFunction wraps a function with Sentinel protection
func ProtectFunction(resource string, fn func() error) error {
	plugin, err := GetSentinel()
	if err != nil {
		return fn() // No protection if plugin not available
	}
	return plugin.Execute(resource, fn)
}

// ProtectFunctionWithContext wraps a function with Sentinel protection and context
func ProtectFunctionWithContext(ctx context.Context, resource string, fn func(context.Context) error) error {
	plugin, err := GetSentinel()
	if err != nil {
		return fn(ctx) // No protection if plugin not available
	}
	return plugin.ExecuteWithContext(ctx, resource, fn)
}

// CheckResourceAvailable checks if a resource is available (not blocked)
func CheckResourceAvailable(resource string) bool {
	plugin, err := GetSentinel()
	if err != nil {
		return true // Allow if plugin not available
	}
	
	result := plugin.CheckFlow(resource)
	return result.Allowed
}
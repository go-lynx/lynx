package grpc

import (
	"context"
	"time"

	"github.com/go-lynx/lynx/app"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
)

// Define monitoring metrics
var (
	// grpcServerUp indicates whether the gRPC server is running
	grpcServerUp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "grpc_server_up",
			Help: "Whether the gRPC server is up",
		},
		[]string{"server_name", "address"},
	)

	// grpcRequestsTotal records the total number of gRPC requests
	grpcRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"method", "status"},
	)

	// grpcRequestDuration records the duration of gRPC requests
	grpcRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "Duration of gRPC requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	// grpcActiveConnections records the number of active connections
	grpcActiveConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "grpc_active_connections",
			Help: "Number of active gRPC connections",
		},
		[]string{"server_name"},
	)

	// grpcServerStartTime records the server start time
	grpcServerStartTime = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "grpc_server_start_time_seconds",
			Help: "Unix timestamp of gRPC server start time",
		},
		[]string{"server_name"},
	)

	// grpcServerErrors records the number of server errors
	grpcServerErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_server_errors_total",
			Help: "Total number of gRPC server errors",
		},
		[]string{"error_type"},
	)
)

// recordHealthCheckMetricsInternal records health check metrics (internal method)
func (g *ServiceGrpc) recordHealthCheckMetricsInternal(healthy bool) {
	if g.conf == nil {
		return
	}

	if healthy {
		grpcServerUp.WithLabelValues(app.GetName(), g.conf.Addr).Set(1)
	} else {
		grpcServerUp.WithLabelValues(app.GetName(), g.conf.Addr).Set(0)
	}
}

// recordRequestMetrics records request metrics
func (g *ServiceGrpc) recordRequestMetrics(method string, duration time.Duration, status string) {
	grpcRequestsTotal.WithLabelValues(method, status).Inc()
	grpcRequestDuration.WithLabelValues(method).Observe(duration.Seconds())
}

// updateConnectionMetrics updates connection metrics
func (g *ServiceGrpc) updateConnectionMetrics(active int) {
	grpcActiveConnections.WithLabelValues(app.GetName()).Set(float64(active))
}

// recordServerStartTime records server start time
func (g *ServiceGrpc) recordServerStartTime() {
	grpcServerStartTime.WithLabelValues(app.GetName()).Set(float64(time.Now().Unix()))
}

// recordServerError records server errors
func (g *ServiceGrpc) recordServerError(errorType string) {
	grpcServerErrors.WithLabelValues(errorType).Inc()
}

// getMetricsHandler returns Prometheus metrics handler
func (g *ServiceGrpc) getMetricsHandler() func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		// Call the next handler
		resp, err := handler(ctx, req)

		// Calculate duration
		duration := time.Since(start)

		// Determine status
		status := "success"
		if err != nil {
			status = "error"
			g.recordServerError("request_error")
		}

		// Record metrics
		g.recordRequestMetrics(info.FullMethod, duration, status)

		return resp, err
	}
}

// Initialize monitoring metrics
func init() {
	// Register custom metrics - use promauto to avoid duplicate registration
	// promauto already handles registration automatically
}

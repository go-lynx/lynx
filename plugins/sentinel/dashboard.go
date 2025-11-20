package sentinel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-lynx/lynx/app/log"
)

// NewDashboardServer creates a new dashboard server
func NewDashboardServer(port int, metricsCollector *MetricsCollector) *DashboardServer {
	return &DashboardServer{
		port:             int32(port),
		metricsCollector: metricsCollector,
		server:           nil,
	}
}

// Start starts the dashboard server
func (ds *DashboardServer) Start(wg *sync.WaitGroup, stopCh chan struct{}) {
	defer wg.Done()

	mux := http.NewServeMux()

	// Register endpoints
	mux.HandleFunc("/", ds.handleIndex)
	mux.HandleFunc("/api/metrics", ds.handleMetrics)
	mux.HandleFunc("/api/resources", ds.handleResources)
	mux.HandleFunc("/api/rules", ds.handleRules)
	mux.HandleFunc("/api/health", ds.handleHealth)

	ds.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", ds.port),
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		log.Infof("Sentinel dashboard server starting on port %d", ds.port)
		if err := ds.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("Dashboard server error: %v", err)
		}
	}()

	// Wait for stop signal
	<-stopCh

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := ds.server.Shutdown(ctx); err != nil {
		log.Errorf("Dashboard server shutdown error: %v", err)
	} else {
		log.Infof("Sentinel dashboard server stopped")
	}
}

// handleIndex serves the dashboard index page
func (ds *DashboardServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Sentinel Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background-color: #f0f0f0; padding: 20px; border-radius: 5px; }
        .section { margin: 20px 0; padding: 15px; border: 1px solid #ddd; border-radius: 5px; }
        .metric { display: inline-block; margin: 10px; padding: 10px; background-color: #e8f4fd; border-radius: 3px; }
        .resource { margin: 10px 0; padding: 10px; background-color: #f9f9f9; border-left: 4px solid #007cba; }
        table { width: 100%; border-collapse: collapse; }
        th, td { padding: 8px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background-color: #f2f2f2; }
        .status-ok { color: green; }
        .status-error { color: red; }
    </style>
    <script>
        function refreshData() {
            fetch('/api/metrics')
                .then(response => response.json())
                .then(data => {
                    document.getElementById('metrics-data').innerHTML = JSON.stringify(data, null, 2);
                });
            
            fetch('/api/resources')
                .then(response => response.json())
                .then(data => {
                    let html = '';
                    for (const [resource, stats] of Object.entries(data)) {
                        html += '<div class="resource">';
                        html += '<h4>' + resource + '</h4>';
                        html += '<p>Total QPS: ' + stats.total_qps.toFixed(2) + '</p>';
                        html += '<p>Pass QPS: ' + stats.pass_qps.toFixed(2) + '</p>';
                        html += '<p>Block QPS: ' + stats.block_qps.toFixed(2) + '</p>';
                        html += '<p>Avg RT: ' + stats.avg_rt.toFixed(2) + 'ms</p>';
                        html += '</div>';
                    }
                    document.getElementById('resources-data').innerHTML = html;
                });
        }
        
        setInterval(refreshData, 5000); // Refresh every 5 seconds
        window.onload = refreshData;
    </script>
</head>
<body>
    <div class="header">
        <h1>Sentinel Dashboard</h1>
        <p>Real-time monitoring for flow control and circuit breaking</p>
    </div>
    
    <div class="section">
        <h2>System Metrics</h2>
        <pre id="metrics-data">Loading...</pre>
    </div>
    
    <div class="section">
        <h2>Resource Statistics</h2>
        <div id="resources-data">Loading...</div>
    </div>
    
    <div class="section">
        <h2>Quick Links</h2>
        <ul>
            <li><a href="/api/metrics">Metrics API</a></li>
            <li><a href="/api/resources">Resources API</a></li>
            <li><a href="/api/rules">Rules API</a></li>
            <li><a href="/api/health">Health Check</a></li>
        </ul>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// handleMetrics serves metrics data
func (ds *DashboardServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if ds.metricsCollector == nil {
		http.Error(w, "Metrics collector not available", http.StatusServiceUnavailable)
		return
	}

	summary := ds.metricsCollector.GetMetricsSummary()

	if err := json.NewEncoder(w).Encode(summary); err != nil {
		log.Errorf("Failed to encode metrics: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleResources serves resource statistics
func (ds *DashboardServer) handleResources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if ds.metricsCollector == nil {
		http.Error(w, "Metrics collector not available", http.StatusServiceUnavailable)
		return
	}

	stats := ds.metricsCollector.GetAllResourceStats()

	// Convert to JSON-friendly format
	result := make(map[string]interface{})
	for resource, stat := range stats {
		result[resource] = map[string]interface{}{
			"resource":  stat.Resource,
			"total_qps": stat.TotalQPS,
			"pass_qps":  stat.PassQPS,
			"block_qps": stat.BlockQPS,
			"avg_rt":    stat.AvgRT,
			"min_rt":    stat.MinRT,
			"max_rt":    stat.MaxRT,
			"timestamp": stat.Timestamp.Format(time.RFC3339),
		}
	}

	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Errorf("Failed to encode resources: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleRules serves current rules information
func (ds *DashboardServer) handleRules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get plugin instance to access rules
	plugin, err := GetSentinel()
	if err != nil {
		http.Error(w, fmt.Sprintf("Sentinel plugin not available: %v", err), http.StatusServiceUnavailable)
		return
	}

	// Get all rules
	flowRules := plugin.GetFlowRules()
	circuitBreakerRules := plugin.GetCircuitBreakerRules()
	systemRules := plugin.GetSystemRules()

	// Convert to JSON-friendly format
	rules := map[string]interface{}{
		"flow_rules":            flowRules,
		"circuit_breaker_rules": circuitBreakerRules,
		"system_rules":          systemRules,
		"count": map[string]int{
			"flow_rules":            len(flowRules),
			"circuit_breaker_rules": len(circuitBreakerRules),
			"system_rules":          len(systemRules),
		},
	}

	if err := json.NewEncoder(w).Encode(rules); err != nil {
		log.Errorf("Failed to encode rules: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleHealth serves health check
func (ds *DashboardServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
		"uptime":    time.Since(time.Now()).String(), // This would be calculated properly in real implementation
		"version":   "1.0.0",
		"services": map[string]string{
			"dashboard":         "running",
			"metrics_collector": "running",
			"sentinel_core":     "running",
		},
	}

	if err := json.NewEncoder(w).Encode(health); err != nil {
		log.Errorf("Failed to encode health: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetPort returns the dashboard server port
func (ds *DashboardServer) GetPort() int {
	return int(ds.port)
}

// IsRunning returns whether the server is running
func (ds *DashboardServer) IsRunning() bool {
	return ds.server != nil
}

// GetURL returns the dashboard URL
func (ds *DashboardServer) GetURL() string {
	return fmt.Sprintf("http://localhost:%d", ds.port)
}

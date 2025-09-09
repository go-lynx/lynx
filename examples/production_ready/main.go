package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/observability/metrics"
	"github.com/go-lynx/lynx/boot"
	"github.com/go-lynx/lynx/plugins"
)

// ProductionReadyExample demonstrates production-ready features
func main() {
	// Create production metrics
	productionMetrics := metrics.NewProductionMetrics()
	productionMetrics.Start()
	defer productionMetrics.Stop()

	// Create error recovery manager
	errorRecoveryManager := app.NewErrorRecoveryManager(productionMetrics)
	defer errorRecoveryManager.Stop()

	// Create application with enhanced features
	application := boot.NewApplication(wireApp, createPlugins()...)

	// Note: Custom cleanup can be added by extending the Application struct
	// or by implementing cleanup logic in the wireApp function

	// Run the application with production features
	if err := application.Run(); err != nil {
		log.Fatal(err)
	}
}

// wireApp creates the Kratos application
func wireApp(logger log.Logger) (*kratos.App, error) {
	// Create a simple Kratos app for demonstration
	app := kratos.New(
		kratos.ID("production-ready-example"),
		kratos.Name("Production Ready Example"),
		kratos.Version("1.0.0"),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
	)

	return app, nil
}

// createPlugins creates plugins for the example
func createPlugins() []plugins.Plugin {
	return []plugins.Plugin{
		&ExamplePlugin{
			name: "example-plugin",
			id:   "example-1",
		},
	}
}

// ExamplePlugin demonstrates plugin with error handling
type ExamplePlugin struct {
	*plugins.BasePlugin
	name string
	id   string
}

// ID returns the plugin ID
func (p *ExamplePlugin) ID() string {
	return p.id
}

// Name returns the plugin name
func (p *ExamplePlugin) Name() string {
	return p.name
}

// Version returns the plugin version
func (p *ExamplePlugin) Version() string {
	return "1.0.0"
}

// Description returns the plugin description
func (p *ExamplePlugin) Description() string {
	return "Example plugin for production-ready demonstration"
}

// Dependencies returns plugin dependencies
func (p *ExamplePlugin) Dependencies() []*plugins.Dependency {
	return nil
}

// Initialize initializes the plugin
func (p *ExamplePlugin) Initialize(plugin plugins.Plugin, rt plugins.Runtime) error {
	// Simulate initialization with potential error
	if time.Now().Unix()%10 == 0 { // 10% chance of error
		return fmt.Errorf("simulated initialization error")
	}
	return nil
}

// Start starts the plugin
func (p *ExamplePlugin) Start(plugin plugins.Plugin) error {
	// Simulate startup with potential error
	if time.Now().Unix()%15 == 0 { // ~6.7% chance of error
		return fmt.Errorf("simulated startup error")
	}
	return nil
}

// Stop stops the plugin
func (p *ExamplePlugin) Stop(plugin plugins.Plugin) error {
	// Simulate graceful shutdown
	time.Sleep(100 * time.Millisecond)
	return nil
}

// CheckHealth checks plugin health
func (p *ExamplePlugin) CheckHealth() error {
	// Simulate health check with potential failure
	if time.Now().Unix()%20 == 0 { // 5% chance of health check failure
		return fmt.Errorf("simulated health check failure")
	}
	return nil
}

// CleanupTasks performs cleanup tasks
func (p *ExamplePlugin) CleanupTasks() error {
	// Simulate cleanup
	time.Sleep(50 * time.Millisecond)
	return nil
}

// GetDependencies returns plugin dependencies
func (p *ExamplePlugin) GetDependencies() []plugins.Dependency {
	return nil
}

// InitializeResources initializes plugin resources
func (p *ExamplePlugin) InitializeResources(rt plugins.Runtime) error {
	return nil
}

// StartupTasks performs startup tasks
func (p *ExamplePlugin) StartupTasks() error {
	return nil
}

// Status returns plugin status
func (p *ExamplePlugin) Status(plugin plugins.Plugin) plugins.PluginStatus {
	return plugins.StatusActive
}

// Weight returns plugin weight
func (p *ExamplePlugin) Weight() int {
	return 1
}

// demonstrateProductionFeatures demonstrates production features
func demonstrateProductionFeatures() {
	// Create metrics
	metrics := metrics.NewProductionMetrics()
	metrics.Start()
	defer metrics.Stop()

	// Record various metrics
	metrics.RecordPluginHealth("example-plugin", "example-1", true)
	metrics.RecordPluginLatency("example-plugin", "example-1", "operation", 100*time.Millisecond)
	metrics.RecordEventPublished("test-event", "plugin-bus")
	metrics.RecordRequest("GET", "/api/test", "200", "http", 50*time.Millisecond, 1024, 2048)

	// Create error recovery manager
	errorRecoveryManager := app.NewErrorRecoveryManager(metrics)
	defer errorRecoveryManager.Stop()

	// Record some errors
	errorRecoveryManager.RecordError("database", app.ErrorCategoryDatabase, "connection timeout", "mysql-plugin", app.ErrorSeverityMedium, map[string]interface{}{
		"timeout": "5s",
		"retries": 3,
	})

	errorRecoveryManager.RecordError("network", app.ErrorCategoryNetwork, "connection refused", "http-client", app.ErrorSeverityLow, map[string]interface{}{
		"host": "api.example.com",
		"port": 8080,
	})

	// Wait for recovery attempts
	time.Sleep(2 * time.Second)

	// Get error statistics
	stats := errorRecoveryManager.GetErrorStats()
	fmt.Printf("Error Statistics: %+v\n", stats)

	// Get health report
	healthReport := errorRecoveryManager.GetHealthReport()
	fmt.Printf("Health Report: %+v\n", healthReport)

	// Get metrics snapshot
	metricsSnapshot := metrics.GetMetrics()
	fmt.Printf("Metrics Snapshot: %+v\n", metricsSnapshot)
}

// Example of using the production features in a real application
func ExampleUsage() {
	// This function demonstrates how to use the production features
	// in a real application scenario

	// 1. Initialize production metrics
	productionMetrics := metrics.NewProductionMetrics()
	productionMetrics.Start()
	defer productionMetrics.Stop()

	// 2. Initialize error recovery manager
	errorRecoveryManager := app.NewErrorRecoveryManager(productionMetrics)
	defer errorRecoveryManager.Stop()

	// 3. Create custom recovery strategies
	customStrategy := &CustomRecoveryStrategy{
		name:    "custom-db-recovery",
		timeout: 10 * time.Second,
	}
	errorRecoveryManager.RegisterRecoveryStrategy("database", customStrategy)

	// 4. Use in your application logic
	go func() {
		for {
			// Simulate some work
			time.Sleep(5 * time.Second)

			// Record metrics
			productionMetrics.RecordPluginHealth("my-service", "service-1", true)
			productionMetrics.RecordRequest("POST", "/api/data", "200", "http", 100*time.Millisecond, 512, 1024)

			// Simulate occasional errors
			if time.Now().Unix()%30 == 0 {
				errorRecoveryManager.RecordError("service", app.ErrorCategorySystem, "temporary failure", "my-service", app.ErrorSeverityLow, nil)
			}
		}
	}()

	// 5. Monitor health
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if !errorRecoveryManager.IsHealthy() {
					fmt.Println("Warning: Error recovery manager is unhealthy")

					// Get detailed health report
					report := errorRecoveryManager.GetHealthReport()
					fmt.Printf("Health Report: %+v\n", report)
				}
			}
		}
	}()

	// Keep the application running
	select {}
}

// CustomRecoveryStrategy demonstrates a custom recovery strategy
type CustomRecoveryStrategy struct {
	name    string
	timeout time.Duration
}

// Name returns the strategy name
func (s *CustomRecoveryStrategy) Name() string {
	return s.name
}

// CanRecover checks if this strategy can recover from the error
func (s *CustomRecoveryStrategy) CanRecover(errorType string, severity app.ErrorSeverity) bool {
	return errorType == "database" && severity <= app.ErrorSeverityMedium
}

// Recover attempts to recover from the error
func (s *CustomRecoveryStrategy) Recover(ctx context.Context, record app.ErrorRecord) (bool, error) {
	// Simulate database recovery
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(2 * time.Second):
		// Simulate successful recovery
		return true, nil
	}
}

// GetTimeout returns the recovery timeout
func (s *CustomRecoveryStrategy) GetTimeout() time.Duration {
	return s.timeout
}

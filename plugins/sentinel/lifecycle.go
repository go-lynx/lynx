package sentinel

import (
	"fmt"
	"time"

	"github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/config"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/core/system"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins"
)

// InitializeResources implements custom initialization logic for Sentinel plugin
// Scans and loads Sentinel configuration from runtime config
func (s *PlugSentinel) InitializeResources(rt plugins.Runtime) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Initialize configuration structure
	s.conf = &SentinelConfig{}

	// Scan and load Sentinel configuration from runtime config
	err := rt.GetConfig().Value(confPrefix).Scan(s.conf)
	if err != nil {
		return fmt.Errorf("failed to load sentinel configuration: %w", err)
	}

	// Validate and set default configuration
	if err := s.validateAndSetDefaults(); err != nil {
		return fmt.Errorf("sentinel configuration validation failed: %w", err)
	}

	// Initialize Sentinel core
	if err := s.initializeSentinelCore(); err != nil {
		return fmt.Errorf("failed to initialize sentinel core: %w", err)
	}

	// Initialize metrics collector if enabled
	if s.conf.Metrics.Enabled {
		interval, err := time.ParseDuration(s.conf.Metrics.Interval)
		if err != nil {
			interval = 30 * time.Second // default interval
		}
		s.metricsCollector = NewMetricsCollector(interval)
	}

	// Initialize dashboard server if enabled
	if s.conf.Dashboard.Enabled {
		s.dashboardServer = NewDashboardServer(int(s.conf.Dashboard.Port), s.metricsCollector)
	}

	s.isInitialized = true
	log.Infof("Sentinel plugin initialized successfully")
	return nil
}

// StartupTasks implements plugin startup tasks
func (s *PlugSentinel) StartupTasks() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isInitialized {
		return fmt.Errorf("sentinel plugin not initialized")
	}

	// Load flow control rules
	if err := s.loadFlowRules(); err != nil {
		return fmt.Errorf("failed to load flow rules: %w", err)
	}

	// Load circuit breaker rules
	if err := s.loadCircuitBreakerRules(); err != nil {
		return fmt.Errorf("failed to load circuit breaker rules: %w", err)
	}

	// Load system protection rules
	if err := s.loadSystemRules(); err != nil {
		return fmt.Errorf("failed to load system rules: %w", err)
	}

	// Start metrics collector
	if s.metricsCollector != nil {
		s.wg.Add(1)
		go s.metricsCollector.Start(&s.wg, s.stopCh)
	}

	// Start dashboard server
	if s.dashboardServer != nil {
		s.wg.Add(1)
		go s.dashboardServer.Start(&s.wg, s.stopCh)
	}

	log.Infof("Sentinel plugin started successfully")
	return nil
}

// CleanupTasks implements plugin cleanup tasks
func (s *PlugSentinel) CleanupTasks() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Infof("Stopping Sentinel plugin...")

	// Signal all background tasks to stop
	close(s.stopCh)

	// Wait for all background tasks to complete
	s.wg.Wait()

	// Clear all rules
	flow.ClearRules()
	circuitbreaker.ClearRules()
	system.ClearRules()

	log.Infof("Sentinel plugin stopped successfully")
	return nil
}

// CheckHealth implements health check for Sentinel plugin
func (s *PlugSentinel) CheckHealth() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.isInitialized {
		return fmt.Errorf("sentinel plugin not initialized")
	}

	// Check if Sentinel is initialized
	if !s.sentinelInitialized {
		return fmt.Errorf("sentinel core not initialized")
	}

	// Perform a simple flow control check to verify functionality
	entry, err := api.Entry("health_check")
	if err != nil {
		return fmt.Errorf("sentinel health check failed: %w", err)
	}
	entry.Exit()

	return nil
}

// Configure allows updating Sentinel configuration at runtime
func (s *PlugSentinel) Configure(c any) error {
	if c == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	newConf, ok := c.(*SentinelConfig)
	if !ok {
		return fmt.Errorf("invalid configuration type for sentinel plugin")
	}

	// Validate new configuration
	if err := s.validateConfiguration(newConf); err != nil {
		return fmt.Errorf("invalid sentinel configuration: %w", err)
	}

	// Update configuration
	s.conf = newConf

	// Reload rules with new configuration
	if s.isInitialized {
		if err := s.reloadRules(); err != nil {
			return fmt.Errorf("failed to reload rules: %w", err)
		}
	}

	log.Infof("Sentinel plugin configuration updated successfully")
	return nil
}

// initializeSentinelCore initializes the Sentinel core components
func (s *PlugSentinel) initializeSentinelCore() error {
	// Configure Sentinel
	sentinelConfig := config.NewDefaultConfig()
	sentinelConfig.Sentinel.App.Name = s.conf.AppName
	sentinelConfig.Sentinel.Log.Dir = s.conf.LogDir
	// Note: Sentinel config LogConfig doesn't have Level field, we'll set it via logging API

	// Initialize Sentinel
	err := api.InitWithConfig(sentinelConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize sentinel: %w", err)
	}

	// Set logging level
	if err := s.setLoggingLevel(); err != nil {
		log.Warnf("Failed to set sentinel logging level: %v", err)
	}

	s.sentinelInitialized = true
	return nil
}

// setLoggingLevel sets the Sentinel logging level
func (s *PlugSentinel) setLoggingLevel() error {
	// Note: Sentinel's logging.NewConsoleLogger() doesn't accept level parameter
	// The level is controlled by the global logging configuration
	logging.ResetGlobalLogger(logging.NewConsoleLogger())
	return nil
}

// validateAndSetDefaults validates configuration and sets default values
func (s *PlugSentinel) validateAndSetDefaults() error {
	if s.conf.AppName == "" {
		s.conf.AppName = "lynx-app"
	}

	if s.conf.LogDir == "" {
		s.conf.LogDir = "./logs/sentinel"
	}

	if s.conf.LogLevel == "" {
		s.conf.LogLevel = "info"
	}

	// Set default metrics configuration
	if s.conf.Metrics.Interval == "" {
		s.conf.Metrics.Interval = "30s"
	}

	// Set default dashboard configuration
	if s.conf.Dashboard.Port == 0 {
		s.conf.Dashboard.Port = 8719
	}

	return s.validateConfiguration(s.conf)
}

// validateConfiguration validates the Sentinel configuration
func (s *PlugSentinel) validateConfiguration(conf *SentinelConfig) error {
	if conf.AppName == "" {
		return fmt.Errorf("app_name cannot be empty")
	}

	if conf.Dashboard.Port != 0 && (conf.Dashboard.Port < 1024 || conf.Dashboard.Port > 65535) {
		return fmt.Errorf("dashboard_port must be between 1024 and 65535")
	}

	// Validate flow rules
	for i, rule := range conf.FlowRules {
		if rule.Resource == "" {
			return fmt.Errorf("flow rule %d: resource name cannot be empty", i)
		}
		if rule.Threshold < 0 {
			return fmt.Errorf("flow rule %d: threshold must be non-negative", i)
		}
	}

	// Validate circuit breaker rules
	for i, rule := range conf.CBRules {
		if rule.Resource == "" {
			return fmt.Errorf("circuit breaker rule %d: resource name cannot be empty", i)
		}
		if rule.Threshold < 0 {
			return fmt.Errorf("circuit breaker rule %d: threshold must be non-negative", i)
		}
		if rule.MinRequestAmount < 0 {
			return fmt.Errorf("circuit breaker rule %d: min_request_amount must be non-negative", i)
		}
	}

	return nil
}

// reloadRules reloads all Sentinel rules
func (s *PlugSentinel) reloadRules() error {
	if err := s.loadFlowRules(); err != nil {
		return err
	}
	if err := s.loadCircuitBreakerRules(); err != nil {
		return err
	}
	if err := s.loadSystemRules(); err != nil {
		return err
	}
	return nil
}
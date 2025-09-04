package polaris

import (
	"fmt"
	"strings"

	"github.com/go-lynx/lynx/app/log"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// CheckHealth performs a health check.
func (p *PlugPolaris) CheckHealth() error {
	if err := p.checkInitialized(); err != nil {
		return err
	}

	// Check Polaris instance
	if p.polaris == nil {
		return NewInitError("Polaris instance is nil")
	}

	// Check SDK connection
	if p.sdk == nil {
		return NewInitError("Polaris SDK context is nil")
	}

	// Perform actual health check of the Polaris control plane
	return p.checkPolarisControlPlaneHealth()
}

// checkPolarisControlPlaneHealth checks the health of the Polaris control plane.
func (p *PlugPolaris) checkPolarisControlPlaneHealth() error {
	// Record the start of the health check
	if p.metrics != nil {
		p.metrics.RecordHealthCheck("polaris", "start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordHealthCheck("polaris", "success")
			}
		}()
	}

	log.Infof("Checking Polaris control plane health")

	// Execute health checks using circuit breaker and retry mechanisms
	var healthErr error
	err := p.circuitBreaker.Do(func() error {
		return p.retryManager.DoWithRetry(func() error {
			// 1) Check SDK connection status
			if err := p.checkSDKConnection(); err != nil {
				healthErr = err
				return err
			}

			// 2) Check service discovery functionality
			if err := p.checkServiceDiscoveryHealth(); err != nil {
				healthErr = err
				return err
			}

			// 3) Check configuration management functionality
			if err := p.checkConfigManagementHealth(); err != nil {
				healthErr = err
				return err
			}

			// 4) Check rate limiting functionality
			if err := p.checkRateLimitHealth(); err != nil {
				healthErr = err
				return err
			}

			return nil
		})
	})

	if err != nil {
		log.Errorf("Polaris control plane health check failed: %v", healthErr)
		if p.metrics != nil {
			p.metrics.RecordHealthCheck("polaris", "error")
		}
		return WrapServiceError(healthErr, ErrCodeServiceUnavailable, "Polaris control plane health check failed")
	}

	log.Infof("Polaris control plane health check passed")
	return nil
}

// checkSDKConnection verifies SDK connection status.
func (p *PlugPolaris) checkSDKConnection() error {
	// Try to create a Consumer API client to validate connectivity
	consumerAPI := api.NewConsumerAPIByContext(p.sdk)
	if consumerAPI == nil {
		return fmt.Errorf("failed to create consumer API client")
	}

	// Try to create a simple service discovery request to validate connectivity
	req := &api.GetInstancesRequest{
		GetInstancesRequest: model.GetInstancesRequest{
			Service:   "health-check-service", // use a test service name
			Namespace: p.conf.Namespace,
		},
	}

	// Try calling the API; even if the service does not exist, the connection should succeed
	_, err := consumerAPI.GetInstances(req)
	if err != nil {
		// If the error indicates the service is not found, connectivity is fine
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no instances") {
			log.Debugf("SDK connection test passed (service not found is expected)")
			return nil
		}
		return fmt.Errorf("SDK connection test failed: %v", err)
	}

	return nil
}

// checkServiceDiscoveryHealth checks service discovery functionality.
func (p *PlugPolaris) checkServiceDiscoveryHealth() error {
	// Check status of components related to service discovery
	if p.activeWatchers == nil {
		return fmt.Errorf("service watchers not initialized")
	}

	// Check whether there are active watchers
	watcherCount := len(p.activeWatchers)
	log.Debugf("Service discovery health: %d active watchers", watcherCount)

	return nil
}

// checkConfigManagementHealth checks configuration management functionality.
func (p *PlugPolaris) checkConfigManagementHealth() error {
	// Check status of components related to configuration management
	if p.configWatchers == nil {
		return fmt.Errorf("config watchers not initialized")
	}

	// Check whether there are active config watchers
	configWatcherCount := len(p.configWatchers)
	log.Debugf("Config management health: %d active config watchers", configWatcherCount)

	return nil
}

// checkRateLimitHealth checks rate limiting functionality.
func (p *PlugPolaris) checkRateLimitHealth() error {
	// Check status of components related to rate limiting
	if p.circuitBreaker == nil {
		return fmt.Errorf("circuit breaker not initialized")
	}

	if p.retryManager == nil {
		return fmt.Errorf("retry manager not initialized")
	}

	// Check circuit breaker state
	breakerState := p.circuitBreaker.GetState()
	log.Debugf("Rate limit health: circuit breaker state = %s", breakerState)

	return nil
}

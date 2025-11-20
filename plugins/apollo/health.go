package apollo

import (
	"fmt"
	"strings"

	"github.com/go-lynx/lynx/app/log"
	"github.com/go-lynx/lynx/plugins/apollo/conf"
)

// CheckHealth performs a health check.
func (p *PlugApollo) CheckHealth() error {
	if err := p.checkInitialized(); err != nil {
		return err
	}

	// Check Apollo client
	if p.client == nil {
		return NewInitError("Apollo client is nil")
	}

	// Perform actual health check of the Apollo configuration center
	return p.checkApolloHealth()
}

// checkApolloHealth checks the health of the Apollo configuration center.
func (p *PlugApollo) checkApolloHealth() error {
	// Record the start of the health check
	if p.metrics != nil {
		p.metrics.RecordHealthCheck("start")
		defer func() {
			if p.metrics != nil {
				p.metrics.RecordHealthCheck("success")
			}
		}()
	}

	log.Infof("Checking Apollo configuration center health")

	// Execute health checks using circuit breaker and retry mechanisms
	var healthErr error
	err := p.circuitBreaker.Do(func() error {
		return p.retryManager.DoWithRetry(func() error {
			// 1) Check client connection status
			if err := p.checkClientConnection(); err != nil {
				healthErr = err
				return err
			}

			// 2) Check configuration management functionality
			if err := p.checkConfigManagementHealth(); err != nil {
				healthErr = err
				return err
			}

			return nil
		})
	})

	if err != nil {
		log.Errorf("Apollo configuration center health check failed: %v", healthErr)
		if p.metrics != nil {
			p.metrics.RecordHealthCheck("error")
		}
		return WrapClientError(healthErr, ErrCodeHealthCheckFailed, "Apollo configuration center health check failed")
	}

	log.Infof("Apollo configuration center health check passed")
	return nil
}

// checkClientConnection verifies client connection status.
func (p *PlugApollo) checkClientConnection() error {
	// Try to get a configuration value to validate connectivity
	testNamespace := p.conf.Namespace
	if testNamespace == "" {
		testNamespace = conf.DefaultNamespace
	}

	// Try to get a test configuration key
	// If the key doesn't exist, that's fine - we just need to verify we can connect
	_, err := p.GetConfigValue(testNamespace, "health.check.key")
	if err != nil {
		// If the error indicates the key is not found, connectivity is fine
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not exist") {
			log.Debugf("Client connection test passed (key not found is expected)")
			return nil
		}
		// Other errors indicate connection problems
		return fmt.Errorf("client connection test failed: %v", err)
	}

	// Key exists and was retrieved successfully
	return nil
}

// checkConfigManagementHealth checks configuration management functionality.
func (p *PlugApollo) checkConfigManagementHealth() error {
	// Check status of components related to configuration management
	if p.configWatchers == nil {
		return fmt.Errorf("config watchers not initialized")
	}

	// Check whether there are active config watchers
	configWatcherCount := len(p.configWatchers)
	log.Debugf("Config management health: %d active config watchers", configWatcherCount)

	return nil
}


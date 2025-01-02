package boot

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
)

// LoadLocalBootstrapConfig loads the bootstrap configuration from a local file or directory.
// It reads the configuration from the path specified in flagConf and initializes
// the application's configuration state.
//
// Returns:
//   - error: Any error that occurred during configuration loading
func (b *Boot) LoadLocalBootstrapConfig() error {
	if b == nil {
		return fmt.Errorf("boot instance is nil")
	}

	if flagConf == "" {
		return fmt.Errorf("configuration path is empty")
	}

	// Log the configuration loading attempt
	log.Infof("Loading local bootstrap configuration from: %s", flagConf)

	// Create configuration source from local file
	source := file.NewSource(flagConf)
	if source == nil {
		return fmt.Errorf("failed to create configuration source from: %s", flagConf)
	}

	// Create new configuration instance
	cfg := config.New(
		config.WithSource(source),
	)
	if cfg == nil {
		return fmt.Errorf("failed to create configuration instance")
	}

	// Load the configuration
	if err := cfg.Load(); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Store the configuration before setting up cleanup
	// This ensures we don't lose the reference if cleanup setup fails
	b.conf = cfg

	// Setup configuration cleanup
	if err := b.setupConfigCleanup(cfg); err != nil {
		// Log the cleanup setup failure but continue
		// The configuration is still valid and usable
		log.Warnf("Failed to setup configuration cleanup: %v", err)
	}

	return nil
}

// setupConfigCleanup ensures proper cleanup of configuration resources
// when they are no longer needed.
func (b *Boot) setupConfigCleanup(cfg config.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration instance is nil")
	}

	// Setup deferred cleanup
	b.cleanup = func() {
		if err := cfg.Close(); err != nil {
			log.Errorf("Failed to close configuration: %v", err)
		}
	}

	return nil
}

// Package app provides core application functionality for the Lynx framework
package app

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-lynx/lynx/conf"
)

// Embedded banner file for application startup
//
//go:embed banner.txt
var bannerFS embed.FS

// InitLogger initializes the application's logging system.
// It sets up the main logger with standard output and configures various logging fields
// such as timestamps, caller information, service details, and tracing IDs.
func (a *LynxApp) InitLogger() error {
	if a == nil {
		return fmt.Errorf("lynx app instance is nil")
	}

	// Log the initialization of the logging component
	log.Info("Initializing Lynx logging component")

	// Initialize the main logger with standard output and default fields
	logger := log.With(
		log.NewStdLogger(os.Stdout),
		"timestamp", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", GetHost(),
		"service.name", GetName(),
		"service.version", GetVersion(),
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)

	if logger == nil {
		return fmt.Errorf("failed to create logger")
	}

	// Create a helper for more convenient logging
	helper := log.NewHelper(logger)
	if helper == nil {
		return fmt.Errorf("failed to create logger helper")
	}

	// Store logger instances
	a.logger = logger
	a.logHelper = *helper

	// Log successful initialization
	helper.Info("Lynx logging component initialized successfully")

	// Initialize and display the application banner
	if err := a.initBanner(); err != nil {
		helper.Warnf("Failed to initialize banner: %v", err)
		// Continue execution as banner display is not critical
	}

	return nil
}

// initBanner handles the initialization and display of the application banner.
// It reads the banner from the embedded filesystem and displays it based on configuration.
func (a *LynxApp) initBanner() error {
	// Read banner content from embedded filesystem
	bannerData, err := fs.ReadFile(bannerFS, "banner.txt")
	if err != nil {
		return fmt.Errorf("failed to read banner: %v", err)
	}

	// Read application configuration
	var bootConfig conf.Bootstrap
	if err := a.GetGlobalConfig().Scan(&bootConfig); err != nil {
		return fmt.Errorf("failed to read configuration: %v", err)
	}

	// Check if banner display is enabled
	if bootConfig.GetLynx() == nil ||
		bootConfig.GetLynx().GetApplication() == nil {
		return fmt.Errorf("invalid configuration structure")
	}

	// Display banner if not disabled in configuration
	if !bootConfig.GetLynx().GetApplication().GetCloseBanner() {
		a.logHelper.Infof("\n%s", bannerData)
	}

	return nil
}

// GetLogHelper returns the application's log helper instance.
// This helper provides convenient methods for logging at different levels.
func (a *LynxApp) GetLogHelper() *log.Helper {
	return &a.logHelper
}

// GetLogger returns the application's main logger instance.
// This logger provides the core logging functionality.
func (a *LynxApp) GetLogger() log.Logger {
	return a.logger
}

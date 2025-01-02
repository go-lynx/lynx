package plugins

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
)

// PluginStatus represents the current operational status of a plugin in the system.
// It tracks the plugin's lifecycle state from initialization through termination.
type PluginStatus int

const (
	// StatusInactive indicates that the plugin is loaded but not yet initialized
	// This is the initial state of a plugin when it is first loaded into the system
	StatusInactive PluginStatus = iota

	// StatusInitializing indicates that the plugin is currently performing initialization
	// During this state, the plugin is setting up resources, establishing connections,
	// and preparing for normal operation
	StatusInitializing

	// StatusActive indicates that the plugin is fully operational and running normally
	// In this state, the plugin is processing requests and performing its intended functions
	StatusActive

	// StatusSuspended indicates that the plugin is temporarily paused
	// The plugin retains its resources but is not processing new requests
	// Can be resumed to StatusActive without full reinitialization
	StatusSuspended

	// StatusStopping indicates that the plugin is in the process of shutting down
	// During this state, the plugin is cleaning up resources and finishing pending operations
	StatusStopping

	// StatusTerminated indicates that the plugin has been gracefully shut down
	// All resources have been released and connections closed
	// Requires full reinitialization to become active again
	StatusTerminated

	// StatusFailed indicates that the plugin has encountered a fatal error
	// The plugin is non-operational and may require manual intervention
	// Should transition to StatusTerminated or attempt recovery
	StatusFailed

	// StatusUpgrading indicates that the plugin is currently being upgraded
	// During this state, the plugin may be partially operational
	// Should transition to StatusActive or StatusFailed
	StatusUpgrading

	// StatusRollback indicates that the plugin is rolling back from a failed upgrade
	// Attempting to restore the previous working state
	// Should transition to StatusActive or StatusFailed
	StatusRollback
)

// UpgradeCapability defines the various ways a plugin can be upgraded during runtime
type UpgradeCapability int

const (
	// UpgradeNone indicates the plugin does not support any runtime upgrades
	// Must be stopped and restarted to apply any changes
	UpgradeNone UpgradeCapability = iota

	// UpgradeConfig indicates the plugin can update its configuration without restart
	// Supports runtime configuration changes but not code updates
	UpgradeConfig

	// UpgradeVersion indicates the plugin can perform version upgrades without restart
	// Supports both configuration and code updates during runtime
	UpgradeVersion

	// UpgradeReplace indicates the plugin supports complete replacement during runtime
	// Can be entirely replaced with a new instance while maintaining service
	UpgradeReplace
)

// Plugin is the minimal interface that all plugins must implement
// It combines basic metadata and lifecycle management capabilities
type Plugin interface {
	Metadata
	Lifecycle
}

// Metadata defines methods for retrieving plugin metadata
// This interface provides essential information about the plugin
type Metadata interface {
	// ID returns the unique identifier of the plugin
	// This ID must be unique across all plugins in the system
	ID() string

	// Name returns the display name of the plugin
	// This is a human-readable name used for display purposes
	Name() string

	// Description returns a detailed description of the plugin
	// Should provide information about the plugin's purpose and functionality
	Description() string

	// Version returns the semantic version of the plugin
	// Should follow semver format (MAJOR.MINOR.PATCH)
	Version() string
}

// Lifecycle defines the basic lifecycle methods for a plugin
// Handles initialization, operation, and termination of the plugin
type Lifecycle interface {
	// Initialize prepares the plugin for use
	// Sets up resources, connections, and internal state
	// Returns error if initialization fails
	Initialize(rt Runtime) error

	// Start begins the plugin's main functionality
	// Should only be called after successful initialization
	// Returns error if startup fails
	Start() error

	// Stop gracefully terminates the plugin's functionality
	// Releases resources and closes connections
	// Returns error if shutdown fails
	Stop() error

	// Status returns the current status of the plugin
	// Provides real-time state information
	Status() PluginStatus
}

// ResourceManager provides access to shared plugin resources
// Manages resource allocation, sharing, and lifecycle
type ResourceManager interface {
	// GetResource retrieves a shared plugin resource by name
	// Returns the resource and any error encountered
	GetResource(name string) (any, error)

	// RegisterResource registers a resource to be shared with other plugins
	// Returns error if registration fails
	RegisterResource(name string, resource any) error
}

// ConfigProvider provides access to plugin configuration
// Manages plugin configuration loading and access
type ConfigProvider interface {
	// GetConfig returns the plugin configuration manager
	// Provides access to configuration values and updates
	GetConfig() config.Config
}

// LogProvider provides access to logging functionality
// Manages plugin logging capabilities
type LogProvider interface {
	// GetLogger returns the plugin logger instance
	// Provides structured logging capabilities
	GetLogger() log.Logger
}

// Runtime combines all runtime capabilities for plugins
// Provides a complete runtime environment for plugin operation
type Runtime interface {
	ResourceManager
	ConfigProvider
	LogProvider
	EventEmitter
}

// Dependency describes a dependency relationship between plugins
// Defines requirements and relationships between plugins
type Dependency struct {
	ID       string            // Unique identifier of the required plugin
	Required bool              // Whether this dependency is mandatory
	Checker  DependencyChecker // Validates dependency requirements
	Metadata map[string]any    // Additional dependency information
}

// DependencyChecker defines the interface for dependency validation
// Validates plugin dependencies and their conditions
type DependencyChecker interface {
	// Check validates if the dependency condition is met
	// Returns true if the dependency is satisfied
	Check(plugin Plugin) bool

	// Description returns a human-readable description of the condition
	// Explains what the dependency checker validates
	Description() string
}

// HealthReport represents the detailed health status of a plugin
// Provides comprehensive health information for monitoring
type HealthReport struct {
	Status    string         // Current health status (healthy, degraded, unhealthy)
	Details   map[string]any // Detailed health metrics and information
	Timestamp int64          // Time of the health check (Unix timestamp)
	Message   string         // Optional descriptive message
}

// Upgradable defines methods for plugin upgrade operations
// Manages plugin version upgrades and updates
type Upgradable interface {
	// GetCapabilities returns the supported upgrade capabilities
	// Lists the ways this plugin can be upgraded
	GetCapabilities() []UpgradeCapability

	// PrepareUpgrade prepares for version upgrade
	// Validates and prepares for the upgrade process
	PrepareUpgrade(targetVersion string) error

	// ExecuteUpgrade performs the actual version upgrade
	// Applies the upgrade and verifies success
	ExecuteUpgrade(targetVersion string) error

	// RollbackUpgrade reverts to the previous version
	// Restores the plugin to its previous state
	RollbackUpgrade(previousVersion string) error
}

// Suspendable defines methods for temporary plugin suspension
// Manages temporary plugin deactivation and reactivation
type Suspendable interface {
	// Suspend temporarily suspends plugin operations
	// Pauses plugin activity while maintaining state
	Suspend() error

	// Resume restores plugin operations from a suspended state
	// Resumes normal operation without reinitialization
	Resume() error
}

// Configurable defines methods for plugin configuration management
// Manages plugin configuration updates and validation
type Configurable interface {
	// Configure applies and validates the given configuration
	// Updates plugin configuration during runtime
	Configure(conf any) error
}

// HealthCheck defines methods for plugin health monitoring
// Provides health status and monitoring capabilities
type HealthCheck interface {
	// GetHealth returns the current health status of the plugin
	// Provides detailed health information
	GetHealth() HealthReport
}

// DependencyAware defines methods for plugin dependency management
// Manages plugin dependencies and their relationships
type DependencyAware interface {
	// GetDependencies returns the list of plugin dependencies
	// Lists all required and optional dependencies
	GetDependencies() []Dependency
}

// EventHandler defines methods for plugin event handling
// Processes plugin-related events and notifications
type EventHandler interface {
	// HandleEvent processes plugin lifecycle events
	// Handles various plugin system events
	HandleEvent(event PluginEvent)
}

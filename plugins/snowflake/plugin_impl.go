package snowflake

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-lynx/lynx/plugins"
	pb "github.com/go-lynx/lynx/plugins/snowflake/conf"
)

// Ensure PlugSnowflake implements the Plugin interface
var _ plugins.Plugin = (*PlugSnowflake)(nil)

// Plugin interface implementation

// ID returns the unique identifier of the plugin
func (p *PlugSnowflake) ID() string {
	return PluginName
}

// Name returns the human-readable name of the plugin
func (p *PlugSnowflake) Name() string {
	return PluginName
}

// Description returns a description of what the plugin does
func (p *PlugSnowflake) Description() string {
	return PluginDescription
}

// Version returns the version of the plugin
func (p *PlugSnowflake) Version() string {
	return PluginVersion
}

// Weight returns the loading priority weight of the plugin
func (p *PlugSnowflake) Weight() int {
	return 100 // Medium priority
}

// Initialize initializes the plugin with the given runtime
func (p *PlugSnowflake) Initialize(plugin plugins.Plugin, rt plugins.Runtime) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Status(plugin) != plugins.StatusInactive {
		return fmt.Errorf("plugin is already initialized")
	}

	p.SetStatus(plugins.StatusInitializing)
	p.runtime = rt
	p.logger = rt.GetLogger()

	// Get configuration from runtime
	config := rt.GetConfig()
	if config == nil {
		return fmt.Errorf("configuration not available")
	}

	// Load plugin configuration
	var snowflakeConfig pb.Snowflake
	if err := config.Scan(&snowflakeConfig); err != nil {
		p.SetStatus(plugins.StatusFailed)
		return fmt.Errorf("failed to load snowflake configuration: %w", err)
	}

	p.conf = &snowflakeConfig

	// Initialize the generator
	generator, err := NewSnowflakeGeneratorWithConfig(p.conf)
	if err != nil {
		p.SetStatus(plugins.StatusFailed)
		return fmt.Errorf("failed to create snowflake generator: %w", err)
	}

	p.generator = generator
	p.SetStatus(plugins.StatusActive)

	log.Info("Snowflake plugin initialized successfully")
	return nil
}

// Start starts the plugin
func (p *PlugSnowflake) Start(plugin plugins.Plugin) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Status(plugin) != plugins.StatusActive {
		return fmt.Errorf("plugin is not in active status, current status: %v", p.Status(plugin))
	}

	// Register the plugin instance as a shared resource
	if p.runtime != nil {
		err := p.runtime.RegisterSharedResource(PluginName, p)
		if err != nil {
			log.Errorf("Failed to register snowflake plugin as shared resource: %v", err)
			return err
		}
	}

	log.Info("Snowflake plugin started successfully")
	return nil
}

// Stop stops the plugin
func (p *PlugSnowflake) Stop(plugin plugins.Plugin) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Status(plugin) == plugins.StatusTerminated {
		return nil // Already stopped
	}

	p.SetStatus(plugins.StatusStopping)

	// Cleanup resources if needed
	if p.generator != nil {
		// Snowflake generator doesn't need explicit cleanup
		// but we can add any necessary cleanup logic here
	}

	p.SetStatus(plugins.StatusTerminated)
	log.Info("Snowflake plugin stopped successfully")
	return nil
}

// Status returns the current status of the plugin
func (p *PlugSnowflake) Status(plugin plugins.Plugin) plugins.PluginStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.BasePlugin.Status(plugin)
}

// InitializeResources initializes plugin resources
func (p *PlugSnowflake) InitializeResources(rt plugins.Runtime) error {
	// Resources are initialized in the Initialize method
	return nil
}

// StartupTasks performs startup tasks
func (p *PlugSnowflake) StartupTasks() error {
	// No specific startup tasks needed for snowflake generator
	return nil
}

// CleanupTasks performs cleanup tasks
func (p *PlugSnowflake) CleanupTasks() error {
	// No specific cleanup tasks needed for snowflake generator
	return nil
}

// CheckHealth checks the health of the plugin
func (p *PlugSnowflake) CheckHealth() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.BasePlugin.Status(p) != plugins.StatusActive {
		return fmt.Errorf("plugin is not active, current status: %v", p.BasePlugin.Status(p))
	}

	if p.generator == nil {
		return fmt.Errorf("snowflake generator is not initialized")
	}

	// Try to generate an ID to verify the generator is working
	_, err := p.generator.GenerateID()
	if err != nil {
		return fmt.Errorf("snowflake generator health check failed: %w", err)
	}

	return nil
}

// GetDependencies returns the list of plugin dependencies
func (p *PlugSnowflake) GetDependencies() []plugins.Dependency {
	// Snowflake plugin has no dependencies by default
	// If Redis integration is enabled, it would depend on Redis plugin
	var deps []plugins.Dependency

	if p.conf != nil && p.conf.AutoRegisterWorkerId && p.conf.RedisPluginName != "" {
		deps = append(deps, plugins.Dependency{
			Name:     "redis",
			Type:     "nosql",
		})
	}

	return deps
}

// Plugin-specific methods

// GetGenerator returns the snowflake generator instance
func (p *PlugSnowflake) GetGenerator() *SnowflakeGenerator {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.generator
}

// GetConfig returns the plugin configuration
func (p *PlugSnowflake) GetConfig() *pb.Snowflake {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.conf
}

// GenerateID generates a new snowflake ID
func (p *PlugSnowflake) GenerateID() (int64, error) {
	p.mu.RLock()
	generator := p.generator
	p.mu.RUnlock()

	if generator == nil {
		return 0, fmt.Errorf("snowflake generator is not initialized")
	}

	return generator.GenerateID()
}

// GenerateIDWithMetadata generates a new snowflake ID with metadata
func (p *PlugSnowflake) GenerateIDWithMetadata() (int64, *SnowflakeID, error) {
	p.mu.RLock()
	generator := p.generator
	p.mu.RUnlock()

	if generator == nil {
		return 0, nil, fmt.Errorf("snowflake generator is not initialized")
	}

	return generator.GenerateIDWithMetadata()
}

// ParseID parses a snowflake ID and returns its metadata
func (p *PlugSnowflake) ParseID(id int64) (*SnowflakeID, error) {
	p.mu.RLock()
	generator := p.generator
	p.mu.RUnlock()

	if generator == nil {
		return nil, fmt.Errorf("snowflake generator is not initialized")
	}

	return generator.ParseID(id)
}

// Context-aware lifecycle methods (optional)

// InitializeContext initializes the plugin with context
func (p *PlugSnowflake) InitializeContext(ctx context.Context, plugin plugins.Plugin, rt plugins.Runtime) error {
	// Use the regular Initialize method for now
	return p.Initialize(plugin, rt)
}

// StartContext starts the plugin with context
func (p *PlugSnowflake) StartContext(ctx context.Context, plugin plugins.Plugin) error {
	// Use the regular Start method for now
	return p.Start(plugin)
}

// StopContext stops the plugin with context
func (p *PlugSnowflake) StopContext(ctx context.Context, plugin plugins.Plugin) error {
	// Use the regular Stop method for now
	return p.Stop(plugin)
}

// IsContextAware returns whether the plugin is context-aware
func (p *PlugSnowflake) IsContextAware() bool {
	return true
}

// HealthCheck performs a health check on the plugin
func (p *PlugSnowflake) HealthCheck() (bool, error) {
	if p.generator == nil {
		return false, fmt.Errorf("snowflake generator not initialized")
	}
	
	// Try to generate an ID to verify the generator is working
	_, err := p.generator.GenerateID()
	if err != nil {
		return false, fmt.Errorf("health check failed: %w", err)
	}
	
	return true, nil
}

// Helper function to create a new plugin instance
func NewSnowflakePlugin() *PlugSnowflake {
	return &PlugSnowflake{
		BasePlugin: plugins.NewBasePlugin(
			plugins.GeneratePluginID("", PluginName, "1.0.0"),
			PluginName,
			"Snowflake ID generator plugin",
			"1.0.0",
			"snowflake",
			100,
		),
	}
}
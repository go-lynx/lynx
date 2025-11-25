package snowflake

import (
	"fmt"
	"sync"

	"github.com/go-lynx/lynx/app"
	"github.com/go-lynx/lynx/app/factory"
)

var (
	// fallbackPlugin is used when PluginManager is not available
	fallbackPlugin     *PlugSnowflake
	fallbackPluginOnce sync.Once
	fallbackPluginErr  error
)

// GetSnowflakePlugin retrieves the snowflake plugin from the application
func GetSnowflakePlugin() (*PlugSnowflake, error) {
	// Try to get from application plugin manager first
	if app.Lynx() != nil && app.Lynx().GetPluginManager() != nil {
		plugin := app.Lynx().GetPluginManager().GetPlugin(PluginName)
		if plugin != nil {
			if snowflakePlugin, ok := plugin.(*PlugSnowflake); ok {
				return snowflakePlugin, nil
			}
			return nil, fmt.Errorf("plugin '%s' is not a snowflake plugin", PluginName)
		}
	}

	// Fallback to factory with singleton pattern
	fallbackPluginOnce.Do(func() {
		plugin, err := factory.GlobalTypedFactory().CreatePlugin(PluginName)
		if err != nil {
			fallbackPluginErr = fmt.Errorf("snowflake plugin not found: %w", err)
			return
		}

		snowflakePlugin, ok := plugin.(*PlugSnowflake)
		if !ok {
			fallbackPluginErr = fmt.Errorf("plugin is not a snowflake plugin instance")
			return
		}

		fallbackPlugin = snowflakePlugin
	})

	if fallbackPluginErr != nil {
		return nil, fallbackPluginErr
	}

	return fallbackPlugin, nil
}

// GenerateID generates a new snowflake ID using the global plugin instance
func GenerateID() (int64, error) {
	plugin, err := GetSnowflakePlugin()
	if err != nil {
		return 0, err
	}

	return plugin.GenerateID()
}

// GenerateIDWithMetadata generates a new snowflake ID with metadata using the global plugin instance
func GenerateIDWithMetadata() (int64, *SID, error) {
	plugin, err := GetSnowflakePlugin()
	if err != nil {
		return 0, nil, err
	}

	return plugin.GenerateIDWithMetadata()
}

// ParseID parses a snowflake ID and returns its metadata using the global plugin instance
func ParseID(id int64) (*SID, error) {
	plugin, err := GetSnowflakePlugin()
	if err != nil {
		return nil, err
	}

	return plugin.ParseID(id)
}

// GetGenerator returns the snowflake generator instance from the global plugin
func GetGenerator() (*Generator, error) {
	plugin, err := GetSnowflakePlugin()
	if err != nil {
		return nil, err
	}

	generator := plugin.GetGenerator()
	if generator == nil {
		return nil, fmt.Errorf("snowflake generator is not initialized")
	}

	return generator, nil
}

// CheckHealth checks the health of the snowflake plugin
func CheckHealth() error {
	plugin, err := GetSnowflakePlugin()
	if err != nil {
		return err
	}

	return plugin.CheckHealth()
}

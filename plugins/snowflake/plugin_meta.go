package snowflake

// Plugin metadata
const (
	PluginName        = "snowflake"
	PluginVersion     = "1.0.0"
	PluginDescription = "Snowflake ID generator plugin with clock drift protection and Redis-based worker ID management"
	ConfPrefix        = "lynx.snowflake"
)

// NewSnowflakeGenerator creates a new snowflake generator plugin instance
// This is kept for backward compatibility
func NewSnowflakeGenerator() *PlugSnowflake {
	return NewSnowflakePlugin()
}

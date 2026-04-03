package plugin

// DefaultConfigForPlugin returns a bootstrap-ready config block for a plugin.
// The returned value is intended to be placed under the top-level "lynx.<plugin>" key.
func DefaultConfigForPlugin(plugin *PluginMetadata) map[string]any {
	if plugin == nil {
		return nil
	}
	manager := &PluginManager{}
	return manager.getPluginSpecificConfig(plugin)
}

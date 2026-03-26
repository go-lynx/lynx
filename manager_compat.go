package lynx

// ConfigReloadPlan is retained only as a compatibility report for older callers.
// New code should prefer RestartRequirementReport, which reflects the
// restart-based core model directly.
type ConfigReloadPlan struct {
	HotReloadable   []ConfigReloadEntry
	RestartRequired []ConfigReloadEntry
	Unsupported     []ConfigReloadEntry
	Invalid         []ConfigReloadEntry
}

// GetConfigReloadPlan returns the compatibility config-reload view for older callers.
//
// Deprecated: prefer GetRestartRequirementReport, which matches Lynx core's
// restart-based configuration model.
func (m *DefaultPluginManager[T]) GetConfigReloadPlan() ConfigReloadPlan {
	report := m.GetRestartRequirementReport()
	return ConfigReloadPlan{
		HotReloadable:   make([]ConfigReloadEntry, 0),
		RestartRequired: append([]ConfigReloadEntry(nil), report.RestartRequired...),
		Unsupported:     make([]ConfigReloadEntry, 0),
		Invalid:         append([]ConfigReloadEntry(nil), report.Invalid...),
	}
}

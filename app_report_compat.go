package lynx

// ConfigReloadPlan returns the current plugin manager's compatibility view for older callers.
//
// Deprecated: prefer RestartRequirementReport(), which matches Lynx core's
// restart-based configuration model.
func (a *LynxApp) ConfigReloadPlan() ConfigReloadPlan {
	if a == nil || a.pluginManager == nil {
		return ConfigReloadPlan{}
	}
	report := a.pluginManager.GetRestartRequirementReport()
	return ConfigReloadPlan{
		HotReloadable:   make([]ConfigReloadEntry, 0),
		RestartRequired: append([]ConfigReloadEntry(nil), report.RestartRequired...),
		Unsupported:     make([]ConfigReloadEntry, 0),
		Invalid:         append([]ConfigReloadEntry(nil), report.Invalid...),
	}
}

// RuntimeReport is retained as a compatibility report shape for older callers.
type RuntimeReport struct {
	AppName    string
	AppVersion string
	Host       string
	// RestartRequirementReport is the core-facing view for config-change handling.
	RestartRequirementReport RestartRequirementReport
	// ConfigReloadPlan is retained only for compatibility with older callers.
	ConfigReloadPlan ConfigReloadPlan
	Plugins          []PluginRuntimeReport
}

// RuntimeReport returns the compatibility runtime summary for older callers.
//
// Deprecated: prefer CoreRuntimeReport(), which omits compatibility config-reload vocabulary.
func (a *LynxApp) RuntimeReport() RuntimeReport {
	core := a.CoreRuntimeReport()
	return RuntimeReport{
		AppName:                  core.AppName,
		AppVersion:               core.AppVersion,
		Host:                     core.Host,
		RestartRequirementReport: core.RestartRequirementReport,
		ConfigReloadPlan:         a.ConfigReloadPlan(),
		Plugins:                  core.Plugins,
	}
}

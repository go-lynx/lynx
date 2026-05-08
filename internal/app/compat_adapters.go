//go:build !v2

// compat_adapters.go — deprecated method definitions retained for v1 callers.
// Methods must be in the same package as their receiver types; they cannot be
// moved to internal/app/compat/ without a circular import.
// Will be removed entirely in v2.0.
package app

// ConfigReloadPlan is retained only as a compatibility report for older callers.
// Deprecated: prefer RestartRequirementReport.
type ConfigReloadPlan struct {
	HotReloadable   []ConfigReloadEntry
	RestartRequired []ConfigReloadEntry
	Unsupported     []ConfigReloadEntry
	Invalid         []ConfigReloadEntry
}

// RuntimeReport is retained as a compatibility report shape for older callers.
// Deprecated: prefer CoreRuntimeReport.
type RuntimeReport struct {
	AppName                  string
	AppVersion               string
	Host                     string
	RestartRequirementReport RestartRequirementReport
	ConfigReloadPlan         ConfigReloadPlan
	Plugins                  []PluginRuntimeReport
}

// Shutdown is an alias for Close for backward compatibility.
// Deprecated: use Close() directly.
func (a *LynxApp) Shutdown() error {
	return a.Close()
}

// ConfigReloadPlan returns the current plugin manager's compatibility view.
// Deprecated: prefer GetRestartRequirementReport().
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

// RuntimeReport returns the compatibility runtime summary.
// Deprecated: prefer CoreRuntimeReport().
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

// GetConfigReloadPlan returns the compatibility config-reload view.
// Deprecated: prefer GetRestartRequirementReport().
func (m *DefaultPluginManager[T]) GetConfigReloadPlan() ConfigReloadPlan {
	report := m.GetRestartRequirementReport()
	return ConfigReloadPlan{
		HotReloadable:   make([]ConfigReloadEntry, 0),
		RestartRequired: append([]ConfigReloadEntry(nil), report.RestartRequired...),
		Unsupported:     make([]ConfigReloadEntry, 0),
		Invalid:         append([]ConfigReloadEntry(nil), report.Invalid...),
	}
}

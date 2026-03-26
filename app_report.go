package lynx

import "github.com/go-lynx/lynx/plugins"

// RestartRequirementReport returns the current plugin manager's restart-based
// compatibility report for configuration changes.
func (a *LynxApp) RestartRequirementReport() RestartRequirementReport {
	if a == nil || a.pluginManager == nil {
		return RestartRequirementReport{}
	}
	return a.pluginManager.GetRestartRequirementReport()
}

type PluginRuntimeReport struct {
	ID           string
	Name         string
	Version      string
	Status       plugins.PluginStatus
	Capabilities plugins.PluginCapabilities
}

// CoreRuntimeReport is the preferred core-facing runtime summary.
type CoreRuntimeReport struct {
	AppName    string
	AppVersion string
	Host       string
	// RestartRequirementReport is the core-facing view for config-change handling.
	RestartRequirementReport RestartRequirementReport
	Plugins                  []PluginRuntimeReport
}

// CoreRuntimeReport returns the preferred core-facing runtime summary.
func (a *LynxApp) CoreRuntimeReport() CoreRuntimeReport {
	report := CoreRuntimeReport{}
	if a == nil {
		return report
	}

	report.AppName = a.Name()
	report.AppVersion = a.Version()
	report.Host = a.Host()
	report.RestartRequirementReport = a.RestartRequirementReport()

	if a.pluginManager == nil {
		return report
	}

	list := Plugins(a.pluginManager)
	report.Plugins = make([]PluginRuntimeReport, 0, len(list))
	for _, p := range list {
		if p == nil {
			continue
		}
		report.Plugins = append(report.Plugins, PluginRuntimeReport{
			ID:           p.ID(),
			Name:         p.Name(),
			Version:      p.Version(),
			Status:       p.Status(p),
			Capabilities: plugins.DescribePluginCapabilities(p),
		})
	}

	return report
}

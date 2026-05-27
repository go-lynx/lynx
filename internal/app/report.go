package app

import (
	"github.com/go-lynx/lynx/events"
	"github.com/go-lynx/lynx/pkg/security"
	"github.com/go-lynx/lynx/plugins"
)

// RestartRequirementReport returns the current plugin manager's restart-based
// compatibility report for configuration changes.
func (a *LynxApp) RestartRequirementReport() RestartRequirementReport {
	if a == nil || a.pluginManager == nil {
		return RestartRequirementReport{}
	}
	return a.pluginManager.GetRestartRequirementReport()
}

type PluginRuntimeReport struct {
	ID                          string
	Name                        string
	Version                     string
	Status                      plugins.PluginStatus
	Capabilities                plugins.PluginCapabilities
	LifecycleTimeoutCancellable bool
	LifecycleRisk               string
}

// CoreRuntimeReport is the preferred core-facing runtime summary.
type CoreRuntimeReport struct {
	AppName    string
	AppVersion string
	Host       string
	// ProductionMode reports whether the process is running with a production environment marker.
	ProductionMode bool
	// ProductionReady is false when lynx detects settings that are unsafe for a formal production release.
	ProductionReady bool
	// ProductionReadinessFailures lists concrete blockers that should be fixed before release.
	ProductionReadinessFailures []string
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
	report.ProductionMode = security.IsProduction()
	report.ProductionReady = true
	report.RestartRequirementReport = a.RestartRequirementReport()
	report.ProductionReadinessFailures = append(report.ProductionReadinessFailures, a.productionReadinessFailures()...)
	if len(report.ProductionReadinessFailures) > 0 {
		report.ProductionReady = false
	}

	if a.pluginManager == nil {
		return report
	}

	list := Plugins(a.pluginManager)
	report.Plugins = make([]PluginRuntimeReport, 0, len(list))
	for _, p := range list {
		if p == nil {
			continue
		}
		caps := plugins.DescribePluginCapabilities(p)
		timeoutCancellable := plugins.HasTrueContextLifecycle(p)
		lifecycleRisk := ""
		if !timeoutCancellable {
			lifecycleRisk = "lifecycle timeout returns control to lynx, but plugin work may continue in the background"
		}
		report.Plugins = append(report.Plugins, PluginRuntimeReport{
			ID:                          p.ID(),
			Name:                        p.Name(),
			Version:                     p.Version(),
			Status:                      p.Status(p),
			Capabilities:                caps,
			LifecycleTimeoutCancellable: timeoutCancellable,
			LifecycleRisk:               lifecycleRisk,
		})
	}

	return report
}

func (a *LynxApp) productionReadinessFailures() []string {
	failures := make([]string, 0)
	conf := a.GetGlobalConfig()
	requireContextAware := contextAwareLifecycleRequired(conf)

	if a.pluginManager != nil {
		for _, p := range Plugins(a.pluginManager) {
			if p == nil {
				continue
			}
			if requireContextAware && !plugins.HasTrueContextLifecycle(p) {
				failures = append(failures, "plugin "+p.Name()+" ("+p.ID()+") does not provide cancellable context-aware lifecycle")
			}
		}
	}

	if reportProductionGlobalFallbackRisk() {
		failures = append(failures, "deprecated global event bus fallback is active; use app-owned event facilities or explicit manager APIs")
	}

	return failures
}

func reportProductionGlobalFallbackRisk() bool {
	if !security.IsProduction() {
		return false
	}
	return events.IsGlobalEventBusFallbackActive()
}

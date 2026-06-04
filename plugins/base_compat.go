package plugins

func (p *TypedBasePlugin[T]) applyLegacyTransitionalHealth(report *HealthReport, message string) {
	if report == nil {
		return
	}
	report.Status = "transitional"
	report.Message = message
	p.EmitEvent(PluginEvent{
		Type:     EventHealthStatusUnknown,
		Priority: PriorityNormal,
		Source:   "GetHealth",
		Category: "health",
	})
}

// PrepareUpgrade is intentionally unsupported by the base plugin; live upgrade
// belongs to external rollout tooling, not Lynx core.
//
// Deprecated: retained only for legacy compatibility.
func (p *TypedBasePlugin[T]) PrepareUpgrade(targetVersion string) error {
	return NewPluginError(p.id, "PrepareUpgrade", "Live plugin upgrade is not supported by lynx core", ErrPluginUpgradeNotSupported)
}

// ExecuteUpgrade is intentionally unsupported by the base plugin.
//
// Deprecated: retained only for legacy compatibility.
func (p *TypedBasePlugin[T]) ExecuteUpgrade(targetVersion string) error {
	return NewPluginError(p.id, "ExecuteUpgrade", "Live plugin upgrade is not supported by lynx core", ErrPluginUpgradeNotSupported)
}

// RollbackUpgrade is intentionally unsupported by the base plugin.
//
// Deprecated: retained only for legacy compatibility.
func (p *TypedBasePlugin[T]) RollbackUpgrade(previousVersion string) error {
	return NewPluginError(p.id, "RollbackUpgrade", "Live plugin rollback is not supported by lynx core", ErrPluginUpgradeNotSupported)
}

// PerformUpgrade is intentionally unsupported by the base plugin.
//
// Deprecated: retained only for legacy compatibility.
func (p *TypedBasePlugin[T]) PerformUpgrade(targetVersion string) error {
	return ErrPluginUpgradeNotSupported
}

// PerformRollback is intentionally unsupported by the base plugin.
//
// Deprecated: retained only for legacy compatibility.
func (p *TypedBasePlugin[T]) PerformRollback(previousVersion string) error {
	return ErrPluginUpgradeNotSupported
}

// GetCapabilities returns the plugin's upgrade capabilities.
//
// Deprecated: retained only for legacy compatibility metadata.
func (p *TypedBasePlugin[T]) GetCapabilities() []UpgradeCapability {
	return p.capabilities
}

// SetCapabilities replaces the plugin's declared upgrade capabilities.
//
// Deprecated: retained only for legacy compatibility metadata.
func (p *TypedBasePlugin[T]) SetCapabilities(caps ...UpgradeCapability) {
	if len(caps) == 0 {
		p.capabilities = []UpgradeCapability{UpgradeNone}
		return
	}
	p.capabilities = append([]UpgradeCapability(nil), caps...)
}

// AddCapability appends an upgrade capability when it is not already declared.
//
// Deprecated: retained only for legacy compatibility metadata.
func (p *TypedBasePlugin[T]) AddCapability(cap UpgradeCapability) {
	for _, existing := range p.capabilities {
		if existing == cap {
			return
		}
	}
	if len(p.capabilities) == 1 && p.capabilities[0] == UpgradeNone {
		p.capabilities = []UpgradeCapability{cap}
		return
	}
	p.capabilities = append(p.capabilities, cap)
}

// SupportsCapability reports whether the plugin declares the given upgrade capability.
//
// Deprecated: retained only for legacy compatibility metadata.
func (p *TypedBasePlugin[T]) SupportsCapability(cap UpgradeCapability) bool {
	for _, c := range p.capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// Deprecated: retained only for legacy compatibility; base plugin does not
// support runtime configuration reload.
func (p *TypedBasePlugin[T]) Configure(conf any) error {
	return NewPluginError(p.id, "Configure", "Runtime configuration reload is not supported by the base plugin", ErrRuntimeConfigNotSupported)
}

// Deprecated: retained only for legacy compatibility; base plugin does not
// support runtime configuration rollback.
func (p *TypedBasePlugin[T]) RollbackConfig(previous any) error {
	return NewPluginError(p.id, "RollbackConfig", "Runtime configuration rollback is not supported by the base plugin", ErrRuntimeConfigNotSupported)
}

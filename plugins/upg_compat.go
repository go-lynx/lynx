package plugins

// Upgradable defines legacy plugin-local upgrade hooks.
// This is intentionally optional compatibility metadata; Lynx core focuses on
// orchestration and does not provide a framework guarantee for live plugin
// replacement or in-process rollout.
type Upgradable interface {
	// GetCapabilities returns the legacy upgrade capabilities advertised by the plugin.
	GetCapabilities() []UpgradeCapability

	// PrepareUpgrade prepares plugin-owned upgrade logic.
	PrepareUpgrade(targetVersion string) error

	// ExecuteUpgrade performs plugin-owned upgrade logic.
	ExecuteUpgrade(targetVersion string) error

	// RollbackUpgrade performs plugin-owned rollback logic.
	RollbackUpgrade(previousVersion string) error
}

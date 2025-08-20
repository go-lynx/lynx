package plugins

// Upgradable defines methods for plugin upgrade operations
// Manages plugin version upgrades and updates
type Upgradable interface {
	// GetCapabilities returns the supported upgrade capabilities
	// Lists the ways this plugin can be upgraded
	GetCapabilities() []UpgradeCapability

	// PrepareUpgrade prepares for version upgrade
	// Validates and prepares for the upgrade process
	PrepareUpgrade(targetVersion string) error

	// ExecuteUpgrade performs the actual version upgrade
	// Applies the upgrade and verifies success
	ExecuteUpgrade(targetVersion string) error

	// RollbackUpgrade reverts to the previous version
	// Restores the plugin to its previous state
	RollbackUpgrade(previousVersion string) error
}

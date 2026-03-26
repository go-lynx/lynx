package plugins

// Deprecated: retained only as legacy compatibility vocabulary; Lynx core does
// not treat live upgrade as a standard lifecycle path.
// StatusUpgrading is retained only as legacy compatibility vocabulary.
// Lynx core does not treat live upgrade as a standard lifecycle path.
const StatusUpgrading PluginStatus = 7

// Deprecated: retained only as legacy compatibility vocabulary; Lynx core does
// not orchestrate in-process rollback as a standard lifecycle path.
// StatusRollback is retained only as legacy compatibility vocabulary.
// Lynx core does not orchestrate in-process rollback as a standard lifecycle path.
const StatusRollback PluginStatus = 8

// Deprecated: retained only for legacy compatibility metadata.
// UpgradeCapability describes optional legacy plugin self-management behaviors.
// Lynx core does not treat live upgrade or replacement as a framework-level
// guarantee; these flags are advisory compatibility metadata only and concrete
// plugins must opt in explicitly where needed.
type UpgradeCapability int

const (
	// UpgradeNone indicates the plugin does not advertise legacy live-upgrade hooks.
	// Restart remains the default core path for change application.
	UpgradeNone UpgradeCapability = iota

	// UpgradeConfig indicates the plugin advertises legacy self-managed config mutation.
	// This does not mean Lynx core orchestrates in-process rollout.
	UpgradeConfig

	// UpgradeVersion indicates the plugin advertises legacy self-managed version upgrade hooks.
	// Lynx core still expects restart/external rollout as the standard path.
	UpgradeVersion

	// UpgradeReplace indicates the plugin advertises legacy replacement semantics.
	// Lynx core does not provide a framework guarantee for live replacement.
	UpgradeReplace
)

// Deprecated: retained only for legacy compatibility hooks.
// Configurable defines optional compatibility hooks for plugins that manage
// their own runtime configuration concerns. Lynx core does not orchestrate
// in-process config rollout; configuration changes are applied by restart.
type Configurable interface {
	// Configure applies plugin-specific configuration in plugin-owned code paths.
	Configure(conf any) error
}

// Deprecated: retained only for legacy compatibility hooks.
// ConfigValidator defines optional compatibility validation for plugin-owned
// configuration handling. It does not make live core reload a supported path.
type ConfigValidator interface {
	ValidateConfig(conf any) error
}

// Deprecated: retained only for legacy compatibility hooks.
// ConfigRollbacker defines optional compatibility rollback support for
// plugin-owned configuration handling. Lynx core itself remains restart-based.
type ConfigRollbacker interface {
	RollbackConfig(previous any) error
}

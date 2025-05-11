package plugins

// Upgradable defines methods for plugin upgrade operations
// Manages plugin version upgrades and updates
// Upgradable 定义了插件升级操作的方法。
// 管理插件版本升级和更新。
type Upgradable interface {
	// GetCapabilities returns the supported upgrade capabilities
	// Lists the ways this plugin can be upgraded
	// GetCapabilities 返回支持的升级能力。
	// 列出此插件可进行升级的方式。
	GetCapabilities() []UpgradeCapability

	// PrepareUpgrade prepares for version upgrade
	// Validates and prepares for the upgrade process
	// PrepareUpgrade 为版本升级做准备。
	// 验证并为升级过程做准备。
	PrepareUpgrade(targetVersion string) error

	// ExecuteUpgrade performs the actual version upgrade
	// Applies the upgrade and verifies success
	// ExecuteUpgrade 执行实际的版本升级。
	// 应用升级并验证是否成功。
	ExecuteUpgrade(targetVersion string) error

	// RollbackUpgrade reverts to the previous version
	// Restores the plugin to its previous state
	// RollbackUpgrade 回滚到上一个版本。
	// 将插件恢复到之前的状态。
	RollbackUpgrade(previousVersion string) error
}

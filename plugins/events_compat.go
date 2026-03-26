package plugins

// Upgrade and rollback event types are retained only for compatibility with
// older plugins and external consumers. Lynx core does not use them as part of
// the default lifecycle model, and restart/external rollout remains the
// preferred way to apply code or deployment changes.
const (
	// EventUpgradeAvailable indicates new version availability.
	EventUpgradeAvailable = "upgrade.available"

	// EventUpgradeInitiated indicates upgrade process start.
	EventUpgradeInitiated = "upgrade.initiated"

	// EventUpgradeValidating indicates upgrade validation.
	EventUpgradeValidating = "upgrade.validating"

	// EventUpgradeInProgress indicates that the upgrade process is ongoing.
	EventUpgradeInProgress = "upgrade.in_progress"

	// EventUpgradeCompleted indicates successful upgrade.
	EventUpgradeCompleted = "upgrade.completed"

	// EventUpgradeFailed indicates failed upgrade attempt.
	EventUpgradeFailed = "upgrade.failed"

	// EventRollbackInitiated indicates version rollback start.
	EventRollbackInitiated = "rollback.initiated"

	// EventRollbackInProgress indicates that the rollback process is ongoing.
	EventRollbackInProgress = "rollback.in_progress"

	// EventRollbackCompleted indicates successful rollback.
	EventRollbackCompleted = "rollback.completed"

	// EventRollbackFailed indicates failed rollback attempt.
	EventRollbackFailed = "rollback.failed"
)

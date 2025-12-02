package events

import (
	"time"

	"github.com/go-lynx/lynx/plugins"
)

// ConvertPluginEvent converts a PluginEvent to LynxEvent
func ConvertPluginEvent(pluginEvent plugins.PluginEvent) LynxEvent {
	// Generate EventID if not present in metadata
	eventID := ""
	if pluginEvent.Metadata != nil {
		if id, ok := pluginEvent.Metadata["event_id"].(string); ok && id != "" {
			eventID = id
		}
	}
	// If no EventID in metadata, generate one
	if eventID == "" {
		var t time.Time
		// Check if timestamp is unset (0 or negative) before calling time.Unix
		// time.Unix(0, 0) returns epoch time, not zero time, so IsZero() won't work
		if pluginEvent.Timestamp <= 0 {
			t = time.Now()
		} else {
			t = time.Unix(pluginEvent.Timestamp, 0)
		}
		eventID = generateEventID(pluginEvent.PluginID, ConvertEventType(pluginEvent.Type), t)
	}

	return LynxEvent{
		EventID:   eventID,
		EventType: ConvertEventType(pluginEvent.Type),
		Priority:  ConvertPriority(pluginEvent.Priority),
		Source:    pluginEvent.Source,
		Category:  pluginEvent.Category,
		PluginID:  pluginEvent.PluginID,
		Status:    convertPluginStatus(pluginEvent.Status),
		Error:     pluginEvent.Error,
		Metadata:  pluginEvent.Metadata,
		Timestamp: pluginEvent.Timestamp,
	}
}

// ConvertEventType converts plugins.EventType to events.EventType
func ConvertEventType(eventType plugins.EventType) EventType {
	switch eventType {
	case plugins.EventPluginInitializing:
		return EventPluginInitializing
	case plugins.EventPluginInitialized:
		return EventPluginInitialized
	case plugins.EventPluginStarting:
		return EventPluginStarting
	case plugins.EventPluginStarted:
		return EventPluginStarted
	case plugins.EventPluginStopping:
		return EventPluginStopping
	case plugins.EventPluginStopped:
		return EventPluginStopped
	case plugins.EventHealthCheckStarted:
		return EventHealthCheckStarted
	case plugins.EventHealthCheckRunning:
		return EventHealthCheckRunning
	case plugins.EventHealthCheckDone:
		return EventHealthCheckDone
	case plugins.EventHealthStatusOK:
		return EventHealthStatusOK
	case plugins.EventHealthStatusWarning:
		return EventHealthStatusWarning
	case plugins.EventHealthStatusCritical:
		return EventHealthStatusCritical
	case plugins.EventHealthStatusUnknown:
		return EventHealthStatusUnknown
	case plugins.EventHealthMetricsChanged:
		return EventHealthMetricsChanged
	case plugins.EventHealthThresholdHit:
		return EventHealthThresholdHit
	case plugins.EventHealthStatusChanged:
		return EventHealthStatusChanged
	case plugins.EventHealthCheckFailed:
		return EventHealthCheckFailed
	case plugins.EventConfigurationChanged:
		return EventConfigurationChanged
	case plugins.EventConfigurationInvalid:
		return EventConfigurationInvalid
	case plugins.EventConfigurationApplied:
		return EventConfigurationApplied
	case plugins.EventDependencyMissing:
		return EventDependencyMissing
	case plugins.EventDependencyStatusChanged:
		return EventDependencyStatusChanged
	case plugins.EventDependencyError:
		return EventDependencyError
	case plugins.EventResourceExhausted:
		return EventResourceExhausted
	case plugins.EventPerformanceDegraded:
		return EventPerformanceDegraded
	case plugins.EventResourceCreated:
		return EventResourceCreated
	case plugins.EventResourceModified:
		return EventResourceModified
	case plugins.EventResourceDeleted:
		return EventResourceDeleted
	case plugins.EventResourceUnavailable:
		return EventResourceUnavailable
	case plugins.EventErrorOccurred:
		return EventErrorOccurred
	case plugins.EventErrorResolved:
		return EventErrorResolved
	case plugins.EventPanicRecovered:
		return EventPanicRecovered
	case plugins.EventSecurityViolation:
		return EventSecurityViolation
	case plugins.EventAuthenticationFailed:
		return EventAuthenticationFailed
	case plugins.EventAuthorizationDenied:
		return EventAuthorizationDenied
	case plugins.EventUpgradeAvailable:
		return EventUpgradeAvailable
	case plugins.EventUpgradeInitiated:
		return EventUpgradeInitiated
	case plugins.EventUpgradeValidating:
		return EventUpgradeValidating
	case plugins.EventUpgradeInProgress:
		return EventUpgradeInProgress
	case plugins.EventUpgradeCompleted:
		return EventUpgradeCompleted
	case plugins.EventUpgradeFailed:
		return EventUpgradeFailed
	case plugins.EventRollbackInitiated:
		return EventRollbackInitiated
	case plugins.EventRollbackInProgress:
		return EventRollbackInProgress
	case plugins.EventRollbackCompleted:
		return EventRollbackCompleted
	case plugins.EventRollbackFailed:
		return EventRollbackFailed
	default:
		// For unknown event types, return a default system event
		return EventSystemError
	}
}

// ConvertPriority converts plugins priority to events priority
func ConvertPriority(priority int) Priority {
	switch priority {
	case plugins.PriorityLow:
		return PriorityLow
	case plugins.PriorityNormal:
		return PriorityNormal
	case plugins.PriorityHigh:
		return PriorityHigh
	case plugins.PriorityCritical:
		return PriorityCritical
	default:
		return PriorityNormal
	}
}

// ConvertLynxEvent converts a LynxEvent back to PluginEvent (for backward compatibility)
func ConvertLynxEvent(lynxEvent LynxEvent) plugins.PluginEvent {
	return plugins.PluginEvent{
		Type:      ConvertEventTypeBack(lynxEvent.EventType),
		Priority:  ConvertPriorityBack(lynxEvent.Priority),
		Source:    lynxEvent.Source,
		Category:  lynxEvent.Category,
		PluginID:  lynxEvent.PluginID,
		Status:    convertStringToPluginStatus(lynxEvent.Status),
		Error:     lynxEvent.Error,
		Metadata:  lynxEvent.Metadata,
		Timestamp: lynxEvent.Timestamp,
	}
}

// ConvertEventTypeBack converts events.EventType back to plugins.EventType
func ConvertEventTypeBack(eventType EventType) plugins.EventType {
	switch eventType {
	case EventPluginInitializing:
		return plugins.EventPluginInitializing
	case EventPluginInitialized:
		return plugins.EventPluginInitialized
	case EventPluginStarting:
		return plugins.EventPluginStarting
	case EventPluginStarted:
		return plugins.EventPluginStarted
	case EventPluginStopping:
		return plugins.EventPluginStopping
	case EventPluginStopped:
		return plugins.EventPluginStopped
	case EventHealthCheckStarted:
		return plugins.EventHealthCheckStarted
	case EventHealthCheckRunning:
		return plugins.EventHealthCheckRunning
	case EventHealthCheckDone:
		return plugins.EventHealthCheckDone
	case EventHealthStatusOK:
		return plugins.EventHealthStatusOK
	case EventHealthStatusWarning:
		return plugins.EventHealthStatusWarning
	case EventHealthStatusCritical:
		return plugins.EventHealthStatusCritical
	case EventHealthStatusUnknown:
		return plugins.EventHealthStatusUnknown
	case EventHealthMetricsChanged:
		return plugins.EventHealthMetricsChanged
	case EventHealthThresholdHit:
		return plugins.EventHealthThresholdHit
	case EventHealthStatusChanged:
		return plugins.EventHealthStatusChanged
	case EventHealthCheckFailed:
		return plugins.EventHealthCheckFailed
	case EventConfigurationChanged:
		return plugins.EventConfigurationChanged
	case EventConfigurationInvalid:
		return plugins.EventConfigurationInvalid
	case EventConfigurationApplied:
		return plugins.EventConfigurationApplied
	case EventDependencyMissing:
		return plugins.EventDependencyMissing
	case EventDependencyStatusChanged:
		return plugins.EventDependencyStatusChanged
	case EventDependencyError:
		return plugins.EventDependencyError
	case EventResourceExhausted:
		return plugins.EventResourceExhausted
	case EventPerformanceDegraded:
		return plugins.EventPerformanceDegraded
	case EventResourceCreated:
		return plugins.EventResourceCreated
	case EventResourceModified:
		return plugins.EventResourceModified
	case EventResourceDeleted:
		return plugins.EventResourceDeleted
	case EventResourceUnavailable:
		return plugins.EventResourceUnavailable
	case EventErrorOccurred:
		return plugins.EventErrorOccurred
	case EventErrorResolved:
		return plugins.EventErrorResolved
	case EventPanicRecovered:
		return plugins.EventPanicRecovered
	case EventSecurityViolation:
		return plugins.EventSecurityViolation
	case EventAuthenticationFailed:
		return plugins.EventAuthenticationFailed
	case EventAuthorizationDenied:
		return plugins.EventAuthorizationDenied
	case EventUpgradeAvailable:
		return plugins.EventUpgradeAvailable
	case EventUpgradeInitiated:
		return plugins.EventUpgradeInitiated
	case EventUpgradeValidating:
		return plugins.EventUpgradeValidating
	case EventUpgradeInProgress:
		return plugins.EventUpgradeInProgress
	case EventUpgradeCompleted:
		return plugins.EventUpgradeCompleted
	case EventUpgradeFailed:
		return plugins.EventUpgradeFailed
	case EventRollbackInitiated:
		return plugins.EventRollbackInitiated
	case EventRollbackInProgress:
		return plugins.EventRollbackInProgress
	case EventRollbackCompleted:
		return plugins.EventRollbackCompleted
	case EventRollbackFailed:
		return plugins.EventRollbackFailed
	default:
		// Fallback to a generic error event in plugins package
		return plugins.EventErrorOccurred
	}
}

// ConvertPriorityBack converts events priority back to plugins priority
func ConvertPriorityBack(priority Priority) int {
	switch priority {
	case PriorityLow:
		return plugins.PriorityLow
	case PriorityNormal:
		return plugins.PriorityNormal
	case PriorityHigh:
		return plugins.PriorityHigh
	case PriorityCritical:
		return plugins.PriorityCritical
	default:
		return plugins.PriorityNormal
	}
}

// convertPluginStatus converts plugins.PluginStatus to string
func convertPluginStatus(status plugins.PluginStatus) string {
	switch status {
	case plugins.StatusInactive:
		return "inactive"
	case plugins.StatusInitializing:
		return "initializing"
	case plugins.StatusActive:
		return "active"
	case plugins.StatusStopping:
		return "stopping"
	case plugins.StatusTerminated:
		return "terminated"
	case plugins.StatusFailed:
		return "failed"
	case plugins.StatusSuspended:
		return "suspended"
	default:
		return "inactive"
	}
}

// convertStringToPluginStatus converts string to plugins.PluginStatus
func convertStringToPluginStatus(status string) plugins.PluginStatus {
	switch status {
	case "inactive":
		return plugins.StatusInactive
	case "initializing":
		return plugins.StatusInitializing
	case "active":
		return plugins.StatusActive
	case "stopping":
		return plugins.StatusStopping
	case "terminated":
		return plugins.StatusTerminated
	case "failed":
		return plugins.StatusFailed
	case "suspended":
		return plugins.StatusSuspended
	default:
		return plugins.StatusInactive
	}
}

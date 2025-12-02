package events

// EventClassifier handles event classification and routing to appropriate buses
type EventClassifier struct {
	// Event type to bus mapping
	typeToBus map[EventType]BusType

	// Plugin ID to bus mapping (for special cases)
	pluginToBus map[string]BusType
}

// NewEventClassifier creates a new event classifier with default mappings
func NewEventClassifier() *EventClassifier {
	classifier := &EventClassifier{
		typeToBus:   make(map[EventType]BusType),
		pluginToBus: make(map[string]BusType),
	}

	// Initialize default event type to bus mappings
	classifier.initDefaultMappings()

	return classifier
}

// initDefaultMappings initializes the default event type to bus mappings
func (c *EventClassifier) initDefaultMappings() {
	// Plugin lifecycle events -> Plugin bus
	c.typeToBus[EventPluginInitializing] = BusTypePlugin
	c.typeToBus[EventPluginInitialized] = BusTypePlugin
	c.typeToBus[EventPluginStarting] = BusTypePlugin
	c.typeToBus[EventPluginStarted] = BusTypePlugin
	c.typeToBus[EventPluginStopping] = BusTypePlugin
	c.typeToBus[EventPluginStopped] = BusTypePlugin

	// Health check events -> Health bus
	c.typeToBus[EventHealthCheckStarted] = BusTypeHealth
	c.typeToBus[EventHealthCheckRunning] = BusTypeHealth
	c.typeToBus[EventHealthCheckDone] = BusTypeHealth
	c.typeToBus[EventHealthStatusOK] = BusTypeHealth
	c.typeToBus[EventHealthStatusWarning] = BusTypeHealth
	c.typeToBus[EventHealthStatusCritical] = BusTypeHealth
	c.typeToBus[EventHealthStatusUnknown] = BusTypeHealth
	c.typeToBus[EventHealthMetricsChanged] = BusTypeHealth
	c.typeToBus[EventHealthThresholdHit] = BusTypeHealth
	c.typeToBus[EventHealthStatusChanged] = BusTypeHealth
	c.typeToBus[EventHealthCheckFailed] = BusTypeHealth

	// Add missing event type mappings for better routing
	// Note: EventHealthStatusError is not defined, using EventHealthStatusCritical instead

	// Add custom event type mappings for better isolation
	// Business events -> Business bus (default for unknown types)
	// System events -> System bus
	// Plugin events -> Plugin bus
	// Health events -> Health bus
	// Config events -> Config bus
	// Resource events -> Resource bus
	// Security events -> Security bus
	// Metrics events -> Metrics bus

	// Configuration events -> Config bus
	c.typeToBus[EventConfigurationChanged] = BusTypeConfig
	c.typeToBus[EventConfigurationInvalid] = BusTypeConfig
	c.typeToBus[EventConfigurationApplied] = BusTypeConfig

	// Dependency events -> System bus
	c.typeToBus[EventDependencyMissing] = BusTypeSystem
	c.typeToBus[EventDependencyStatusChanged] = BusTypeSystem
	c.typeToBus[EventDependencyError] = BusTypeSystem

	// Resource events -> Resource bus
	c.typeToBus[EventResourceExhausted] = BusTypeResource
	c.typeToBus[EventPerformanceDegraded] = BusTypeResource
	c.typeToBus[EventResourceCreated] = BusTypeResource
	c.typeToBus[EventResourceModified] = BusTypeResource
	c.typeToBus[EventResourceDeleted] = BusTypeResource
	c.typeToBus[EventResourceUnavailable] = BusTypeResource

	// System events -> System bus
	c.typeToBus[EventSystemStart] = BusTypeSystem
	c.typeToBus[EventSystemShutdown] = BusTypeSystem
	c.typeToBus[EventSystemError] = BusTypeSystem
	c.typeToBus[EventErrorOccurred] = BusTypeSystem
	c.typeToBus[EventErrorResolved] = BusTypeSystem
	c.typeToBus[EventPanicRecovered] = BusTypeSystem

	// Security events -> Security bus
	c.typeToBus[EventSecurityViolation] = BusTypeSecurity
	c.typeToBus[EventAuthenticationFailed] = BusTypeSecurity
	c.typeToBus[EventAuthorizationDenied] = BusTypeSecurity

	// Upgrade events -> System bus
	c.typeToBus[EventUpgradeAvailable] = BusTypeSystem
	c.typeToBus[EventUpgradeInitiated] = BusTypeSystem
	c.typeToBus[EventUpgradeValidating] = BusTypeSystem
	c.typeToBus[EventUpgradeInProgress] = BusTypeSystem
	c.typeToBus[EventUpgradeCompleted] = BusTypeSystem
	c.typeToBus[EventUpgradeFailed] = BusTypeSystem
	c.typeToBus[EventRollbackInitiated] = BusTypeSystem
	c.typeToBus[EventRollbackInProgress] = BusTypeSystem
	c.typeToBus[EventRollbackCompleted] = BusTypeSystem
	c.typeToBus[EventRollbackFailed] = BusTypeSystem
}

// GetBusType determines which bus should handle the given event
func (c *EventClassifier) GetBusType(event LynxEvent) BusType {
	// First check if there's a special mapping for this plugin
	if busType, exists := c.pluginToBus[event.PluginID]; exists {
		return busType
	}

	// Then check the event type mapping
	if busType, exists := c.typeToBus[event.EventType]; exists {
		return busType
	}

	// Default to business bus for unknown event types
	return BusTypeBusiness
}

// SetPluginBusMapping sets a special bus mapping for a specific plugin
func (c *EventClassifier) SetPluginBusMapping(pluginID string, busType BusType) {
	c.pluginToBus[pluginID] = busType
}

// SetEventTypeBusMapping sets a bus mapping for a specific event type
func (c *EventClassifier) SetEventTypeBusMapping(eventType EventType, busType BusType) {
	c.typeToBus[eventType] = busType
}

// RemovePluginBusMapping removes a special bus mapping for a plugin
func (c *EventClassifier) RemovePluginBusMapping(pluginID string) {
	delete(c.pluginToBus, pluginID)
}

// RemoveEventTypeBusMapping removes a bus mapping for an event type
func (c *EventClassifier) RemoveEventTypeBusMapping(eventType EventType) {
	delete(c.typeToBus, eventType)
}

// GetMappings returns all current mappings (for debugging/monitoring)
func (c *EventClassifier) GetMappings() (map[EventType]BusType, map[string]BusType) {
	// Create copies to avoid external modification
	typeMappings := make(map[EventType]BusType)
	pluginMappings := make(map[string]BusType)

	for k, v := range c.typeToBus {
		typeMappings[k] = v
	}

	for k, v := range c.pluginToBus {
		pluginMappings[k] = v
	}

	return typeMappings, pluginMappings
}

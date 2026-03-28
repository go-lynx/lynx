package events

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// initSalt is a process-level random salt computed once at startup.
// It provides sufficient randomness to prevent ID collisions across processes
// without paying the cost of a crypto/rand call for every event.
var initSalt string

func init() {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback: encode the current nanosecond timestamp as hex.
		ts := time.Now().UnixNano()
		b = []byte{
			byte(ts), byte(ts >> 8), byte(ts >> 16), byte(ts >> 24),
			byte(ts >> 32), byte(ts >> 40), byte(ts >> 48), byte(ts >> 56),
		}
	}
	initSalt = hex.EncodeToString(b)
}

// EventType represents the type of event in the Lynx system
type EventType uint32

// Plugin lifecycle event types
const (
	EventPluginInitializing EventType = 0x01
	EventPluginInitialized  EventType = 0x02
	EventPluginStarting     EventType = 0x03
	EventPluginStarted      EventType = 0x04
	EventPluginStopping     EventType = 0x05
	EventPluginStopped      EventType = 0x06
)

// Health check event types
const (
	EventHealthCheckStarted   EventType = 0x10
	EventHealthCheckRunning   EventType = 0x11
	EventHealthCheckDone      EventType = 0x12
	EventHealthStatusOK       EventType = 0x13
	EventHealthStatusWarning  EventType = 0x14
	EventHealthStatusCritical EventType = 0x15
	EventHealthStatusUnknown  EventType = 0x16
	EventHealthMetricsChanged EventType = 0x17
	EventHealthThresholdHit   EventType = 0x18
	EventHealthStatusChanged  EventType = 0x19
	EventHealthCheckFailed    EventType = 0x1A
)

// Configuration event types
const (
	EventConfigurationChanged EventType = 0x20
	EventConfigurationInvalid EventType = 0x21
	EventConfigurationApplied EventType = 0x22
)

// Dependency event types
const (
	EventDependencyMissing       EventType = 0x30
	EventDependencyStatusChanged EventType = 0x31
	EventDependencyError         EventType = 0x32
)

// Resource event types
const (
	EventResourceExhausted   EventType = 0x40
	EventPerformanceDegraded EventType = 0x41
	EventResourceCreated     EventType = 0x42
	EventResourceModified    EventType = 0x43
	EventResourceDeleted     EventType = 0x44
	EventResourceUnavailable EventType = 0x45
)

// System event types
const (
	EventSystemStart             EventType = 0x50
	EventSystemShutdown          EventType = 0x51
	EventSystemError             EventType = 0x52
	EventErrorOccurred           EventType = 0x53
	EventErrorResolved           EventType = 0x54
	EventPanicRecovered          EventType = 0x55
	EventPluginManagerShutdown   EventType = 0x56 // plugin manager shutting down
)

// Security event types
const (
	EventSecurityViolation    EventType = 0x60
	EventAuthenticationFailed EventType = 0x61
	EventAuthorizationDenied  EventType = 0x62
)

// Upgrade and rollback event types are compatibility-only identifiers kept so
// older integrations can still be classified and converted. They are not part
// of Lynx core's preferred lifecycle model.
const (
	EventUpgradeAvailable   EventType = 0x70
	EventUpgradeInitiated   EventType = 0x71
	EventUpgradeValidating  EventType = 0x72
	EventUpgradeInProgress  EventType = 0x73
	EventUpgradeCompleted   EventType = 0x74
	EventUpgradeFailed      EventType = 0x75
	EventRollbackInitiated  EventType = 0x76
	EventRollbackInProgress EventType = 0x77
	EventRollbackCompleted  EventType = 0x78
	EventRollbackFailed     EventType = 0x79
)

// BusType represents different event bus types for isolation
type BusType uint8

const (
	BusTypePlugin   BusType = 0x01 // Plugin lifecycle events
	BusTypeSystem   BusType = 0x02 // System internal events
	BusTypeBusiness BusType = 0x03 // Business events
	BusTypeHealth   BusType = 0x04 // Health check events
	BusTypeConfig   BusType = 0x05 // Configuration events
	BusTypeResource BusType = 0x06 // Resource management events
	BusTypeSecurity BusType = 0x07 // Security events
	BusTypeMetrics  BusType = 0x08 // Monitoring metrics events
)

// Priority represents event priority levels
type Priority int

const (
	PriorityLow      Priority = 0
	PriorityNormal   Priority = 1
	PriorityHigh     Priority = 2
	PriorityCritical Priority = 3
)

// LynxEvent represents a unified event in the Lynx system
type LynxEvent struct {
	EventID   string // Unique event ID for deduplication
	EventType EventType
	Priority  Priority
	Source    string
	Category  string
	PluginID  string
	Status    string
	Error     error
	Metadata  map[string]any
	Timestamp int64
}

// Type returns the event type for kelindar/event compatibility
func (e LynxEvent) Type() uint32 {
	return uint32(e.EventType)
}

// NewLynxEvent creates a new LynxEvent with default values
func NewLynxEvent(eventType EventType, pluginID, source string) LynxEvent {
	now := time.Now()
	return LynxEvent{
		EventID:   generateEventID(pluginID, eventType, now),
		EventType: eventType,
		Priority:  PriorityNormal,
		Source:    source,
		Category:  "default",
		PluginID:  pluginID,
		Timestamp: now.Unix(),
		Metadata:  make(map[string]any),
	}
}

var (
	// eventIDCounter is an atomically-incremented counter that guarantees
	// strict ordering and uniqueness within a single process.
	eventIDCounter atomic.Uint64
)

// generateEventID generates a unique event ID for deduplication.
//
// Format: {pluginID}-{eventType}-{unixSec}-{nano}-{initSalt}-{counter}
//
// Uniqueness guarantee:
//   - initSalt (16 hex chars, random at process start) prevents collisions
//     across different process instances sharing the same nanosecond clock.
//   - counter monotonically increases within the process, preventing collisions
//     even when multiple events are created in the same nanosecond.
//
// This replaces the previous approach that called crypto/rand on every event,
// which caused unnecessary syscall overhead in high-throughput scenarios.
func generateEventID(pluginID string, eventType EventType, t time.Time) string {
	counter := eventIDCounter.Add(1)
	return fmt.Sprintf("%s-%d-%d-%d-%s-%d", pluginID, eventType, t.Unix(), t.Nanosecond(), initSalt, counter)
}

// WithPriority sets the event priority
func (e LynxEvent) WithPriority(priority Priority) LynxEvent {
	e.Priority = priority
	return e
}

// WithCategory sets the event category
func (e LynxEvent) WithCategory(category string) LynxEvent {
	e.Category = category
	return e
}

// WithMetadata sets the event metadata
func (e LynxEvent) WithMetadata(key string, value any) LynxEvent {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	e.Metadata[key] = value
	return e
}

// WithError sets the event error
func (e LynxEvent) WithError(err error) LynxEvent {
	e.Error = err
	return e
}

// WithStatus sets the event status
func (e LynxEvent) WithStatus(status string) LynxEvent {
	e.Status = status
	return e
}

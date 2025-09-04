package events

import (
	"sync"
	"time"
)

// EventHistory manages event history for a bus
type EventHistory struct {
	events      []LynxEvent
	maxSize     int
	maxAge      time.Duration // Maximum age of events to keep
	lastCleanup time.Time     // Last cleanup time
	mu          sync.RWMutex
}

// NewEventHistory creates a new event history with the given maximum size
func NewEventHistory(maxSize int) *EventHistory {
	return &EventHistory{
		events:      make([]LynxEvent, 0, maxSize),
		maxSize:     maxSize,
		maxAge:      24 * time.Hour, // Default: keep events for 24 hours
		lastCleanup: time.Now(),
	}
}

// NewEventHistoryWithAge creates a new event history with custom age limit
func NewEventHistoryWithAge(maxSize int, maxAge time.Duration) *EventHistory {
	return &EventHistory{
		events:      make([]LynxEvent, 0, maxSize),
		maxSize:     maxSize,
		maxAge:      maxAge,
		lastCleanup: time.Now(),
	}
}

// Add adds an event to the history
func (h *EventHistory) Add(event LynxEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Perform cleanup if needed (every 100 events or every hour)
	if len(h.events)%100 == 0 || time.Since(h.lastCleanup) > time.Hour {
		h.cleanupExpiredEvents()
	}

	h.events = append(h.events, event)

	// Trim if exceeds max size
	if len(h.events) > h.maxSize {
		h.events = h.events[len(h.events)-h.maxSize:]
	}
}

// cleanupExpiredEvents removes events older than maxAge
func (h *EventHistory) cleanupExpiredEvents() {
	if h.maxAge <= 0 {
		return
	}

	cutoffTime := time.Now().Add(-h.maxAge).Unix()
	var validEvents []LynxEvent

	for _, event := range h.events {
		if event.Timestamp >= cutoffTime {
			validEvents = append(validEvents, event)
		}
	}

	h.events = validEvents
	h.lastCleanup = time.Now()
}

// SetMaxAge sets the maximum age for events
func (h *EventHistory) SetMaxAge(maxAge time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.maxAge = maxAge
}

// GetMaxAge returns the maximum age setting
func (h *EventHistory) GetMaxAge() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.maxAge
}

// GetEvents returns all events in history
func (h *EventHistory) GetEvents() []LynxEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Directly create result slice to avoid unnecessary intermediate allocation
	result := make([]LynxEvent, len(h.events))
	copy(result, h.events)
	return result
}

// GetEventsByType returns events filtered by type
func (h *EventHistory) GetEventsByType(eventType EventType) []LynxEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []LynxEvent
	for _, event := range h.events {
		if event.EventType == eventType {
			result = append(result, event)
		}
	}
	return result
}

// GetEventsByPlugin returns events filtered by plugin ID
func (h *EventHistory) GetEventsByPlugin(pluginID string) []LynxEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []LynxEvent
	for _, event := range h.events {
		if event.PluginID == pluginID {
			result = append(result, event)
		}
	}
	return result
}

// GetEventsByTimeRange returns events within the specified time range
func (h *EventHistory) GetEventsByTimeRange(from, to int64) []LynxEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []LynxEvent
	for _, event := range h.events {
		if event.Timestamp >= from && event.Timestamp <= to {
			result = append(result, event)
		}
	}
	return result
}

// GetEventsByFilter returns events that match the given filter criteria
func (h *EventHistory) GetEventsByFilter(filter *EventFilter) []LynxEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []LynxEvent
	for _, event := range h.events {
		if h.eventMatchesFilter(event, filter) {
			result = append(result, event)
		}
	}
	return result
}

// eventMatchesFilter checks if an event matches the given filter
func (h *EventHistory) eventMatchesFilter(event LynxEvent, filter *EventFilter) bool {
	if filter == nil {
		return true
	}

	// Check event types
	if len(filter.EventTypes) > 0 {
		typeMatch := false
		for _, filterType := range filter.EventTypes {
			if event.EventType == filterType {
				typeMatch = true
				break
			}
		}
		if !typeMatch {
			return false
		}
	}

	// Check priorities
	if len(filter.Priorities) > 0 {
		priorityMatch := false
		for _, filterPriority := range filter.Priorities {
			if event.Priority == filterPriority {
				priorityMatch = true
				break
			}
		}
		if !priorityMatch {
			return false
		}
	}

	// Check sources
	if len(filter.Sources) > 0 {
		sourceMatch := false
		for _, filterSource := range filter.Sources {
			if event.Source == filterSource {
				sourceMatch = true
				break
			}
		}
		if !sourceMatch {
			return false
		}
	}

	// Check categories
	if len(filter.Categories) > 0 {
		categoryMatch := false
		for _, filterCategory := range filter.Categories {
			if event.Category == filterCategory {
				categoryMatch = true
				break
			}
		}
		if !categoryMatch {
			return false
		}
	}

	// Check plugin IDs
	if len(filter.PluginIDs) > 0 {
		pluginMatch := false
		for _, filterPluginID := range filter.PluginIDs {
			if event.PluginID == filterPluginID {
				pluginMatch = true
				break
			}
		}
		if !pluginMatch {
			return false
		}
	}

	// Check time range
	if filter.FromTime > 0 && event.Timestamp < filter.FromTime {
		return false
	}
	if filter.ToTime > 0 && event.Timestamp > filter.ToTime {
		return false
	}

	// Check metadata
	if len(filter.Metadata) > 0 {
		for key, value := range filter.Metadata {
			if event.Metadata == nil {
				return false
			}
			if eventValue, exists := event.Metadata[key]; !exists || eventValue != value {
				return false
			}
		}
	}

	// Check error condition
	if filter.HasError && event.Error == nil {
		return false
	}

	// Check statuses
	if len(filter.Statuses) > 0 {
		statusMatch := false
		for _, filterStatus := range filter.Statuses {
			if event.Status == filterStatus {
				statusMatch = true
				break
			}
		}
		if !statusMatch {
			return false
		}
	}

	return true
}

// Clear clears all events from history
func (h *EventHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.events = h.events[:0]
}

// Size returns the current number of events in history
func (h *EventHistory) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.events)
}

// MaxSize returns the maximum size of the history
func (h *EventHistory) MaxSize() int {
	return h.maxSize
}

// SetMaxSize sets the maximum size of the history
func (h *EventHistory) SetMaxSize(maxSize int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.maxSize = maxSize

	// Trim if current size exceeds new max size
	if len(h.events) > maxSize {
		h.events = h.events[len(h.events)-maxSize:]
	}
}

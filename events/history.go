package events

import (
	"reflect"
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
	// Index for faster queries
	byPluginID  map[string][]int    // pluginID -> indices in events slice
	byEventType map[EventType][]int // eventType -> indices in events slice
	indexMu     sync.RWMutex
}

// NewEventHistory creates a new event history with the given maximum size
func NewEventHistory(maxSize int) *EventHistory {
	return &EventHistory{
		events:      make([]LynxEvent, 0, maxSize),
		maxSize:     maxSize,
		maxAge:      24 * time.Hour, // Default: keep events for 24 hours
		lastCleanup: time.Now(),
		byPluginID:  make(map[string][]int),
		byEventType: make(map[EventType][]int),
	}
}

// NewEventHistoryWithAge creates a new event history with custom age limit
func NewEventHistoryWithAge(maxSize int, maxAge time.Duration) *EventHistory {
	return &EventHistory{
		events:      make([]LynxEvent, 0, maxSize),
		maxSize:     maxSize,
		maxAge:      maxAge,
		lastCleanup: time.Now(),
		byPluginID:  make(map[string][]int),
		byEventType: make(map[EventType][]int),
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

	eventIndex := len(h.events)
	h.events = append(h.events, event)

	// Update indexes for faster queries
	// Limit index size to prevent unbounded growth
	h.indexMu.Lock()
	if event.PluginID != "" {
		indices := h.byPluginID[event.PluginID]
		// Limit per-plugin index size to maxSize to prevent memory growth
		if len(indices) < h.maxSize {
			h.byPluginID[event.PluginID] = append(indices, eventIndex)
		} else {
			// Optimized: Use copy instead of slice operation to prevent memory leak
			// Slice operation [1:] keeps the underlying array, causing memory leak
			copy(indices, indices[1:])
			h.byPluginID[event.PluginID] = append(indices[:len(indices)-1], eventIndex)
		}
	}
	indices := h.byEventType[event.EventType]
	// Limit per-event-type index size to maxSize
	if len(indices) < h.maxSize {
		h.byEventType[event.EventType] = append(indices, eventIndex)
	} else {
		// Optimized: Use copy instead of slice operation to prevent memory leak
		// Slice operation [1:] keeps the underlying array, causing memory leak
		copy(indices, indices[1:])
		h.byEventType[event.EventType] = append(indices[:len(indices)-1], eventIndex)
	}
	h.indexMu.Unlock()

	// Trim if exceeds max size
	// Use more aggressive trimming to prevent memory growth
	if len(h.events) > h.maxSize {
		trimCount := len(h.events) - h.maxSize
		// Trim slightly more to reduce frequent trimming
		if trimCount < h.maxSize/4 {
			trimCount = h.maxSize / 4 // Trim 25% when close to limit
		}

		// Optimized: Update indexes incrementally instead of full rebuild
		// Remove indices for trimmed events and adjust remaining indices
		h.indexMu.Lock()
		// Remove indices for events that will be trimmed (indices 0 to trimCount-1)
		for pluginID, indices := range h.byPluginID {
			newIndices := make([]int, 0, len(indices))
			for _, idx := range indices {
				if idx >= trimCount {
					// Adjust index by subtracting trimCount
					newIndices = append(newIndices, idx-trimCount)
				}
			}
			if len(newIndices) == 0 {
				delete(h.byPluginID, pluginID)
			} else {
				h.byPluginID[pluginID] = newIndices
			}
		}
		for eventType, indices := range h.byEventType {
			newIndices := make([]int, 0, len(indices))
			for _, idx := range indices {
				if idx >= trimCount {
					// Adjust index by subtracting trimCount
					newIndices = append(newIndices, idx-trimCount)
				}
			}
			if len(newIndices) == 0 {
				delete(h.byEventType, eventType)
			} else {
				h.byEventType[eventType] = newIndices
			}
		}
		h.indexMu.Unlock()

		// Trim from the beginning
		h.events = h.events[trimCount:]
	}
}

// cleanupExpiredEvents removes events older than maxAge
// Optimized to reduce memory allocations
func (h *EventHistory) cleanupExpiredEvents() {
	if h.maxAge <= 0 {
		return
	}

	cutoffTime := time.Now().Add(-h.maxAge).Unix()

	// Count valid events first to pre-allocate slice
	validCount := 0
	for _, event := range h.events {
		if event.Timestamp >= cutoffTime {
			validCount++
		}
	}

	// If all events are valid, no cleanup needed
	if validCount == len(h.events) {
		h.lastCleanup = time.Now()
		return
	}

	// Pre-allocate slice with known capacity
	validEvents := make([]LynxEvent, 0, validCount)
	for _, event := range h.events {
		if event.Timestamp >= cutoffTime {
			validEvents = append(validEvents, event)
		}
	}

	// Optimized: Update indexes incrementally instead of full rebuild
	// Build a mapping from old index to new index
	oldToNewIndex := make(map[int]int, validCount)
	newIdx := 0
	for oldIdx, event := range h.events {
		if event.Timestamp >= cutoffTime {
			oldToNewIndex[oldIdx] = newIdx
			newIdx++
		}
	}

	// Update indexes using the mapping
	h.indexMu.Lock()
	// Update byPluginID index
	for pluginID, indices := range h.byPluginID {
		newIndices := make([]int, 0, len(indices))
		for _, oldIdx := range indices {
			if newIdx, exists := oldToNewIndex[oldIdx]; exists {
				newIndices = append(newIndices, newIdx)
			}
		}
		if len(newIndices) == 0 {
			delete(h.byPluginID, pluginID)
		} else {
			h.byPluginID[pluginID] = newIndices
		}
	}
	// Update byEventType index
	for eventType, indices := range h.byEventType {
		newIndices := make([]int, 0, len(indices))
		for _, oldIdx := range indices {
			if newIdx, exists := oldToNewIndex[oldIdx]; exists {
				newIndices = append(newIndices, newIdx)
			}
		}
		if len(newIndices) == 0 {
			delete(h.byEventType, eventType)
		} else {
			h.byEventType[eventType] = newIndices
		}
	}
	h.indexMu.Unlock()

	h.events = validEvents
	h.lastCleanup = time.Now()
}

// rebuildIndexes rebuilds all indexes from scratch
func (h *EventHistory) rebuildIndexes() {
	h.indexMu.Lock()
	defer h.indexMu.Unlock()

	// Clear existing indexes
	h.byPluginID = make(map[string][]int)
	h.byEventType = make(map[EventType][]int)

	// Rebuild indexes with capacity hints
	// Pre-allocate maps with estimated size to reduce allocations
	if len(h.events) > 0 {
		estimatedPluginCount := min(len(h.events)/10, 100) // Estimate 10 events per plugin, max 100 plugins
		if estimatedPluginCount > 0 {
			for k := range h.byPluginID {
				delete(h.byPluginID, k) // Clear old entries
			}
		}
		for k := range h.byEventType {
			delete(h.byEventType, k) // Clear old entries
		}
	}

	// Rebuild indexes
	for i, event := range h.events {
		if event.PluginID != "" {
			if h.byPluginID[event.PluginID] == nil {
				// Pre-allocate with reasonable capacity
				h.byPluginID[event.PluginID] = make([]int, 0, min(h.maxSize/10, 100))
			}
			h.byPluginID[event.PluginID] = append(h.byPluginID[event.PluginID], i)
		}
		if h.byEventType[event.EventType] == nil {
			// Pre-allocate with reasonable capacity
			h.byEventType[event.EventType] = make([]int, 0, min(h.maxSize/10, 100))
		}
		h.byEventType[event.EventType] = append(h.byEventType[event.EventType], i)
	}
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

// GetEventsByType returns events filtered by type (optimized with index)
func (h *EventHistory) GetEventsByType(eventType EventType) []LynxEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Use index for faster lookup
	h.indexMu.RLock()
	indices, hasIndex := h.byEventType[eventType]
	h.indexMu.RUnlock()

	if hasIndex && len(indices) > 0 {
		result := make([]LynxEvent, 0, len(indices))
		for _, idx := range indices {
			if idx < len(h.events) {
				result = append(result, h.events[idx])
			}
		}
		return result
	}

	// Fallback to linear search if index not available
	var result []LynxEvent
	for _, event := range h.events {
		if event.EventType == eventType {
			result = append(result, event)
		}
	}
	return result
}

// GetEventsByPlugin returns events filtered by plugin ID (optimized with index)
func (h *EventHistory) GetEventsByPlugin(pluginID string) []LynxEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Use index for faster lookup
	h.indexMu.RLock()
	indices, hasIndex := h.byPluginID[pluginID]
	h.indexMu.RUnlock()

	if hasIndex && len(indices) > 0 {
		result := make([]LynxEvent, 0, len(indices))
		for _, idx := range indices {
			if idx < len(h.events) {
				result = append(result, h.events[idx])
			}
		}
		return result
	}

	// Fallback to linear search if index not available
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

	// Check metadata (deep equality to support maps/slices without panic)
	if len(filter.Metadata) > 0 {
		if event.Metadata == nil {
			return false
		}
		for key, value := range filter.Metadata {
			if eventValue, exists := event.Metadata[key]; !exists || !reflect.DeepEqual(eventValue, value) {
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

	// Clear indexes
	h.indexMu.Lock()
	h.byPluginID = make(map[string][]int)
	h.byEventType = make(map[EventType][]int)
	h.indexMu.Unlock()
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
		trimCount := len(h.events) - maxSize
		h.events = h.events[trimCount:]
		// Rebuild indexes after trimming to ensure index references are valid
		h.rebuildIndexes()
	}
}

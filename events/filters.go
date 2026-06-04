package events

import (
	"reflect"
	"time"
)

// EventFilter selects events by any combination of criteria. An empty filter
// (see IsEmpty) matches everything. See Matches for the AND/OR semantics across
// and within fields.
type EventFilter struct {
	EventTypes []EventType `yaml:"event_types" json:"event_types"`
	Priorities []Priority  `yaml:"priorities" json:"priorities"`
	Sources    []string    `yaml:"sources" json:"sources"`
	Categories []string    `yaml:"categories" json:"categories"`
	PluginIDs  []string    `yaml:"plugin_ids" json:"plugin_ids"`

	// FromTime/ToTime bound event Timestamp (Unix seconds); 0 means unbounded.
	FromTime int64 `yaml:"from_time" json:"from_time"`
	ToTime   int64 `yaml:"to_time" json:"to_time"`

	Metadata map[string]any `yaml:"metadata" json:"metadata"`

	// HasError, when true, keeps only events that carry a non-nil Error.
	HasError bool     `yaml:"has_error" json:"has_error"`
	Statuses []string `yaml:"statuses" json:"statuses"`
}

func deepEqual(a, b any) bool { return reflect.DeepEqual(a, b) }

// NewEventFilter creates a new event filter
func NewEventFilter() *EventFilter {
	return &EventFilter{
		EventTypes: make([]EventType, 0),
		Priorities: make([]Priority, 0),
		Sources:    make([]string, 0),
		Categories: make([]string, 0),
		PluginIDs:  make([]string, 0),
		Statuses:   make([]string, 0),
		Metadata:   make(map[string]any),
	}
}

// WithEventType adds an event type filter
func (f *EventFilter) WithEventType(eventType EventType) *EventFilter {
	f.EventTypes = append(f.EventTypes, eventType)
	return f
}

// WithPriority adds a priority filter
func (f *EventFilter) WithPriority(priority Priority) *EventFilter {
	f.Priorities = append(f.Priorities, priority)
	return f
}

// WithSource adds a source filter
func (f *EventFilter) WithSource(source string) *EventFilter {
	f.Sources = append(f.Sources, source)
	return f
}

// WithCategory adds a category filter
func (f *EventFilter) WithCategory(category string) *EventFilter {
	f.Categories = append(f.Categories, category)
	return f
}

// WithPluginID adds a plugin ID filter
func (f *EventFilter) WithPluginID(pluginID string) *EventFilter {
	f.PluginIDs = append(f.PluginIDs, pluginID)
	return f
}

// WithTimeRange adds a time range filter
func (f *EventFilter) WithTimeRange(from, to time.Time) *EventFilter {
	f.FromTime = from.Unix()
	f.ToTime = to.Unix()
	return f
}

// WithMetadata adds a metadata filter
func (f *EventFilter) WithMetadata(key string, value any) *EventFilter {
	if f.Metadata == nil {
		f.Metadata = make(map[string]any)
	}
	f.Metadata[key] = value
	return f
}

// WithError adds an error filter
func (f *EventFilter) WithError(hasError bool) *EventFilter {
	f.HasError = hasError
	return f
}

// WithStatus adds a status filter
func (f *EventFilter) WithStatus(status string) *EventFilter {
	f.Statuses = append(f.Statuses, status)
	return f
}

// Matches reports whether an event satisfies every set criterion. Each field
// group is ANDed together; values within a group (e.g. EventTypes) are ORed.
// Metadata is compared by deep equality so map/slice values are safe.
func (f *EventFilter) Matches(event LynxEvent) bool {
	if len(f.EventTypes) > 0 {
		found := false
		for _, eventType := range f.EventTypes {
			if event.EventType == eventType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(f.Priorities) > 0 {
		found := false
		for _, priority := range f.Priorities {
			if event.Priority == priority {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(f.Sources) > 0 {
		found := false
		for _, source := range f.Sources {
			if event.Source == source {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(f.Categories) > 0 {
		found := false
		for _, category := range f.Categories {
			if event.Category == category {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(f.PluginIDs) > 0 {
		found := false
		for _, pluginID := range f.PluginIDs {
			if event.PluginID == pluginID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if f.FromTime > 0 && event.Timestamp < f.FromTime {
		return false
	}
	if f.ToTime > 0 && event.Timestamp > f.ToTime {
		return false
	}

	if len(f.Metadata) > 0 {
		for key, expectedValue := range f.Metadata {
			actualValue, exists := event.Metadata[key]
			if !exists {
				return false
			}
			if !deepEqual(actualValue, expectedValue) {
				return false
			}
		}
	}

	if f.HasError && event.Error == nil {
		return false
	}

	if len(f.Statuses) > 0 {
		found := false
		for _, status := range f.Statuses {
			if event.Status == status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// IsEmpty checks if the filter is empty (no filters applied)
func (f *EventFilter) IsEmpty() bool {
	return len(f.EventTypes) == 0 &&
		len(f.Priorities) == 0 &&
		len(f.Sources) == 0 &&
		len(f.Categories) == 0 &&
		len(f.PluginIDs) == 0 &&
		f.FromTime == 0 &&
		f.ToTime == 0 &&
		len(f.Metadata) == 0 &&
		!f.HasError &&
		len(f.Statuses) == 0
}

// Clone returns a deep copy of the filter suitable for use across goroutines/listeners.
// Slices and the Metadata map are copied. For Metadata values, this performs a best-effort
// deep copy for common types (map[string]any, []any, []string). Other values are copied as-is.
func (f *EventFilter) Clone() *EventFilter {
	if f == nil {
		return nil
	}
	nf := &EventFilter{
		EventTypes: append([]EventType(nil), f.EventTypes...),
		Priorities: append([]Priority(nil), f.Priorities...),
		Sources:    append([]string(nil), f.Sources...),
		Categories: append([]string(nil), f.Categories...),
		PluginIDs:  append([]string(nil), f.PluginIDs...),
		FromTime:   f.FromTime,
		ToTime:     f.ToTime,
		HasError:   f.HasError,
		Statuses:   append([]string(nil), f.Statuses...),
	}
	if f.Metadata != nil {
		nf.Metadata = deepCopyMapStringAny(f.Metadata)
	} else {
		nf.Metadata = make(map[string]any)
	}
	return nf
}

func deepCopyMapStringAny(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = deepCopyAny(v)
	}
	return dst
}

func deepCopyAny(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return deepCopyMapStringAny(x)
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = deepCopyAny(x[i])
		}
		return out
	case []string:
		out := make([]string, len(x))
		copy(out, x)
		return out
	case []int:
		out := make([]int, len(x))
		copy(out, x)
		return out
	default:
		// For scalars and unsupported complex types, return as-is
		return x
	}
}

// Package config provides a minimal validation framework used by plugins and core.
package config

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// ValidationContext carries optional metadata for a validation call.
type ValidationContext struct {
	Field string
	Path  string
}

// ValidationIssue represents a single validation message.
type ValidationIssue struct {
	Field   string
	Value   interface{}
	Message string
}

// ValidationResult aggregates validation errors and warnings.
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationIssue
	Warnings []ValidationIssue
	mu       sync.Mutex
}

func (r *ValidationResult) ensureInit() {
	if r == nil {
		return
	}
	// mark valid by default
	if r.Errors == nil && r.Warnings == nil {
		r.Valid = true
	}
}

// AddError records a validation error and marks the result invalid.
func (r *ValidationResult) AddError(field string, value interface{}, message string) {
	r.ensureInit()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Errors = append(r.Errors, ValidationIssue{Field: field, Value: value, Message: message})
	r.Valid = false
}

// AddWarning records a validation warning but does not change validity.
func (r *ValidationResult) AddWarning(field string, value interface{}, message string) {
	r.ensureInit()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Warnings = append(r.Warnings, ValidationIssue{Field: field, Value: value, Message: message})
	if r.Valid == false {
		// keep invalid if already invalid
		return
	}
	if r.Errors == nil {
		r.Valid = true
	}
}

// ValidationRule defines a field validator rule.
type ValidationRule struct {
	Name        string
	Description string
	Validator   func(value interface{}, ctx ValidationContext) error
	Required    bool
}

// ConfigValidator maintains a set of validation rules for a config struct.
type ConfigValidator struct {
	strict bool
	env    string
	rules  map[string]ValidationRule // key: field name of target struct
}

// NewConfigValidator creates a new validator instance.
func NewConfigValidator(strict bool, environment string) *ConfigValidator {
	return &ConfigValidator{
		strict: strict,
		env:    environment,
		rules:  make(map[string]ValidationRule),
	}
}

// AddRule registers a rule for a field.
func (cv *ConfigValidator) AddRule(field string, rule ValidationRule) {
	if cv.rules == nil {
		cv.rules = make(map[string]ValidationRule)
	}
	cv.rules[field] = rule
}

// ValidateConfig validates the provided config struct using registered rules.
// config should be a struct or pointer to struct.
func (cv *ConfigValidator) ValidateConfig(config interface{}, prefix string) *ValidationResult {
	res := &ValidationResult{Valid: true}
	if config == nil {
		res.AddError(prefix, nil, "config is nil")
		return res
	}

	v := reflect.ValueOf(config)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		res.AddError(prefix, nil, "config must be a struct or pointer to struct")
		return res
	}

	for field, rule := range cv.rules {
		fv := v.FieldByName(field)
		if !fv.IsValid() {
			if cv.strict {
				res.AddError(field, nil, "unknown field for validation")
			}
			continue
		}
		var val interface{}
		if fv.CanInterface() {
			val = fv.Interface()
		}
		// Required check for zero values
		if rule.Required && isZeroValue(fv) {
			res.AddError(field, val, "field is required")
			continue
		}
		if rule.Validator != nil {
			if err := rule.Validator(val, ValidationContext{Field: field, Path: prefix}); err != nil {
				res.AddError(field, val, err.Error())
			}
		}
	}
	return res
}

func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface:
		return v.IsNil()
	default:
		z := reflect.Zero(v.Type())
		return reflect.DeepEqual(v.Interface(), z.Interface())
	}
}

// ValidateRange returns a validator ensuring a numeric value is in [min, max].
func ValidateRange(min, max int64) func(value interface{}, ctx ValidationContext) error {
	return func(value interface{}, ctx ValidationContext) error {
		if value == nil {
			return nil
		}
		i, ok := toInt64(value)
		if !ok {
			return fmt.Errorf("expected numeric type, got %T", value)
		}
		if i < min || i > max {
			return fmt.Errorf("must be in range [%d,%d]", min, max)
		}
		return nil
	}
}

// ValidateDuration returns a validator ensuring a duration is in [min, max].
func ValidateDuration(min, max time.Duration) func(value interface{}, ctx ValidationContext) error {
	return func(value interface{}, ctx ValidationContext) error {
		if value == nil {
			return nil
		}
		d, ok := toDuration(value)
		if !ok {
			return fmt.Errorf("expected duration type, got %T", value)
		}
		if d < min || d > max {
			return fmt.Errorf("duration must be in range [%s,%s]", min, max)
		}
		return nil
	}
}

func toInt64(v interface{}) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint:
		return int64(x), true
	case uint8:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		if x > ^uint64(0)>>1 {
			return 0, false
		}
		return int64(x), true
	default:
		// try reflect for pointers to numeric
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr && !rv.IsNil() {
			return toInt64(rv.Elem().Interface())
		}
		return 0, false
	}
}

// DefaultValueSetter applies default values to struct fields by name.
type DefaultValueSetter struct {
	static map[string]interface{}
	funcs  map[string]func(interface{}) interface{}
}

func NewDefaultValueSetter() *DefaultValueSetter {
	return &DefaultValueSetter{static: make(map[string]interface{}), funcs: make(map[string]func(interface{}) interface{})}
}

func (d *DefaultValueSetter) SetDefault(field string, value interface{}) {
	if d.static == nil {
		d.static = make(map[string]interface{})
	}
	d.static[field] = value
}

func (d *DefaultValueSetter) SetDefaultFunc(field string, fn func(interface{}) interface{}) {
	if d.funcs == nil {
		d.funcs = make(map[string]func(interface{}) interface{})
	}
	d.funcs[field] = fn
}

// ApplyDefaults sets default values for zero-value fields in the struct pointed to by cfg.
func (d *DefaultValueSetter) ApplyDefaults(cfg interface{}) error {
	if cfg == nil {
		return errors.New("nil config")
	}
	rv := reflect.ValueOf(cfg)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return errors.New("ApplyDefaults requires pointer to struct")
	}
	s := rv.Elem()
	for name, val := range d.static {
		f := s.FieldByName(name)
		if !f.IsValid() || !f.CanSet() {
			continue
		}
		if isZeroValue(f) {
			vv := reflect.ValueOf(val)
			if vv.IsValid() && vv.Type().AssignableTo(f.Type()) {
				f.Set(vv)
			}
		}
	}
	for name, fn := range d.funcs {
		f := s.FieldByName(name)
		if !f.IsValid() || !f.CanSet() {
			continue
		}
		if isZeroValue(f) {
			cur := interface{}(nil)
			if f.CanInterface() {
				cur = f.Interface()
			}
			def := fn(cur)
			vv := reflect.ValueOf(def)
			if vv.IsValid() && vv.Type().AssignableTo(f.Type()) {
				f.Set(vv)
			}
		}
	}
	return nil
}

// toDuration tries to coerce a variety of types to time.Duration.
func toDuration(v interface{}) (time.Duration, bool) {
	switch x := v.(type) {
	case time.Duration:
		return x, true
	case *time.Duration:
		if x == nil {
			return 0, false
		}
		return *x, true
	default:
		// Support protobuf duration types via reflection (e.g., *durationpb.Duration with AsDuration method)
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr && !rv.IsNil() {
			m := rv.MethodByName("AsDuration")
			if m.IsValid() && m.Type().NumIn() == 0 && m.Type().NumOut() == 1 {
				out := m.Call(nil)
				if len(out) == 1 {
					if d, ok := out[0].Interface().(time.Duration); ok {
						return d, true
					}
				}
			}
		}
		return 0, false
	}
}

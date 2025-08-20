package ptr

// Ptr returns a pointer to the given value.
func Ptr[T any](v T) *T { return &v }

// Deref dereferences the pointer; if nil, returns def.
func Deref[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

// OrDefault returns def when v equals the type's zero value; otherwise returns v.
// Note: works only for comparable types.
func OrDefault[T comparable](v, def T) T {
	var zero T
	if v == zero {
		return def
	}
	return v
}

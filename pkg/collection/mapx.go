package collection

// Keys returns all keys of the map (unordered).
func Keys[M ~map[K]V, K comparable, V any](m M) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Values returns all values of the map.
func Values[M ~map[K]V, K comparable, V any](m M) []V {
	vals := make([]V, 0, len(m))
	for _, v := range m {
		vals = append(vals, v)
	}
	return vals
}

// Merge merges src into dst and returns dst (in-place modification).
func Merge[K comparable, V any](dst, src map[K]V) map[K]V {
	if dst == nil && src == nil {
		return nil
	}
	if dst == nil {
		dst = make(map[K]V, len(src))
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// Invert reverses map[K]V to map[V]K; later entries overwrite earlier ones on conflicts.
func Invert[K comparable, V comparable](m map[K]V) map[V]K {
	res := make(map[V]K, len(m))
	for k, v := range m {
		res[v] = k
	}
	return res
}

package collection

// Map maps slice elements to another type.
func Map[A any, B any](in []A, f func(A) B) []B {
	if len(in) == 0 {
		return []B{}
	}
	out := make([]B, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

// Filter filters slice elements by predicate.
func Filter[T any](in []T, pred func(T) bool) []T {
	if len(in) == 0 {
		return []T{}
	}
	out := make([]T, 0, len(in))
	for _, v := range in {
		if pred(v) {
			out = append(out, v)
		}
	}
	return out
}

// Unique returns a de-duplicated slice preserving the first occurrence order.
func Unique[T comparable](in []T) []T {
	if len(in) == 0 {
		return []T{}
	}
	seen := make(map[T]struct{}, len(in))
	out := make([]T, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// Chunk splits the slice into chunks of size n. Returns empty when n<=0.
func Chunk[T any](in []T, n int) [][]T {
	if n <= 0 || len(in) == 0 {
		return [][]T{}
	}
	res := make([][]T, 0, (len(in)+n-1)/n)
	for i := 0; i < len(in); i += n {
		end := i + n
		if end > len(in) {
			end = len(in)
		}
		res = append(res, in[i:end])
	}
	return res
}

// GroupBy groups slice elements by key.
func GroupBy[T any, K comparable](in []T, key func(T) K) map[K][]T {
	m := make(map[K][]T)
	for _, v := range in {
		k := key(v)
		m[k] = append(m[k], v)
	}
	return m
}

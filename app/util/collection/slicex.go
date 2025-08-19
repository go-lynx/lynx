package collection

// Map 将切片元素映射为另一类型。
func Map[A any, B any](in []A, f func(A) B) []B {
	if len(in) == 0 { return []B{} }
	out := make([]B, len(in))
	for i, v := range in { out[i] = f(v) }
	return out
}

// Filter 过滤切片元素。
func Filter[T any](in []T, pred func(T) bool) []T {
	if len(in) == 0 { return []T{} }
	out := make([]T, 0, len(in))
	for _, v := range in { if pred(v) { out = append(out, v) } }
	return out
}

// Unique 返回去重后的新切片（保持首次出现顺序）。
func Unique[T comparable](in []T) []T {
	if len(in) == 0 { return []T{} }
	seen := make(map[T]struct{}, len(in))
	out := make([]T, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok { continue }
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// Chunk 将切片按 n 分块。n<=0 时返回空。
func Chunk[T any](in []T, n int) [][]T {
	if n <= 0 || len(in) == 0 { return [][]T{} }
	res := make([][]T, 0, (len(in)+n-1)/n)
	for i := 0; i < len(in); i += n {
		end := i + n
		if end > len(in) { end = len(in) }
		res = append(res, in[i:end])
	}
	return res
}

// GroupBy 将切片按 key 分组。
func GroupBy[T any, K comparable](in []T, key func(T) K) map[K][]T {
	m := make(map[K][]T)
	for _, v := range in {
		k := key(v)
		m[k] = append(m[k], v)
	}
	return m
}

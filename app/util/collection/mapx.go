package collection

// Keys 返回 map 的所有键（未排序）。
func Keys[M ~map[K]V, K comparable, V any](m M) []K {
	keys := make([]K, 0, len(m))
	for k := range m { keys = append(keys, k) }
	return keys
}

// Values 返回 map 的所有值。
func Values[M ~map[K]V, K comparable, V any](m M) []V {
	vals := make([]V, 0, len(m))
	for _, v := range m { vals = append(vals, v) }
	return vals
}

// Merge 将 src 合并到 dst 并返回 dst（原地修改）。
func Merge[K comparable, V any](dst, src map[K]V) map[K]V {
	if dst == nil && src == nil { return nil }
	if dst == nil { dst = make(map[K]V, len(src)) }
	for k, v := range src { dst[k] = v }
	return dst
}

// Invert 将 map[K]V 反转为 map[V]K；若值存在冲突，后者覆盖前者。
func Invert[K comparable, V comparable](m map[K]V) map[V]K {
	res := make(map[V]K, len(m))
	for k, v := range m { res[v] = k }
	return res
}

package collection

type Set[T comparable] map[T]struct{}

func NewSet[T comparable](vals ...T) Set[T] {
	s := make(Set[T], len(vals))
	for _, v := range vals { s[v] = struct{}{} }
	return s
}

func (s Set[T]) Add(v T) { s[v] = struct{}{} }
func (s Set[T]) Del(v T) { delete(s, v) }
func (s Set[T]) Has(v T) bool { _, ok := s[v]; return ok }
func (s Set[T]) Len() int { return len(s) }

func (s Set[T]) ToSlice() []T {
	out := make([]T, 0, len(s))
	for v := range s { out = append(out, v) }
	return out
}

func (s Set[T]) Union(t Set[T]) Set[T] {
	res := make(Set[T], len(s)+len(t))
	for v := range s { res[v] = struct{}{} }
	for v := range t { res[v] = struct{}{} }
	return res
}

func (s Set[T]) Intersect(t Set[T]) Set[T] {
	res := make(Set[T])
	if len(s) > len(t) { s, t = t, s }
	for v := range s {
		if _, ok := t[v]; ok { res[v] = struct{}{} }
	}
	return res
}

func (s Set[T]) Diff(t Set[T]) Set[T] {
	res := make(Set[T])
	for v := range s {
		if _, ok := t[v]; !ok { res[v] = struct{}{} }
	}
	return res
}

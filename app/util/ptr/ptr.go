package ptr

// Ptr 返回任意值的指针，避免临时变量。
func Ptr[T any](v T) *T { return &v }

// Deref 解引用指针；若指针为 nil，返回默认值 def。
func Deref[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

// OrDefault 当 v 等于类型零值时，返回 def；否则返回 v。
// 注意：仅对可比较类型生效（comparable）。
func OrDefault[T comparable](v, def T) T {
	var zero T
	if v == zero {
		return def
	}
	return v
}

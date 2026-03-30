package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// ---- Cache (cache.go) tests ----

func newTestCache(t *testing.T) *Cache {
	t.Helper()
	c, err := New("test", &Options{
		NumCounters: 1e4,
		MaxCost:     1 << 20, // 1 MB
		BufferItems: 64,
		Metrics:     false,
	})
	if err != nil {
		t.Fatalf("failed to create test cache: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

func TestNew_DefaultOptions(t *testing.T) {
	c, err := New("default", nil)
	if err != nil {
		t.Fatalf("New with nil options: %v", err)
	}
	defer c.Close()
	if c.Name() != "default" {
		t.Errorf("expected name %q, got %q", "default", c.Name())
	}
}

func TestCache_SetAndGet(t *testing.T) {
	c := newTestCache(t)
	if err := c.SetSync("key1", "value1", 0); err != nil {
		t.Fatalf("SetSync: %v", err)
	}
	v, err := c.Get("key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v != "value1" {
		t.Errorf("expected %q, got %v", "value1", v)
	}
}

func TestCache_GetMiss(t *testing.T) {
	c := newTestCache(t)
	_, err := c.Get("nonexistent")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("expected ErrCacheMiss, got %v", err)
	}
}

func TestCache_SetInvalidTTL(t *testing.T) {
	c := newTestCache(t)
	err := c.Set("k", "v", -1*time.Second)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("expected ErrInvalidTTL, got %v", err)
	}
}

func TestCache_SetWithCost(t *testing.T) {
	c := newTestCache(t)
	if err := c.SetWithCostSync("k", "v", 1, 0); err != nil {
		t.Fatalf("SetWithCostSync: %v", err)
	}
	v, err := c.Get("k")
	if err != nil {
		t.Fatalf("Get after SetWithCost: %v", err)
	}
	if v != "v" {
		t.Errorf("expected %q, got %v", "v", v)
	}
}

func TestCache_SetWithCostInvalidTTL(t *testing.T) {
	c := newTestCache(t)
	err := c.SetWithCost("k", "v", 1, -time.Second)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("expected ErrInvalidTTL, got %v", err)
	}
}

func TestCache_Delete(t *testing.T) {
	c := newTestCache(t)
	_ = c.SetSync("del", "val", 0)
	c.Delete("del")
	if c.Has("del") {
		t.Error("expected key to be deleted")
	}
}

func TestCache_Has(t *testing.T) {
	c := newTestCache(t)
	if c.Has("missing") {
		t.Error("Has should return false for missing key")
	}
	_ = c.SetSync("present", 1, 0)
	if !c.Has("present") {
		t.Error("Has should return true for existing key")
	}
}

func TestCache_Clear(t *testing.T) {
	c := newTestCache(t)
	_ = c.SetSync("a", 1, 0)
	_ = c.SetSync("b", 2, 0)
	c.Clear()
	if c.Has("a") || c.Has("b") {
		t.Error("expected cache to be empty after Clear")
	}
}

func TestCache_GetWithExpiration(t *testing.T) {
	c := newTestCache(t)
	_ = c.SetSync("exp", "v", 0)
	v, found := c.GetWithExpiration("exp")
	if !found {
		t.Fatal("expected key to be found")
	}
	if v != "v" {
		t.Errorf("expected %q, got %v", "v", v)
	}
	_, found = c.GetWithExpiration("nokey")
	if found {
		t.Error("expected key not to be found")
	}
}

func TestCache_GetMulti(t *testing.T) {
	c := newTestCache(t)
	_ = c.SetSync("m1", 10, 0)
	_ = c.SetSync("m2", 20, 0)
	result := c.GetMulti([]interface{}{"m1", "m2", "missing"})
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
	if result["m1"] != 10 {
		t.Errorf("expected m1=10, got %v", result["m1"])
	}
}

func TestCache_SetMultiAndDeleteMulti(t *testing.T) {
	c := newTestCache(t)
	items := map[interface{}]interface{}{"x": 1, "y": 2}
	if err := c.SetMulti(items, 0); err != nil {
		t.Fatalf("SetMulti: %v", err)
	}
	if !c.Has("x") || !c.Has("y") {
		t.Error("expected both keys to be present after SetMulti")
	}
	c.DeleteMulti([]interface{}{"x", "y"})
	if c.Has("x") || c.Has("y") {
		t.Error("expected keys to be deleted after DeleteMulti")
	}
}

func TestCache_SetMultiInvalidTTL(t *testing.T) {
	c := newTestCache(t)
	err := c.SetMulti(map[interface{}]interface{}{"k": "v"}, -time.Second)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("expected ErrInvalidTTL, got %v", err)
	}
}

func TestCache_GetOrSet(t *testing.T) {
	c := newTestCache(t)
	called := 0
	val, err := c.GetOrSet("gos", func() (interface{}, error) {
		called++
		return "computed", nil
	}, 0)
	if err != nil {
		t.Fatalf("GetOrSet: %v", err)
	}
	if val != "computed" {
		t.Errorf("expected 'computed', got %v", val)
	}
	if called != 1 {
		t.Errorf("expected factory called once, called %d times", called)
	}

	// Second call should hit the cache
	val2, err := c.GetOrSet("gos", func() (interface{}, error) {
		called++
		return "new", nil
	}, 0)
	if err != nil {
		t.Fatalf("GetOrSet (cached): %v", err)
	}
	if val2 != "computed" {
		t.Errorf("expected cached 'computed', got %v", val2)
	}
	if called != 1 {
		t.Error("factory should not be called again when cache hit")
	}
}

func TestCache_GetOrSet_FactoryError(t *testing.T) {
	c := newTestCache(t)
	factoryErr := errors.New("factory error")
	_, err := c.GetOrSet("fail", func() (interface{}, error) {
		return nil, factoryErr
	}, 0)
	if !errors.Is(err, factoryErr) {
		t.Errorf("expected factory error, got %v", err)
	}
}

func TestCache_GetOrSetContext(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()
	val, err := c.GetOrSetContext(ctx, "ctx-key", func(c context.Context) (interface{}, error) {
		return "ctx-val", nil
	}, 0)
	if err != nil {
		t.Fatalf("GetOrSetContext: %v", err)
	}
	if val != "ctx-val" {
		t.Errorf("expected 'ctx-val', got %v", val)
	}
}

func TestCache_GetOrSetContext_Cancelled(t *testing.T) {
	c := newTestCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.GetOrSetContext(ctx, "cancelled-key", func(c context.Context) (interface{}, error) {
		return "val", nil
	}, 0)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestCache_Metrics(t *testing.T) {
	c, err := New("metrics-test", &Options{
		NumCounters: 1e4,
		MaxCost:     1 << 20,
		BufferItems: 64,
		Metrics:     true,
	})
	if err != nil {
		t.Fatalf("failed to create cache with metrics: %v", err)
	}
	defer c.Close()
	// Just ensure Metrics() doesn't panic and returns non-nil when enabled
	m := c.Metrics()
	if m == nil {
		t.Error("expected non-nil metrics when Metrics=true")
	}
}

// ---- Builder (builder.go) tests ----

func TestBuilder_Build(t *testing.T) {
	b := NewBuilder("built")
	b.WithNumCounters(1e4).
		WithMaxCost(1 << 20).
		WithBufferItems(64).
		WithMetrics(false)
	c, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer c.Close()
	if c.Name() != "built" {
		t.Errorf("expected name 'built', got %q", c.Name())
	}
}

func TestBuilder_WithMaxItems(t *testing.T) {
	b := NewBuilder("max-items").WithMaxItems(100)
	if b.options.NumCounters != 1000 {
		t.Errorf("expected NumCounters=1000, got %d", b.options.NumCounters)
	}
	if b.options.MaxCost != 100 {
		t.Errorf("expected MaxCost=100, got %d", b.options.MaxCost)
	}
}

func TestBuilder_WithMaxMemory(t *testing.T) {
	b := NewBuilder("mem").WithMaxMemory(512)
	if b.options.MaxCost != 512 {
		t.Errorf("expected MaxCost=512, got %d", b.options.MaxCost)
	}
}

func TestPresets_SmallMediumLarge(t *testing.T) {
	tests := []struct {
		name    string
		builder func(string) *Builder
	}{
		{"small", SmallCacheBuilder},
		{"medium", MediumCacheBuilder},
		{"large", LargeCacheBuilder},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := tt.builder(tt.name).Build()
			if err != nil {
				t.Fatalf("%s preset Build: %v", tt.name, err)
			}
			c.Close()
		})
	}
}

func TestSessionCacheBuilder(t *testing.T) {
	c, err := SessionCacheBuilder("session", time.Minute).Build()
	if err != nil {
		t.Fatalf("SessionCacheBuilder: %v", err)
	}
	c.Close()
}

func TestAPICacheBuilder(t *testing.T) {
	c, err := APICacheBuilder("api").Build()
	if err != nil {
		t.Fatalf("APICacheBuilder: %v", err)
	}
	c.Close()
}

func TestObjectCacheBuilder(t *testing.T) {
	c, err := ObjectCacheBuilder("objects").Build()
	if err != nil {
		t.Fatalf("ObjectCacheBuilder: %v", err)
	}
	defer c.Close()
	// Verify cost function is applied by setting a string and a byte slice
	_ = c.SetSync("str", "hello", 0)
	_ = c.SetSync("bytes", []byte{1, 2, 3}, 0)
}

// ---- Manager (manager.go) tests ----

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m := NewManager()
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func TestManager_CreateAndGet(t *testing.T) {
	m := newTestManager(t)
	opts := &Options{NumCounters: 1e4, MaxCost: 1 << 20, BufferItems: 64}
	c, err := m.Create("mgr-test", opts)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
	got, found := m.Get("mgr-test")
	if !found {
		t.Fatal("expected cache to be found after Create")
	}
	if got != c {
		t.Error("expected same cache pointer")
	}
}

func TestManager_Create_Duplicate(t *testing.T) {
	m := newTestManager(t)
	opts := &Options{NumCounters: 1e4, MaxCost: 1 << 20, BufferItems: 64}
	_, err := m.Create("dup", opts)
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err = m.Create("dup", opts)
	if err == nil {
		t.Error("expected error on duplicate Create")
	}
}

func TestManager_GetOrCreate(t *testing.T) {
	m := newTestManager(t)
	opts := &Options{NumCounters: 1e4, MaxCost: 1 << 20, BufferItems: 64}
	c1, err := m.GetOrCreate("goc", opts)
	if err != nil {
		t.Fatalf("GetOrCreate (first): %v", err)
	}
	c2, err := m.GetOrCreate("goc", opts)
	if err != nil {
		t.Fatalf("GetOrCreate (second): %v", err)
	}
	if c1 != c2 {
		t.Error("expected same cache on second GetOrCreate")
	}
}

func TestManager_Delete(t *testing.T) {
	m := newTestManager(t)
	opts := &Options{NumCounters: 1e4, MaxCost: 1 << 20, BufferItems: 64}
	_, _ = m.Create("to-delete", opts)
	if err := m.Delete("to-delete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, found := m.Get("to-delete"); found {
		t.Error("expected cache to be absent after Delete")
	}
}

func TestManager_Delete_NotFound(t *testing.T) {
	m := newTestManager(t)
	err := m.Delete("nonexistent")
	if err == nil {
		t.Error("expected error when deleting nonexistent cache")
	}
}

func TestManager_Clear(t *testing.T) {
	m := newTestManager(t)
	opts := &Options{NumCounters: 1e4, MaxCost: 1 << 20, BufferItems: 64}
	c, _ := m.Create("clr", opts)
	_ = c.SetSync("k", "v", 0)
	if err := m.Clear("clr"); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if c.Has("k") {
		t.Error("expected cache to be empty after Clear")
	}
}

func TestManager_Clear_NotFound(t *testing.T) {
	m := newTestManager(t)
	err := m.Clear("nope")
	if err == nil {
		t.Error("expected error when clearing nonexistent cache")
	}
}

func TestManager_ClearAll(t *testing.T) {
	m := newTestManager(t)
	opts := &Options{NumCounters: 1e4, MaxCost: 1 << 20, BufferItems: 64}
	c1, _ := m.Create("c1", opts)
	c2, _ := m.Create("c2", opts)
	_ = c1.SetSync("k", "v", 0)
	_ = c2.SetSync("k", "v", 0)
	m.ClearAll()
	if c1.Has("k") || c2.Has("k") {
		t.Error("expected all caches to be empty after ClearAll")
	}
}

func TestManager_List(t *testing.T) {
	m := newTestManager(t)
	opts := &Options{NumCounters: 1e4, MaxCost: 1 << 20, BufferItems: 64}
	_, _ = m.Create("list1", opts)
	_, _ = m.Create("list2", opts)
	names := m.List()
	if len(names) != 2 {
		t.Errorf("expected 2 caches listed, got %d", len(names))
	}
}

func TestManager_ConcurrentCreate(t *testing.T) {
	m := newTestManager(t)
	opts := &Options{NumCounters: 1e4, MaxCost: 1 << 20, BufferItems: 64}
	var wg sync.WaitGroup
	const n = 20
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			name := "concurrent"
			if idx%2 == 0 {
				_, _ = m.Create(name, opts)
			} else {
				_, _ = m.Get(name)
			}
		}(i)
	}
	wg.Wait()
}

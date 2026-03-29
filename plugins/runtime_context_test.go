package plugins

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
)

type countingCloser struct {
	closeCount atomic.Int32
}

func (c *countingCloser) Close() error {
	if c.closeCount.Add(1) > 1 {
		return net.ErrClosed
	}
	return nil
}

// TestWithPluginContextConcurrent verifies that using WithPluginContext across
// many goroutines to register shared/private resources is safe (no panic/race)
// and that resources are properly tracked in resourceInfo.
func TestWithPluginContextConcurrent(t *testing.T) {
	r := NewSimpleRuntime()
	const N = 200
	pluginCount := 5
	pluginNames := make([]string, pluginCount)
	for i := 0; i < pluginCount; i++ {
		pluginNames[i] = fmt.Sprintf("plugin-%d", i)
	}

	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			p := pluginNames[i%pluginCount]
			rt := r.WithPluginContext(p)

			// Register shared and private resources; errors should be nil
			if err := rt.RegisterSharedResource(fmt.Sprintf("s-%d", i), i); err != nil {
				// Fail fast if any unexpected error occurs
				t.Errorf("RegisterSharedResource failed: %v", err)
			}
			if err := rt.RegisterPrivateResource(fmt.Sprintf("pr-%d", i), i); err != nil {
				t.Errorf("RegisterPrivateResource failed: %v", err)
			}
		}(i)
	}
	wg.Wait()

	// Expect exactly N shared + N private resource info entries
	infos := r.ListResources()
	expected := 2 * N
	if len(infos) != expected {
		t.Fatalf("unexpected resource info count: got=%d, want=%d", len(infos), expected)
	}

	// Spot-check: ensure at least one private resource is marked and has plugin ID
	var foundPrivate bool
	for _, info := range infos {
		if info.IsPrivate {
			if info.PluginID == "" {
				t.Fatalf("private resource missing PluginID: %+v", info)
			}
			foundPrivate = true
			break
		}
	}
	if !foundPrivate {
		t.Fatalf("no private resource found in resource info list")
	}
}

func TestPrivateResourceNamespace_DoesNotCollideWithShared(t *testing.T) {
	base := NewUnifiedRuntime()
	pluginRuntime := base.WithPluginContext("plugin-a")

	if err := base.RegisterSharedResource("plugin-a:db", "shared"); err != nil {
		t.Fatalf("failed to register shared resource: %v", err)
	}
	if err := pluginRuntime.RegisterPrivateResource("db", "private"); err != nil {
		t.Fatalf("failed to register private resource: %v", err)
	}

	shared, err := base.GetSharedResource("plugin-a:db")
	if err != nil {
		t.Fatalf("failed to resolve shared resource: %v", err)
	}
	if shared.(string) != "shared" {
		t.Fatalf("unexpected shared resource value: %v", shared)
	}

	privateValue, err := pluginRuntime.GetPrivateResource("db")
	if err != nil {
		t.Fatalf("failed to resolve private resource: %v", err)
	}
	if privateValue.(string) != "private" {
		t.Fatalf("unexpected private resource value: %v", privateValue)
	}

	sharedInfo, err := base.GetResourceInfo("plugin-a:db")
	if err != nil {
		t.Fatalf("failed to get shared resource info: %v", err)
	}
	if sharedInfo.IsPrivate {
		t.Fatalf("expected shared resource info, got private: %+v", sharedInfo)
	}

	privateInfo, err := base.GetResourceInfo("private:plugin-a:db")
	if err != nil {
		t.Fatalf("failed to get private resource info: %v", err)
	}
	if !privateInfo.IsPrivate {
		t.Fatalf("expected private resource info, got shared: %+v", privateInfo)
	}
}

func TestPrivateResourceInfo_ResolvesLegacyDisplayNameWithoutBreakingSharedStorage(t *testing.T) {
	base := NewUnifiedRuntime()
	pluginRuntime := base.WithPluginContext("plugin-b")

	if err := pluginRuntime.RegisterPrivateResource("cache", 42); err != nil {
		t.Fatalf("failed to register private resource: %v", err)
	}

	info, err := base.GetResourceInfo("private:plugin-b:cache")
	if err != nil {
		t.Fatalf("expected private display-name lookup to work: %v", err)
	}
	if !info.IsPrivate {
		t.Fatalf("expected private resource info, got shared: %+v", info)
	}
	if info.PluginID != "plugin-b" {
		t.Fatalf("unexpected plugin id: %+v", info)
	}

	legacyInfo, err := base.GetResourceInfo("plugin-b:cache")
	if err != nil {
		t.Fatalf("expected legacy private display-name lookup to remain compatible: %v", err)
	}
	if !legacyInfo.IsPrivate {
		t.Fatalf("expected legacy private resource info, got shared: %+v", legacyInfo)
	}
}

func TestCleanupResources_DeduplicatesAliasedClosers(t *testing.T) {
	base := NewUnifiedRuntime()
	pluginRuntime := base.WithPluginContext("plugin-c")
	closer := &countingCloser{}

	if err := pluginRuntime.RegisterSharedResource("redis", closer); err != nil {
		t.Fatalf("failed to register shared alias: %v", err)
	}
	if err := pluginRuntime.RegisterSharedResource("redis.client", closer); err != nil {
		t.Fatalf("failed to register stable shared alias: %v", err)
	}
	if err := pluginRuntime.RegisterPrivateResource("client", closer); err != nil {
		t.Fatalf("failed to register private alias: %v", err)
	}

	if err := base.CleanupResources("plugin-c"); err != nil {
		t.Fatalf("cleanup should succeed for aliased resource: %v", err)
	}
	if got := closer.closeCount.Load(); got != 1 {
		t.Fatalf("expected aliased closer to be closed once, got %d", got)
	}
}

func TestCleanupResources_IgnoresAlreadyClosedErrors(t *testing.T) {
	base := NewUnifiedRuntime()
	pluginRuntime := base.WithPluginContext("plugin-d")
	closer := &countingCloser{}

	if err := pluginRuntime.RegisterSharedResource("redis", closer); err != nil {
		t.Fatalf("failed to register shared resource: %v", err)
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("failed to pre-close resource: %v", err)
	}

	if err := base.CleanupResources("plugin-d"); err != nil {
		t.Fatalf("cleanup should ignore already-closed resource: %v", err)
	}
}

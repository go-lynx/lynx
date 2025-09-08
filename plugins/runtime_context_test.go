package plugins

import (
	"fmt"
	"sync"
	"testing"
)

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

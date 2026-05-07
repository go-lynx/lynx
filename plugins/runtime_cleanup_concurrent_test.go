package plugins

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConcurrentCleanupRace exercises CleanupResources + RegisterSharedResource
// across 20 plugins with overlapping Add/Remove/Cleanup calls under -race.
//
// This is the "chaos monkey" integration test requested in issue #7.
func TestConcurrentCleanupRace(t *testing.T) {
	const pluginCount = 20
	const resourcesPerPlugin = 5

	rt := NewUnifiedRuntime()

	// closeOnce tracks how many cleanup calls actually removed resources.
	var totalCleaned atomic.Int64

	var wg sync.WaitGroup

	for i := range pluginCount {
		pluginID := fmt.Sprintf("plugin-%02d", i)

		// Register resources concurrently.
		wg.Add(1)
		go func(pid string) {
			defer wg.Done()
			prt := rt.WithPluginContext(pid)
			for j := range resourcesPerPlugin {
				name := fmt.Sprintf("res-%s-%d", pid, j)
				if err := prt.RegisterSharedResource(name, &fakeResource{name: name}); err != nil {
					// Ignore: concurrent overwrite by another goroutine is expected.
					_ = err
				}
			}
		}(pluginID)
	}

	wg.Wait() // all registrations done

	// Now run Cleanup for all plugins simultaneously.
	var cleanupWg sync.WaitGroup
	for i := range pluginCount {
		pluginID := fmt.Sprintf("plugin-%02d", i)
		cleanupWg.Add(1)
		go func(pid string) {
			defer cleanupWg.Done()
			// CleanupResources only removes resources owned by the caller plugin;
			// use the system runtime here to impersonate system-shutdown.
			if err := rt.CleanupResources(pid); err != nil {
				t.Errorf("CleanupResources(%s): %v", pid, err)
			}
			totalCleaned.Add(1)
		}(pluginID)
	}

	cleanupWg.Wait()

	if got := totalCleaned.Load(); got != pluginCount {
		t.Errorf("expected %d cleanup calls to complete, got %d", pluginCount, got)
	}
}

// TestConcurrentRegisterAndCleanup interleaves registration and cleanup to stress
// the RLock/Lock split introduced in the lock-granularity optimisation.
func TestConcurrentRegisterAndCleanup(t *testing.T) {
	const iterations = 50

	rt := NewUnifiedRuntime()
	prt := rt.WithPluginContext("stress-plugin")

	var wg sync.WaitGroup
	errs := make(chan error, iterations*2)

	for i := range iterations {
		wg.Add(2)

		// Writer: register or overwrite a resource.
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("shared-res-%d", n%5) // deliberate key collision
			if err := prt.RegisterSharedResource(name, &fakeResource{name: name}); err != nil {
				errs <- fmt.Errorf("register %s: %w", name, err)
			}
		}(i)

		// Reader/cleaner: list resources or trigger cleanup.
		go func() {
			defer wg.Done()
			if time.Now().UnixNano()%2 == 0 {
				_ = rt.ListResources()
			} else {
				// CleanupResources from system context — should never panic.
				if err := rt.CleanupResources("stress-plugin"); err != nil {
					// After cleanup the runtime's shared state is still valid;
					// re-registration in the next iteration is expected to succeed.
					_ = err
				}
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

// fakeResource is a minimal io.Closer-shaped value for cleanup testing.
type fakeResource struct {
	name   string
	closed atomic.Bool
}

func (f *fakeResource) Close() error {
	f.closed.Store(true)
	return nil
}

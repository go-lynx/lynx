package plugins

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestChaos_ConcurrentLifecycle hammers a single plugin's state machine from many
// goroutines. The real assertion is the race detector: statusMu must protect every
// status read/write, and no transition may panic. Run with -race.
func TestChaos_ConcurrentLifecycle(t *testing.T) {
	p := NewBasePlugin("chaos", "chaos", "d", "1.0.0", "p", 0)
	require.NoError(t, p.Initialize(p, NewSimpleRuntime()))

	const workers = 80
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			switch n % 6 {
			case 0:
				_ = p.Start(p)
			case 1:
				_ = p.Stop(p)
			case 2:
				_ = p.Suspend()
			case 3:
				_ = p.Resume()
			case 4:
				_ = p.Status(p)
			case 5:
				p.SetStatus(StatusActive)
			}
		}(i)
	}
	wg.Wait()

	// Whatever the interleaving, the plugin must land in a defined status.
	switch p.Status(p) {
	case StatusInactive, StatusInitializing, StatusActive, StatusSuspended,
		StatusStopping, StatusTerminated, StatusFailed:
	default:
		t.Fatalf("plugin ended in an undefined status: %v", p.Status(p))
	}
}

// TestChaos_CancellationStorm fires many StartContext calls with tiny, racing
// deadlines against context-aware plugins. Core invariants under cancellation:
// the plugin is NEVER left Active, and a context-aware plugin never leaks a goroutine.
func TestChaos_CancellationStorm(t *testing.T) {
	rt := NewSimpleRuntime()

	const workers = 60
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			p := &ctxAwareStartPlugin{
				TypedBasePlugin: NewTypedBasePlugin[any](fmt.Sprintf("c%d", n), "c", "d", "1.0.0", "p", 0, nil),
				entered:         make(chan struct{}),
			}
			require.NoError(t, p.Initialize(p, rt))

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(n%5+1)*time.Millisecond)
			defer cancel()

			err := p.StartContext(ctx, p)
			require.Error(t, err, "a cancelled start must report an error")
			require.NotEqual(t, StatusActive, p.Status(p), "a cancelled plugin must never be left Active")
			require.Zero(t, p.OrphanedStageCount(), "a context-aware plugin must never leak")
		}(i)
	}
	wg.Wait()
}

// TestChaos_LegacyOrphanAccountingUnderLoad drives many legacy (ctx-ignoring) tasks
// past their deadline concurrently, then releases them. The orphan counter must be
// consistent: it rises while tasks are stuck and returns to zero once they finish —
// never negative, never stuck above zero.
func TestChaos_LegacyOrphanAccountingUnderLoad(t *testing.T) {
	rt := NewSimpleRuntime()

	const workers = 30
	plugins := make([]*legacyBlockingStartPlugin, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		p := &legacyBlockingStartPlugin{
			TypedBasePlugin: NewTypedBasePlugin[any](fmt.Sprintf("L%d", i), "L", "d", "1.0.0", "p", 0, nil),
			entered:         make(chan struct{}),
			release:         make(chan struct{}),
		}
		plugins[i] = p
		require.NoError(t, p.Initialize(p, rt))

		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			err := p.StartContext(ctx, p)
			require.Error(t, err)
			require.NotEqual(t, StatusActive, p.Status(p))
		}()
	}
	wg.Wait()

	// All blocked tasks are now orphaned and counted.
	var totalOrphans int64
	for _, p := range plugins {
		<-p.entered
		totalOrphans += p.OrphanedStageCount()
	}
	require.Equal(t, int64(workers), totalOrphans, "every stuck legacy task must be counted exactly once")

	// Release them; every counter must settle back to zero.
	for _, p := range plugins {
		close(p.release)
	}
	require.Eventually(t, func() bool {
		for _, p := range plugins {
			if p.OrphanedStageCount() != 0 {
				return false
			}
		}
		return true
	}, 3*time.Second, 10*time.Millisecond, "orphan counters must drain to zero, never get stuck or go negative")

	// And none of them must have flipped to Active on late completion.
	for _, p := range plugins {
		require.NotEqual(t, StatusActive, p.Status(p))
	}
}

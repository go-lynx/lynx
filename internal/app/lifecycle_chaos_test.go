package app

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/go-lynx/lynx/plugins"
)

// TestManager_ConcurrentLifecycleChaos drives the manager's safe lifecycle calls
// concurrently with a mix of plugins: fast ones that succeed, slow ones that blow
// past their deadline (non-cancellable), context-aware ones that genuinely cancel,
// and ones that panic. It asserts the manager never deadlocks, never races (run
// with -race), and does not leak goroutines without bound after the slow tasks drain.
func TestManager_ConcurrentLifecycleChaos(t *testing.T) {
	manager := NewPluginManager[plugins.Plugin]()
	rt := manager.GetRuntime()

	runtime.GC()
	before := runtime.NumGoroutine()

	const workers = 50
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			switch n % 5 {
			case 0:
				// Non-cancellable slow start: times out, goroutine drains after 2s.
				_ = manager.safeStartPlugin(&SlowPlugin{startDuration: 2 * time.Second}, 40*time.Millisecond)
			case 1:
				// Fast start: succeeds well within the deadline.
				_ = manager.safeStartPlugin(&FastPlugin{}, time.Second)
			case 2:
				// Non-cancellable slow stop: times out.
				_ = manager.safeStopPlugin(&SlowPlugin{stopDuration: 2 * time.Second}, 40*time.Millisecond)
			case 3:
				// Context-aware slow init: genuinely cancels, returns promptly, no leak.
				_ = manager.safeInitPlugin(&ContextAwareSlowPlugin{initDuration: 2 * time.Second}, rt, 40*time.Millisecond)
			case 4:
				// Panic in a lifecycle method must be recovered, not crash the process.
				_ = manager.safeStartPlugin(&PanicPlugin{panicInStart: true}, time.Second)
			}
		}(i)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("manager lifecycle deadlocked under concurrent load")
	}

	// Let the abandoned (non-cancellable) SlowPlugin goroutines finish their 2s sleeps.
	time.Sleep(3 * time.Second)
	runtime.GC()
	after := runtime.NumGoroutine()
	if after > before+15 {
		t.Errorf("goroutine count grew unbounded under chaos: before=%d after=%d", before, after)
	}
}

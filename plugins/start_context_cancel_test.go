package plugins

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ctxAwareStartPlugin implements the context-aware startup hook, so its work
// genuinely observes cancellation.
type ctxAwareStartPlugin struct {
	*TypedBasePlugin[any]
	entered chan struct{}
}

func (p *ctxAwareStartPlugin) StartupTasksContext(ctx context.Context) error {
	close(p.entered)
	<-ctx.Done() // genuine cancellation: the work stops when ctx fires
	return ctx.Err()
}

// legacyBlockingStartPlugin only implements the non-context startup task and
// blocks until released, ignoring ctx entirely.
type legacyBlockingStartPlugin struct {
	*TypedBasePlugin[any]
	entered chan struct{}
	release chan struct{}
}

func (p *legacyBlockingStartPlugin) StartupTasks() error {
	close(p.entered)
	<-p.release // never observes ctx
	return nil
}

// TestStartContext_GenuineCancellation verifies that a plugin which observes ctx
// is actually cancelled: StartContext returns a context error and the plugin ends
// in StatusFailed rather than being promoted to StatusActive.
func TestStartContext_GenuineCancellation(t *testing.T) {
	rt := NewSimpleRuntime()
	p := &ctxAwareStartPlugin{
		TypedBasePlugin: NewTypedBasePlugin[any]("ctx-aware", "ctx-aware", "d", "1.0.0", "p", 0, nil),
		entered:         make(chan struct{}),
	}
	require.NoError(t, p.Initialize(p, rt))

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- p.StartContext(ctx, p) }()

	<-p.entered // ensure the work has actually started
	cancel()

	select {
	case err := <-errCh:
		require.Error(t, err)
		require.True(t, isLifecycleContextErr(err), "expected a context error, got: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("StartContext did not return after cancellation — work was not genuinely cancellable")
	}

	require.Equal(t, StatusFailed, p.Status(p), "plugin must not be Active after cancellation")
	require.Zero(t, p.OrphanedStageCount(), "context-aware task must not leak")
}

// TestStartContext_LegacyAbandonedSafely verifies that a legacy task which ignores
// ctx is abandoned safely on timeout: StartContext returns a deadline error, the
// plugin is StatusFailed (never flipped to Active by the still-running goroutine),
// the orphan is counted, and the count returns to zero once the task finishes.
func TestStartContext_LegacyAbandonedSafely(t *testing.T) {
	rt := NewSimpleRuntime()
	p := &legacyBlockingStartPlugin{
		TypedBasePlugin: NewTypedBasePlugin[any]("legacy", "legacy", "d", "1.0.0", "p", 0, nil),
		entered:         make(chan struct{}),
		release:         make(chan struct{}),
	}
	require.NoError(t, p.Initialize(p, rt))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := p.StartContext(ctx, p)
	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded), "expected deadline exceeded, got: %v", err)

	<-p.entered // the legacy task really started
	require.Equal(t, StatusFailed, p.Status(p), "abandoned plugin must be Failed, not Active")
	require.Equal(t, int64(1), p.OrphanedStageCount(), "the still-running legacy task must be counted as orphaned")

	// Release the legacy task; the orphan watcher should drain and decrement.
	close(p.release)
	require.Eventually(t, func() bool { return p.OrphanedStageCount() == 0 }, 2*time.Second, 5*time.Millisecond,
		"orphan count should return to zero once the abandoned task finishes")

	// Critical: the late-completing task must NOT have promoted the plugin to Active.
	require.Equal(t, StatusFailed, p.Status(p), "late legacy completion must not corrupt status to Active")
}

// TestRouting_ContextStepsImplyTrueContextLifecycle verifies that implementing a
// context-aware step hook is, by itself, enough for the framework to route the
// plugin through its context-aware entrypoints — no explicit IsContextAware() or
// PluginProtocol().ContextLifecycle opt-in required.
func TestRouting_ContextStepsImplyTrueContextLifecycle(t *testing.T) {
	p := &ctxAwareStartPlugin{
		TypedBasePlugin: NewTypedBasePlugin[any]("ctx-route", "ctx-route", "d", "1.0.0", "p", 0, nil),
		entered:         make(chan struct{}),
	}

	caps := DescribePluginCapabilities(p)
	require.True(t, caps.HasLifecycleWithCtx, "embedding the base provides the *Context entrypoints")
	require.True(t, caps.HasContextSteps, "plugin implements StartupTasksContext")
	require.False(t, caps.IsTrulyContextAware, "plugin does not override IsContextAware()")
	require.False(t, caps.Protocol.ContextLifecycle, "plugin does not declare the legacy protocol")

	require.True(t, HasTrueContextLifecycle(p), "a context step hook alone must enable the context-aware path")
	lc, ok := GetTrueContextLifecycle(p)
	require.True(t, ok)
	require.NotNil(t, lc)
}

// TestRouting_LegacyPluginNotContextLifecycle verifies that a plugin with only
// legacy (non-context) steps is NOT routed through the context path, so it keeps
// the manager's own timeout/abandon machinery rather than double-wrapping.
func TestRouting_LegacyPluginNotContextLifecycle(t *testing.T) {
	p := &legacyBlockingStartPlugin{
		TypedBasePlugin: NewTypedBasePlugin[any]("legacy-route", "legacy-route", "d", "1.0.0", "p", 0, nil),
		entered:         make(chan struct{}),
		release:         make(chan struct{}),
	}

	require.False(t, HasTrueContextLifecycle(p), "a plugin with no context hooks must not be routed through the context path")
	_, ok := GetTrueContextLifecycle(p)
	require.False(t, ok)
}

package plugins

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func newBase(id string) *BasePlugin {
	return NewBasePlugin(id, id, "desc", "1.2.3", "prefix", 7)
}

// failStartup overrides StartupTasks to fail.
type failStartup struct {
	*TypedBasePlugin[any]
}

func (failStartup) StartupTasks() error { return errors.New("boom startup") }

// failHealth overrides CheckHealth to fail.
type failHealth struct {
	*TypedBasePlugin[any]
}

func (failHealth) CheckHealth() error { return errors.New("unhealthy") }

// failCleanup overrides CleanupTasks to fail.
type failCleanup struct {
	*TypedBasePlugin[any]
}

func (failCleanup) CleanupTasks() error { return errors.New("boom cleanup") }

func TestBase_HappyLifecycle(t *testing.T) {
	rt := NewSimpleRuntime()
	p := newBase("happy")
	require.Equal(t, StatusInactive, p.Status(p))

	require.NoError(t, p.Initialize(p, rt))
	require.Equal(t, StatusInactive, p.Status(p), "Initialize ends Inactive, ready to start")

	require.NoError(t, p.Start(p))
	require.Equal(t, StatusActive, p.Status(p))

	require.NoError(t, p.Stop(p))
	require.Equal(t, StatusTerminated, p.Status(p))
}

func TestBase_StartBeforeInitialize(t *testing.T) {
	p := newBase("uninit")
	err := p.Start(p)
	require.ErrorIs(t, err, ErrPluginNotInitialized, "starting without a runtime must be rejected")
}

func TestBase_DoubleStart(t *testing.T) {
	p := newBase("double")
	require.NoError(t, p.Initialize(p, NewSimpleRuntime()))
	require.NoError(t, p.Start(p))
	err := p.Start(p)
	require.ErrorIs(t, err, ErrPluginAlreadyActive)
}

func TestBase_StopWhenNotActive(t *testing.T) {
	p := newBase("inactive")
	require.NoError(t, p.Initialize(p, NewSimpleRuntime()))
	err := p.Stop(p) // never started
	require.Error(t, err, "stopping a non-active plugin must fail")
	require.NotEqual(t, StatusTerminated, p.Status(p))
}

func TestBase_StartupFailureMovesToFailed(t *testing.T) {
	p := &failStartup{NewTypedBasePlugin[any]("fs", "fs", "d", "1.0.0", "p", 0, nil)}
	require.NoError(t, p.Initialize(p, NewSimpleRuntime()))
	err := p.Start(p)
	require.Error(t, err)
	require.Equal(t, StatusFailed, p.Status(p), "a failed startup must leave the plugin Failed, not Active")
}

func TestBase_HealthFailureMovesToFailed(t *testing.T) {
	p := &failHealth{NewTypedBasePlugin[any]("fh", "fh", "d", "1.0.0", "p", 0, nil)}
	require.NoError(t, p.Initialize(p, NewSimpleRuntime()))
	err := p.Start(p)
	require.Error(t, err)
	require.Equal(t, StatusFailed, p.Status(p), "a failed health check must leave the plugin Failed")
}

func TestBase_CleanupFailureMovesToFailed(t *testing.T) {
	p := &failCleanup{NewTypedBasePlugin[any]("fc", "fc", "d", "1.0.0", "p", 0, nil)}
	require.NoError(t, p.Initialize(p, NewSimpleRuntime()))
	require.NoError(t, p.Start(p))
	err := p.Stop(p)
	require.Error(t, err)
	require.Equal(t, StatusFailed, p.Status(p))
}

func TestBase_SuspendResume(t *testing.T) {
	p := newBase("sr")
	require.NoError(t, p.Initialize(p, NewSimpleRuntime()))

	// Suspend requires Active.
	require.Error(t, p.Suspend(), "cannot suspend a plugin that is not active")

	require.NoError(t, p.Start(p))
	require.NoError(t, p.Suspend())
	require.Equal(t, StatusSuspended, p.Status(p))

	// Resume requires Suspended; double resume must fail.
	require.NoError(t, p.Resume())
	require.Equal(t, StatusActive, p.Status(p))
	require.Error(t, p.Resume(), "cannot resume a plugin that is not suspended")
}

func TestBase_MetadataAccessors(t *testing.T) {
	p := newBase("meta")
	require.Equal(t, "meta", p.ID())
	require.Equal(t, "meta", p.Name())
	require.Equal(t, "desc", p.Description())
	require.Equal(t, "1.2.3", p.Version())
	require.Equal(t, 7, p.Weight())

	p.SetStatus(StatusSuspended)
	require.Equal(t, StatusSuspended, p.Status(p))
}

func TestBase_DependencyAccessors(t *testing.T) {
	p := newBase("deps")
	require.Nil(t, p.GetDependencies())
	p.AddDependency(Dependency{ID: "x", Type: DependencyTypeRequired})
	got := p.GetDependencies()
	require.Len(t, got, 1)
	// mutating the returned copy must not affect the plugin
	got[0].ID = "mutated"
	require.Equal(t, "x", p.GetDependencies()[0].ID)
}

func TestBase_EventFilters(t *testing.T) {
	p := newBase("filt")
	// No filters => everything emits.
	require.True(t, p.ShouldEmitEvent(PluginEvent{Type: EventPluginStarted}))

	p.AddEventFilter(EventFilter{Types: []EventType{EventPluginStarted}})
	require.True(t, p.EventMatchesFilter(PluginEvent{Type: EventPluginStarted}, p.eventFilters[0]))
	require.False(t, p.EventMatchesFilter(PluginEvent{Type: EventPluginStopped}, p.eventFilters[0]))
	require.True(t, p.ShouldEmitEvent(PluginEvent{Type: EventPluginStarted}))
	require.False(t, p.ShouldEmitEvent(PluginEvent{Type: EventPluginStopped}), "non-matching event must be filtered out")

	// Removing the filter restores pass-through.
	p.RemoveEventFilter(0)
	require.True(t, p.ShouldEmitEvent(PluginEvent{Type: EventPluginStopped}))
	// Out-of-range remove is a no-op (must not panic).
	p.RemoveEventFilter(99)
}

func TestBase_GetHealthReport(t *testing.T) {
	p := newBase("health")
	require.NoError(t, p.Initialize(p, NewSimpleRuntime()))
	report := p.GetHealth()
	require.NotEmpty(t, report.Status)
	require.NotZero(t, report.Timestamp)
}

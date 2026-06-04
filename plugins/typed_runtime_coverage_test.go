package plugins

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTypedRuntime_SharedResources exercises the TypedRuntimeImpl delegation over
// the real UnifiedRuntime for shared resource registration, lookup, info and stats.
func TestTypedRuntime_SharedResources(t *testing.T) {
	rt := NewTypedRuntime()

	require.NoError(t, rt.RegisterResource("db", 123))
	v, err := rt.GetResource("db")
	require.NoError(t, err)
	require.Equal(t, 123, v)

	require.NoError(t, rt.RegisterSharedResource("cache", "hot"))
	v, err = rt.GetSharedResource("cache")
	require.NoError(t, err)
	require.Equal(t, "hot", v)

	_, err = rt.GetResource("missing")
	require.Error(t, err)
	_, err = rt.GetResource("")
	require.Error(t, err)

	info, err := rt.GetResourceInfo("db")
	require.NoError(t, err)
	require.Equal(t, "db", info.Name)

	require.NotEmpty(t, rt.ListResources())
	require.NotNil(t, rt.GetResourceStats())
}

// TestTypedRuntime_PrivateResourceNeedsContext verifies private resources require a
// plugin context, and that a context-scoped runtime can register them.
func TestTypedRuntime_PrivateResourceNeedsContext(t *testing.T) {
	rt := NewTypedRuntime()

	// No plugin context on the bare runtime.
	require.Equal(t, "", rt.GetCurrentPluginContext())
	require.Error(t, rt.RegisterPrivateResource("secret", 1), "private register needs a plugin context")
	_, err := rt.GetPrivateResource("secret")
	require.Error(t, err)

	// A context-scoped runtime can register a private resource.
	scoped := rt.WithPluginContext("plug-a")
	require.NotNil(t, scoped)
	require.NoError(t, scoped.RegisterPrivateResource("secret", 42))
	got, err := scoped.GetPrivateResource("secret")
	require.NoError(t, err)
	require.Equal(t, 42, got)
}

func TestTypedRuntime_TypedHelpersAndHandle(t *testing.T) {
	rt := NewTypedRuntime()

	require.NoError(t, RegisterTypedResource[int](rt, "n", 7))
	n, err := GetTypedResource[int](rt, "n")
	require.NoError(t, err)
	require.Equal(t, 7, n)

	// Wrong type assertion is a clean error, not a panic.
	_, err = GetTypedResource[string](rt, "n")
	require.Error(t, err)

	h := NewResourceHandle[int](rt, "n")
	require.Equal(t, "n", h.Name())
	hv, err := h.Get()
	require.NoError(t, err)
	require.Equal(t, 7, hv)
	hi, err := h.Info()
	require.NoError(t, err)
	require.Equal(t, "n", hi.Name)

	// A handle with a nil manager reports an error from Info rather than panicking.
	var bad ResourceHandle[int]
	_, err = bad.Info()
	require.Error(t, err)
}

func TestTypedRuntime_EventsAndListeners(t *testing.T) {
	rt := NewTypedRuntime()
	l := &legacyRuntimeListener{id: "L1"}

	// Add/remove and plugin-scoped listener registration must not panic.
	rt.AddListener(l, nil)
	rt.AddPluginListener("plug", l, nil)
	rt.EmitEvent(PluginEvent{Type: EventPluginStarted})
	rt.EmitPluginEvent("plug", "custom", map[string]any{"k": 1})
	rt.RemoveListener(l)

	// History queries return slices (possibly empty) without error.
	require.NotPanics(t, func() {
		_ = rt.GetEventHistory(EventFilter{})
		_ = rt.GetPluginEventHistory("plug", EventFilter{})
	})
}

func TestTypedRuntime_LoggerCleanupShutdown(t *testing.T) {
	rt := NewTypedRuntime()
	require.NotNil(t, rt.GetLogger())

	require.NoError(t, rt.RegisterResource("temp", "v"))
	// Cleanup of the system owner must not error.
	require.NoError(t, rt.CleanupResources("system"))

	// Shutdown is idempotent enough to call without panic.
	require.NotPanics(t, func() { rt.Shutdown() })
}

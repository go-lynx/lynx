package plugins

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// depPlugin is a minimal Plugin with a configurable id and version, used to drive
// the dependency graph and version-conflict logic.
type depPlugin struct {
	*TypedBasePlugin[any]
}

func newVerPlugin(id, version string) *depPlugin {
	return &depPlugin{NewTypedBasePlugin[any](id, id, "d", version, "p", 0, nil)}
}

// compile-time check that the base plugin satisfies the managed Plugin contract.
var _ Plugin = (*depPlugin)(nil)

func indexOf(order []string, id string) int {
	for i, v := range order {
		if v == id {
			return i
		}
	}
	return -1
}

func TestDepGraph_AddRemoveQuery(t *testing.T) {
	g := NewDependencyGraph()
	a := newVerPlugin("a", "1.0.0")
	b := newVerPlugin("b", "1.0.0")
	g.AddPlugin(a)
	g.AddPlugin(b)

	require.True(t, g.HasPlugin("a"))
	require.False(t, g.HasPlugin("missing"))
	require.Len(t, g.GetAllPlugins(), 2)

	// AddDependency error paths.
	require.Error(t, g.AddDependency("missing", &Dependency{ID: "b", Type: DependencyTypeRequired}),
		"adding a dependency for an unknown plugin must fail")
	require.Error(t, g.AddDependency("a", &Dependency{ID: "missing", Type: DependencyTypeRequired}),
		"depending on an unknown plugin must fail")

	require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Name: "b", Type: DependencyTypeRequired, Required: true}))

	// GetDependencies returns a copy — mutating it must not affect the graph.
	deps := g.GetDependencies("a")
	require.Len(t, deps, 1)
	deps[0] = nil
	require.NotNil(t, g.GetDependencies("a")[0], "GetDependencies must return a defensive copy")

	require.Equal(t, []string{"a"}, g.GetDependents("b"))

	// RemoveDependency: unknown plugin and unknown dep.
	require.Error(t, g.RemoveDependency("nope", "b"))
	require.NoError(t, g.RemoveDependency("a", "b"))
	require.Empty(t, g.GetDependencies("a"))
}

func TestDepGraph_RemovePluginCleansDependents(t *testing.T) {
	g := NewDependencyGraph()
	g.AddPlugin(newVerPlugin("a", "1.0.0"))
	g.AddPlugin(newVerPlugin("b", "1.0.0"))
	require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired}))
	require.Equal(t, []string{"a"}, g.GetDependents("b"))

	// Removing the dependent must scrub it out of b's dependents and the reverse index.
	g.RemovePlugin("a")
	require.False(t, g.HasPlugin("a"))
	require.Nil(t, g.GetDependents("b"), "b should have no dependents after a is removed")
	require.Empty(t, g.GetDependencies("a"))
}

func TestDepGraph_CircularDependencies(t *testing.T) {
	t.Run("no cycle", func(t *testing.T) {
		g := NewDependencyGraph()
		g.AddPlugin(newVerPlugin("a", "1.0.0"))
		g.AddPlugin(newVerPlugin("b", "1.0.0"))
		require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired}))
		_, err := g.CheckCircularDependencies()
		require.NoError(t, err)
	})

	t.Run("self cycle", func(t *testing.T) {
		g := NewDependencyGraph()
		g.AddPlugin(newVerPlugin("a", "1.0.0"))
		require.NoError(t, g.AddDependency("a", &Dependency{ID: "a", Type: DependencyTypeRequired}))
		_, err := g.CheckCircularDependencies()
		require.Error(t, err, "a self-dependency is a cycle")
	})

	t.Run("two-node cycle", func(t *testing.T) {
		g := NewDependencyGraph()
		g.AddPlugin(newVerPlugin("a", "1.0.0"))
		g.AddPlugin(newVerPlugin("b", "1.0.0"))
		require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired}))
		require.NoError(t, g.AddDependency("b", &Dependency{ID: "a", Type: DependencyTypeRequired}))
		cycle, err := g.CheckCircularDependencies()
		require.Error(t, err)
		require.NotEmpty(t, cycle)
	})

	t.Run("optional dependency is not a cycle", func(t *testing.T) {
		g := NewDependencyGraph()
		g.AddPlugin(newVerPlugin("a", "1.0.0"))
		g.AddPlugin(newVerPlugin("b", "1.0.0"))
		require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired}))
		require.NoError(t, g.AddDependency("b", &Dependency{ID: "a", Type: DependencyTypeOptional}))
		_, err := g.CheckCircularDependencies()
		require.NoError(t, err, "only required dependencies form cycles")
	})
}

func TestDepGraph_ResolveOrder(t *testing.T) {
	t.Run("linear dependency comes before dependent", func(t *testing.T) {
		g := NewDependencyGraph()
		for _, id := range []string{"a", "b", "c"} {
			g.AddPlugin(newVerPlugin(id, "1.0.0"))
		}
		// a requires b, b requires c  =>  c before b before a
		require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired}))
		require.NoError(t, g.AddDependency("b", &Dependency{ID: "c", Type: DependencyTypeRequired}))

		order, err := g.ResolveDependencies()
		require.NoError(t, err)
		require.Len(t, order, 3)
		require.Less(t, indexOf(order, "c"), indexOf(order, "b"))
		require.Less(t, indexOf(order, "b"), indexOf(order, "a"))
	})

	t.Run("diamond", func(t *testing.T) {
		g := NewDependencyGraph()
		for _, id := range []string{"top", "left", "right", "base"} {
			g.AddPlugin(newVerPlugin(id, "1.0.0"))
		}
		require.NoError(t, g.AddDependency("top", &Dependency{ID: "left", Type: DependencyTypeRequired}))
		require.NoError(t, g.AddDependency("top", &Dependency{ID: "right", Type: DependencyTypeRequired}))
		require.NoError(t, g.AddDependency("left", &Dependency{ID: "base", Type: DependencyTypeRequired}))
		require.NoError(t, g.AddDependency("right", &Dependency{ID: "base", Type: DependencyTypeRequired}))

		order, err := g.ResolveDependencies()
		require.NoError(t, err)
		require.Less(t, indexOf(order, "base"), indexOf(order, "left"))
		require.Less(t, indexOf(order, "base"), indexOf(order, "right"))
		require.Less(t, indexOf(order, "left"), indexOf(order, "top"))
		require.Less(t, indexOf(order, "right"), indexOf(order, "top"))
	})

	t.Run("cycle fails to resolve", func(t *testing.T) {
		g := NewDependencyGraph()
		g.AddPlugin(newVerPlugin("a", "1.0.0"))
		g.AddPlugin(newVerPlugin("b", "1.0.0"))
		require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired}))
		require.NoError(t, g.AddDependency("b", &Dependency{ID: "a", Type: DependencyTypeRequired}))
		_, err := g.ResolveDependencies()
		require.Error(t, err)
	})
}

func TestDepGraph_VersionConflicts(t *testing.T) {
	build := func(depVersion string, c *VersionConstraint) ([]VersionConflict, error) {
		g := NewDependencyGraph()
		g.AddPlugin(newVerPlugin("a", "1.0.0"))
		g.AddPlugin(newVerPlugin("b", depVersion))
		require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired, VersionConstraint: c}))
		return g.CheckVersionConflicts()
	}

	t.Run("exact mismatch", func(t *testing.T) {
		conflicts, err := build("1.0.0", &VersionConstraint{ExactVersion: "2.0.0"})
		require.NoError(t, err)
		require.Len(t, conflicts, 1)
		require.Equal(t, "exact_version_mismatch", conflicts[0].ConflictType)
	})
	t.Run("too low", func(t *testing.T) {
		conflicts, _ := build("1.0.0", &VersionConstraint{MinVersion: "2.0.0"})
		require.Len(t, conflicts, 1)
		require.Equal(t, "version_too_low", conflicts[0].ConflictType)
	})
	t.Run("too high", func(t *testing.T) {
		conflicts, _ := build("3.0.0", &VersionConstraint{MaxVersion: "2.0.0"})
		require.Len(t, conflicts, 1)
		require.Equal(t, "version_too_high", conflicts[0].ConflictType)
	})
	t.Run("excluded", func(t *testing.T) {
		conflicts, _ := build("1.5.0", &VersionConstraint{ExcludeVersions: []string{"1.5.0"}})
		require.Len(t, conflicts, 1)
		require.Equal(t, "excluded_version", conflicts[0].ConflictType)
	})
	t.Run("satisfied", func(t *testing.T) {
		conflicts, _ := build("1.5.0", &VersionConstraint{MinVersion: "1.0.0", MaxVersion: "2.0.0"})
		require.Empty(t, conflicts)
	})
}

func TestNormalizeSemver(t *testing.T) {
	require.Equal(t, "1.0.0", normalizeSemver("v1"))
	require.Equal(t, "1.0.0", normalizeSemver("V1"))
	require.Equal(t, "1.2.0", normalizeSemver("1.2"))
	require.Equal(t, "1.2.3", normalizeSemver("1.2.3"))
	require.Equal(t, "", normalizeSemver("  "))
}

func TestCompareVersionNumeric(t *testing.T) {
	c, ok := compareVersionNumeric("1.2.0", "1.2.0")
	require.True(t, ok)
	require.Equal(t, 0, c)

	c, ok = compareVersionNumeric("1.2.0", "1.3.0")
	require.True(t, ok)
	require.Negative(t, c)

	// numeric, not lexical: 10 > 9
	c, ok = compareVersionNumeric("1.10.0", "1.9.0")
	require.True(t, ok)
	require.Positive(t, c)

	// shorter is treated as less
	c, ok = compareVersionNumeric("1.2", "1.2.1")
	require.True(t, ok)
	require.Negative(t, c)

	_, ok = compareVersionNumeric("abc", "1.0.0")
	require.False(t, ok, "non-numeric parts must report not-comparable")
}

func TestDepGraph_ValidateDependencies(t *testing.T) {
	// AddDependency requires the dependency plugin to exist in the graph; the
	// "missing" case in ValidateDependencies is relative to the map passed in.
	t.Run("missing dependency", func(t *testing.T) {
		g := NewDependencyGraph()
		g.AddPlugin(newVerPlugin("a", "1.0.0"))
		g.AddPlugin(newVerPlugin("b", "1.0.0"))
		require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired}))

		// b is absent from the snapshot handed to ValidateDependencies.
		errs, err := g.ValidateDependencies(map[string]Plugin{"a": newVerPlugin("a", "1.0.0")})
		require.NoError(t, err)
		require.Len(t, errs, 1)
		require.Equal(t, "missing_dependency", errs[0].ErrorType)
	})

	t.Run("version conflict", func(t *testing.T) {
		g := NewDependencyGraph()
		g.AddPlugin(newVerPlugin("a", "1.0.0"))
		g.AddPlugin(newVerPlugin("b", "1.0.0"))
		require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired,
			VersionConstraint: &VersionConstraint{MinVersion: "2.0.0"}}))

		errs, err := g.ValidateDependencies(g.GetAllPlugins())
		require.NoError(t, err)
		require.Len(t, errs, 1)
		require.Equal(t, "version_conflict", errs[0].ErrorType)
	})
}

type failChecker struct{}

func (failChecker) Check(Plugin) bool   { return false }
func (failChecker) Description() string { return "always fails" }

func TestDepGraph_ValidateChecker(t *testing.T) {
	g := NewDependencyGraph()
	g.AddPlugin(newVerPlugin("a", "1.0.0"))
	g.AddPlugin(newVerPlugin("b", "1.0.0"))
	require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired, Checker: failChecker{}}))

	errs, err := g.ValidateDependencies(g.GetAllPlugins())
	require.NoError(t, err)
	require.Len(t, errs, 1)
	require.Equal(t, "dependency_check_failed", errs[0].ErrorType)
}

func TestDepGraph_TreeStatsCleanup(t *testing.T) {
	g := NewDependencyGraph()
	g.AddPlugin(newVerPlugin("a", "1.0.0"))
	g.AddPlugin(newVerPlugin("b", "1.0.0"))
	require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired}))
	require.NoError(t, g.AddDependency("b", &Dependency{ID: "a", Type: DependencyTypeRequired})) // cycle for tree marker

	tree := g.GetDependencyTree("a")
	require.Equal(t, "a", tree["id"])

	stats := g.GetDependencyStats()
	require.Equal(t, 2, stats["total_plugins"])
	require.Equal(t, 2, stats["required_deps"])

	// Orphan a dependency by dropping a plugin behind the graph's back, then clean up.
	g.AddPlugin(newVerPlugin("c", "1.0.0"))
	require.NoError(t, g.AddDependency("a", &Dependency{ID: "c", Type: DependencyTypeRequired}))
	delete(g.plugins, "c") // simulate a plugin vanishing without RemovePlugin
	cleaned := g.CleanupOrphanedDependencies()
	require.GreaterOrEqual(t, cleaned, 1, "the dangling dependency on c must be cleaned")
}

func TestDepGraph_ConcurrentReadWrite(t *testing.T) {
	g := NewDependencyGraph()
	for i := 0; i < 20; i++ {
		g.AddPlugin(newVerPlugin(string(rune('a'+i)), "1.0.0"))
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := string(rune('a' + n%20))
			_ = g.GetDependencies(id)
			_ = g.GetDependents(id)
			_, _ = g.CheckCircularDependencies()
			_ = g.GetDependencyStats()
		}(i)
	}
	wg.Wait()
}

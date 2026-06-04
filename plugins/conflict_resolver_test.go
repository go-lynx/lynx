package plugins

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func hasConflictType(conflicts []DependencyConflict, ct ConflictType) bool {
	for _, c := range conflicts {
		if c.Type == ct {
			return true
		}
	}
	return false
}

func TestConflictResolver_DetectCircular(t *testing.T) {
	g := NewDependencyGraph()
	g.AddPlugin(newVerPlugin("a", "1.0.0"))
	g.AddPlugin(newVerPlugin("b", "1.0.0"))
	require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired}))
	require.NoError(t, g.AddDependency("b", &Dependency{ID: "a", Type: DependencyTypeRequired}))

	cr := NewConflictResolver(nil)
	conflicts, err := cr.DetectConflicts(g)
	require.NoError(t, err)
	require.True(t, hasConflictType(conflicts, ConflictTypeCircular), "a cycle must be detected")
}

func TestConflictResolver_DetectVersion(t *testing.T) {
	g := NewDependencyGraph()
	g.AddPlugin(newVerPlugin("a", "1.0.0"))
	g.AddPlugin(newVerPlugin("b", "1.0.0"))
	require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired,
		VersionConstraint: &VersionConstraint{MinVersion: "2.0.0"}}))

	cr := NewConflictResolver(nil)
	conflicts, err := cr.DetectConflicts(g)
	require.NoError(t, err)
	require.True(t, hasConflictType(conflicts, ConflictTypeVersion))
}

func TestConflictResolver_DetectMissing(t *testing.T) {
	g := NewDependencyGraph()
	g.AddPlugin(newVerPlugin("a", "1.0.0"))
	g.AddPlugin(newVerPlugin("b", "1.0.0"))
	require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired}))
	// Drop b so a's required dependency dangles.
	g.RemovePlugin("b")

	cr := NewConflictResolver(nil)
	conflicts, err := cr.DetectConflicts(g)
	require.NoError(t, err)
	require.True(t, hasConflictType(conflicts, ConflictTypeMissing))
}

func TestConflictResolver_ResolveEmpty(t *testing.T) {
	cr := NewConflictResolver(nil)
	res, err := cr.ResolveConflicts(nil)
	require.NoError(t, err)
	require.Equal(t, "No conflicts to resolve", res.Summary)
	require.Empty(t, res.Actions)
}

func TestConflictResolver_ResolveWithSolutions(t *testing.T) {
	g := NewDependencyGraph()
	g.AddPlugin(newVerPlugin("a", "1.0.0"))
	g.AddPlugin(newVerPlugin("b", "1.0.0"))
	require.NoError(t, g.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired}))
	require.NoError(t, g.AddDependency("b", &Dependency{ID: "a", Type: DependencyTypeRequired}))

	cr := NewConflictResolver(nil)
	conflicts, err := cr.DetectConflicts(g)
	require.NoError(t, err)
	require.NotEmpty(t, conflicts)

	res, err := cr.ResolveConflicts(conflicts)
	require.NoError(t, err)
	// The circular conflict ships with generated solutions, so it must be resolved.
	require.Contains(t, res.ResolvedConflicts, "circular_dependency")
	require.NotEmpty(t, res.Actions)
	require.NotEmpty(t, res.Summary)
}

func TestConflictResolver_ResolveNoSolutionRemains(t *testing.T) {
	cr := NewConflictResolver(nil).(*DefaultConflictResolver)
	res, err := cr.ResolveConflicts([]DependencyConflict{
		{ID: "unsolvable", Type: ConflictTypeIncompatible, Severity: ConflictSeverityHigh, Solutions: nil},
	})
	require.NoError(t, err)
	require.Contains(t, res.RemainingConflicts, "unsolvable")
	require.Empty(t, res.ResolvedConflicts)
}

func TestConflictResolver_SelectBestSolutionByPriority(t *testing.T) {
	cr := NewConflictResolver(nil).(*DefaultConflictResolver)
	conflict := DependencyConflict{
		Solutions: []ConflictSolution{
			{ID: "low-prio", Priority: 5, Risk: SolutionRiskLow},
			{ID: "high-prio", Priority: 1, Risk: SolutionRiskHigh},
		},
	}
	best := cr.selectBestSolution(conflict)
	require.NotNil(t, best)
	require.Equal(t, "high-prio", best.ID, "lower Priority value wins")

	require.Nil(t, cr.selectBestSolution(DependencyConflict{Solutions: nil}))
}

func TestConflictResolver_Weights(t *testing.T) {
	cr := NewConflictResolver(nil).(*DefaultConflictResolver)
	require.Greater(t, cr.getSeverityWeight(ConflictSeverityCritical), cr.getSeverityWeight(ConflictSeverityHigh))
	require.Greater(t, cr.getSeverityWeight(ConflictSeverityHigh), cr.getSeverityWeight(ConflictSeverityLow))
	require.Equal(t, 0, cr.getSeverityWeight(ConflictSeverity("nonsense")))

	require.Greater(t, cr.getRiskWeight(SolutionRiskHigh), cr.getRiskWeight(SolutionRiskLow))
	require.Equal(t, 0, cr.getRiskWeight(SolutionRisk("nonsense")))
}

func TestConflictResolver_ValidateResolution(t *testing.T) {
	cr := NewConflictResolver(nil)

	clean := NewDependencyGraph()
	clean.AddPlugin(newVerPlugin("a", "1.0.0"))
	clean.AddPlugin(newVerPlugin("b", "1.0.0"))
	require.NoError(t, clean.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired}))
	require.NoError(t, cr.ValidateResolution(&ConflictResolution{}, clean), "an acyclic graph validates")

	cyclic := NewDependencyGraph()
	cyclic.AddPlugin(newVerPlugin("a", "1.0.0"))
	cyclic.AddPlugin(newVerPlugin("b", "1.0.0"))
	require.NoError(t, cyclic.AddDependency("a", &Dependency{ID: "b", Type: DependencyTypeRequired}))
	require.NoError(t, cyclic.AddDependency("b", &Dependency{ID: "a", Type: DependencyTypeRequired}))
	require.Error(t, cr.ValidateResolution(&ConflictResolution{}, cyclic), "a remaining cycle must fail validation")
}

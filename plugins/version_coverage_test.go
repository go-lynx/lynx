package plugins

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseVersion_NoPanicAndFields(t *testing.T) {
	vm := NewVersionManager()

	// Regression: a plain version with no build metadata must parse, not panic.
	v, err := vm.ParseVersion("1.2.3")
	require.NoError(t, err)
	require.Equal(t, 1, v.Major)
	require.Equal(t, 2, v.Minor)
	require.Equal(t, 3, v.Patch)
	require.Empty(t, v.PreRelease)
	require.Empty(t, v.Build)

	// Leading v, partial versions.
	v, err = vm.ParseVersion("v2")
	require.NoError(t, err)
	require.Equal(t, 2, v.Major)
	require.Equal(t, 0, v.Minor)

	v, err = vm.ParseVersion("3.4")
	require.NoError(t, err)
	require.Equal(t, 4, v.Minor)
	require.Equal(t, 0, v.Patch)

	// Prerelease and build metadata.
	v, err = vm.ParseVersion("1.0.0-alpha.1+build7")
	require.NoError(t, err)
	require.Equal(t, "alpha.1", v.PreRelease)
	require.Equal(t, "build7", v.Build)

	// Prerelease without build must also not panic.
	v, err = vm.ParseVersion("1.0.0-rc1")
	require.NoError(t, err)
	require.Equal(t, "rc1", v.PreRelease)
	require.Empty(t, v.Build)

	// Errors.
	_, err = vm.ParseVersion("")
	require.Error(t, err)
	_, err = vm.ParseVersion("abc")
	require.Error(t, err)
	_, err = vm.ParseVersion("1.x.0")
	require.Error(t, err)
}

func TestCompareVersions(t *testing.T) {
	vm := NewVersionManager().(*DefaultVersionManager)
	mk := func(s string) *Version {
		v, err := vm.ParseVersion(s)
		require.NoError(t, err)
		return v
	}

	require.Equal(t, -1, vm.CompareVersions(mk("1.0.0"), mk("2.0.0")))
	require.Equal(t, 1, vm.CompareVersions(mk("1.2.0"), mk("1.1.9")))
	require.Equal(t, 0, vm.CompareVersions(mk("1.2.3"), mk("1.2.3")))
	require.Equal(t, 1, vm.CompareVersions(mk("1.0.1"), mk("1.0.0")))

	// release > prerelease
	require.Equal(t, 1, vm.CompareVersions(mk("1.0.0"), mk("1.0.0-rc1")))
	require.Equal(t, -1, vm.CompareVersions(mk("1.0.0-rc1"), mk("1.0.0")))

	// prerelease ordering: alpha < beta; numeric 2 < 10
	require.Equal(t, -1, vm.CompareVersions(mk("1.0.0-alpha"), mk("1.0.0-beta")))
	require.Equal(t, -1, vm.CompareVersions(mk("1.0.0-alpha.2"), mk("1.0.0-alpha.10")))

	// nil-safe
	require.Equal(t, 0, vm.CompareVersions(nil, mk("1.0.0")))
}

func TestSatisfiesConstraint(t *testing.T) {
	vm := NewVersionManager().(*DefaultVersionManager)
	mk := func(s string) *Version {
		v, _ := vm.ParseVersion(s)
		return v
	}

	require.True(t, vm.SatisfiesConstraint(mk("1.5.0"), nil), "nil constraint is always satisfied")

	require.True(t, vm.SatisfiesConstraint(mk("2.0.0"), &VersionConstraint{ExactVersion: "2.0.0"}))
	require.False(t, vm.SatisfiesConstraint(mk("2.0.1"), &VersionConstraint{ExactVersion: "2.0.0"}))

	require.True(t, vm.SatisfiesConstraint(mk("1.5.0"), &VersionConstraint{MinVersion: "1.0.0", MaxVersion: "2.0.0"}))
	require.False(t, vm.SatisfiesConstraint(mk("0.9.0"), &VersionConstraint{MinVersion: "1.0.0"}))
	require.False(t, vm.SatisfiesConstraint(mk("2.1.0"), &VersionConstraint{MaxVersion: "2.0.0"}))

	require.False(t, vm.SatisfiesConstraint(mk("1.5.0"), &VersionConstraint{ExcludeVersions: []string{"1.5.0"}}))
	require.True(t, vm.SatisfiesConstraint(mk("1.6.0"), &VersionConstraint{ExcludeVersions: []string{"1.5.0"}}))
}

func TestVersionRange(t *testing.T) {
	vm := NewVersionManager().(*DefaultVersionManager)
	mk := func(s string) *Version {
		v, _ := vm.ParseVersion(s)
		return v
	}

	rng, err := vm.ParseVersionRange(">=1.0.0")
	require.NoError(t, err)
	require.True(t, vm.IsVersionInRange(mk("1.2.0"), rng))
	require.False(t, vm.IsVersionInRange(mk("0.9.0"), rng))

	rng, err = vm.ParseVersionRange("<=2.0.0")
	require.NoError(t, err)
	require.True(t, vm.IsVersionInRange(mk("2.0.0"), rng))
	require.False(t, vm.IsVersionInRange(mk("2.0.1"), rng))

	rng, err = vm.ParseVersionRange("1.0.0 - 2.0.0")
	require.NoError(t, err)
	require.True(t, vm.IsVersionInRange(mk("1.5.0"), rng))
	require.False(t, vm.IsVersionInRange(mk("2.5.0"), rng))

	// ">" bumps the min by one patch
	rng, err = vm.ParseVersionRange(">1.0.0")
	require.NoError(t, err)
	require.False(t, vm.IsVersionInRange(mk("1.0.0"), rng))
	require.True(t, vm.IsVersionInRange(mk("1.0.1"), rng))

	_, err = vm.ParseVersionRange("~weird~")
	require.Error(t, err)
}

func TestVersionStringAndFlags(t *testing.T) {
	vm := NewVersionManager().(*DefaultVersionManager)

	v, _ := vm.ParseVersion("1.2.3-rc1+meta")
	require.Equal(t, "1.2.3-rc1+meta", v.String())
	require.False(t, v.IsStable())
	require.True(t, v.IsPreRelease())

	stable, _ := vm.ParseVersion("1.2.3")
	require.True(t, stable.IsStable())
	require.False(t, stable.IsPreRelease())
	require.Equal(t, "1.2.3", stable.String())

	var nilV *Version
	require.Empty(t, nilV.String())
}

func TestGetCompatibleVersions(t *testing.T) {
	vm := NewVersionManager().(*DefaultVersionManager)
	mk := func(s string) *Version {
		v, _ := vm.ParseVersion(s)
		return v
	}
	available := []*Version{mk("0.9.0"), mk("1.0.0"), mk("1.5.0"), mk("2.1.0")}
	compatible := vm.GetCompatibleVersions(&VersionConstraint{MinVersion: "1.0.0", MaxVersion: "2.0.0"}, available)
	require.Len(t, compatible, 2)
}

func TestResolveVersionConflict(t *testing.T) {
	vm := NewVersionManager().(*DefaultVersionManager)

	res, err := vm.ResolveVersionConflict(nil)
	require.NoError(t, err)
	require.Nil(t, res)

	conflicts := []VersionConflict{
		{PluginID: "a", DependencyID: "x", AvailableVersion: "1.0.0"},
		{PluginID: "a", DependencyID: "x", AvailableVersion: "1.3.0"},
	}
	res, err = vm.ResolveVersionConflict(conflicts)
	require.NoError(t, err)
	// Conflicts are grouped by DependencyID, and the highest available version wins.
	require.Equal(t, "1.3.0", res["x"])
}

func TestPluginID(t *testing.T) {
	// vX-style version round-trips through generate/parse (4 dot-separated parts).
	id := GeneratePluginID("go-lynx", "http", "v1")
	require.Equal(t, "go-lynx.plugin.http.v1", id)
	require.NoError(t, ValidatePluginID(id))

	f, err := ParsePluginID(id)
	require.NoError(t, err)
	require.Equal(t, "go-lynx", f.Organization)
	require.Equal(t, "http", f.Name)
	require.Equal(t, "v1", f.Version)

	main, err := GetPluginMainVersion(id)
	require.NoError(t, err)
	require.Equal(t, "v1", main)

	require.True(t, IsPluginVersionCompatible("go-lynx.plugin.http.v1", "go-lynx.plugin.http.v1"))
	require.False(t, IsPluginVersionCompatible("go-lynx.plugin.http.v1", "go-lynx.plugin.http.v2"))

	// Empty org falls back to the default org prefix.
	require.Contains(t, GeneratePluginID("", "redis", "v2"), ".plugin.redis.v2")

	// Malformed IDs are rejected.
	_, err = ParsePluginID("not-an-id")
	require.Error(t, err)
	require.Error(t, ValidatePluginID("bad.id"))
}

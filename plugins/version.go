package plugins

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version structure
type Version struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	Build      string
	Original   string
}

// VersionManager version manager interface
type VersionManager interface {
	// ParseVersion parses version string
	ParseVersion(version string) (*Version, error)
	// CompareVersions compares two versions
	CompareVersions(v1, v2 *Version) int
	// SatisfiesConstraint checks if version satisfies constraint
	SatisfiesConstraint(version *Version, constraint *VersionConstraint) bool
	// ResolveVersionConflict resolves version conflicts
	ResolveVersionConflict(conflicts []VersionConflict) (map[string]string, error)
	// GetCompatibleVersions gets compatible version list
	GetCompatibleVersions(required *VersionConstraint, available []*Version) []*Version
}

// DefaultVersionManager default version manager implementation
type DefaultVersionManager struct{}

// NewVersionManager creates a new version manager
func NewVersionManager() VersionManager {
	return &DefaultVersionManager{}
}

// ParseVersion parses version string
func (vm *DefaultVersionManager) ParseVersion(version string) (*Version, error) {
	if version == "" {
		return nil, fmt.Errorf("version string cannot be empty")
	}

	// Remove prefix v (if exists)
	version = strings.TrimPrefix(version, "v")

	// Split version and pre-release information
	parts := strings.SplitN(version, "-", 2)
	versionPart := parts[0]
	var preRelease string
	if len(parts) > 1 {
		preRelease = parts[1]
	}

	// Split version and build information
	buildParts := strings.SplitN(preRelease, "+", 2)
	if len(buildParts) > 1 {
		preRelease = buildParts[0]
	}

	// Parse major version number
	versionNumbers := strings.Split(versionPart, ".")
	if len(versionNumbers) < 1 {
		return nil, fmt.Errorf("invalid version format: %s", version)
	}

	major, err := strconv.Atoi(versionNumbers[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", versionNumbers[0])
	}

	minor := 0
	if len(versionNumbers) > 1 {
		minor, err = strconv.Atoi(versionNumbers[1])
		if err != nil {
			return nil, fmt.Errorf("invalid minor version: %s", versionNumbers[1])
		}
	}

	patch := 0
	if len(versionNumbers) > 2 {
		patch, err = strconv.Atoi(versionNumbers[2])
		if err != nil {
			return nil, fmt.Errorf("invalid patch version: %s", versionNumbers[2])
		}
	}

	return &Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: preRelease,
		Build:      buildParts[1],
		Original:   version,
	}, nil
}

// CompareVersions compares two versions
// Return value: -1 (v1 < v2), 0 (v1 == v2), 1 (v1 > v2)
func (vm *DefaultVersionManager) CompareVersions(v1, v2 *Version) int {
	if v1 == nil || v2 == nil {
		return 0
	}

	// Compare major version numbers
	if v1.Major != v2.Major {
		if v1.Major < v2.Major {
			return -1
		}
		return 1
	}

	// Compare minor version numbers
	if v1.Minor != v2.Minor {
		if v1.Minor < v2.Minor {
			return -1
		}
		return 1
	}

	// Compare patch version numbers
	if v1.Patch != v2.Patch {
		if v1.Patch < v2.Patch {
			return -1
		}
		return 1
	}

	// Compare pre-release versions
	if v1.PreRelease == "" && v2.PreRelease == "" {
		return 0
	}
	if v1.PreRelease == "" {
		return 1 // Release version > pre-release version
	}
	if v2.PreRelease == "" {
		return -1 // Pre-release version < release version
	}

	// Pre-release version comparison
	return vm.comparePreRelease(v1.PreRelease, v2.PreRelease)
}

// comparePreRelease compares pre-release versions
func (vm *DefaultVersionManager) comparePreRelease(pr1, pr2 string) int {
	parts1 := strings.Split(pr1, ".")
	parts2 := strings.Split(pr2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var part1, part2 string
		if i < len(parts1) {
			part1 = parts1[i]
		}
		if i < len(parts2) {
			part2 = parts2[i]
		}

		// Numeric part comparison
		if vm.isNumeric(part1) && vm.isNumeric(part2) {
			num1, _ := strconv.Atoi(part1)
			num2, _ := strconv.Atoi(part2)
			if num1 != num2 {
				if num1 < num2 {
					return -1
				}
				return 1
			}
		} else {
			// String part comparison
			if part1 < part2 {
				return -1
			}
			if part1 > part2 {
				return 1
			}
		}
	}

	return 0
}

// isNumeric checks if string is numeric
func (vm *DefaultVersionManager) isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// SatisfiesConstraint checks if version satisfies constraint
func (vm *DefaultVersionManager) SatisfiesConstraint(version *Version, constraint *VersionConstraint) bool {
	if constraint == nil || version == nil {
		return true
	}

	// Check exact version
	if constraint.ExactVersion != "" {
		exactVersion, err := vm.ParseVersion(constraint.ExactVersion)
		if err != nil {
			return false
		}
		return vm.CompareVersions(version, exactVersion) == 0
	}

	// Check excluded versions
	for _, excludedVersion := range constraint.ExcludeVersions {
		excluded, err := vm.ParseVersion(excludedVersion)
		if err != nil {
			continue
		}
		if vm.CompareVersions(version, excluded) == 0 {
			return false
		}
	}

	// Check minimum version
	if constraint.MinVersion != "" {
		minVersion, err := vm.ParseVersion(constraint.MinVersion)
		if err != nil {
			return false
		}
		if vm.CompareVersions(version, minVersion) < 0 {
			return false
		}
	}

	// Check maximum version
	if constraint.MaxVersion != "" {
		maxVersion, err := vm.ParseVersion(constraint.MaxVersion)
		if err != nil {
			return false
		}
		if vm.CompareVersions(version, maxVersion) > 0 {
			return false
		}
	}

	return true
}

// ResolveVersionConflict resolves version conflicts
func (vm *DefaultVersionManager) ResolveVersionConflict(conflicts []VersionConflict) (map[string]string, error) {
	if len(conflicts) == 0 {
		return nil, nil
	}

	resolution := make(map[string]string)
	conflictGroups := vm.groupConflictsByPlugin(conflicts)

	for pluginID, pluginConflicts := range conflictGroups {
		// Select best version for each plugin
		bestVersion, err := vm.selectBestVersion(pluginConflicts)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve conflicts for plugin %s: %w", pluginID, err)
		}
		resolution[pluginID] = bestVersion
	}

	return resolution, nil
}

// groupConflictsByPlugin groups conflicts by plugin
func (vm *DefaultVersionManager) groupConflictsByPlugin(conflicts []VersionConflict) map[string][]VersionConflict {
	groups := make(map[string][]VersionConflict)

	for _, conflict := range conflicts {
		groups[conflict.DependencyID] = append(groups[conflict.DependencyID], conflict)
	}

	return groups
}

// selectBestVersion selects the best version for a plugin
func (vm *DefaultVersionManager) selectBestVersion(conflicts []VersionConflict) (string, error) {
	if len(conflicts) == 0 {
		return "", fmt.Errorf("no conflicts provided")
	}

	// Collect all available versions
	availableVersions := make(map[string]bool)
	for _, conflict := range conflicts {
		availableVersions[conflict.AvailableVersion] = true
	}

	// Parse all versions
	versions := make([]*Version, 0)
	for versionStr := range availableVersions {
		version, err := vm.ParseVersion(versionStr)
		if err != nil {
			continue
		}
		versions = append(versions, version)
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no valid versions available")
	}

	// Select highest version (usually the latest stable version)
	bestVersion := versions[0]
	for _, version := range versions[1:] {
		if vm.CompareVersions(version, bestVersion) > 0 {
			bestVersion = version
		}
	}

	return bestVersion.Original, nil
}

// GetCompatibleVersions gets compatible version list
func (vm *DefaultVersionManager) GetCompatibleVersions(required *VersionConstraint, available []*Version) []*Version {
	var compatible []*Version

	for _, version := range available {
		if vm.SatisfiesConstraint(version, required) {
			compatible = append(compatible, version)
		}
	}

	return compatible
}

// VersionRange version range
type VersionRange struct {
	Min *Version
	Max *Version
}

// ParseVersionRange parses version range string
func (vm *DefaultVersionManager) ParseVersionRange(rangeStr string) (*VersionRange, error) {
	// Supported formats: ">=1.0.0", "<=2.0.0", "1.0.0 - 2.0.0"

	// Check range format
	if strings.Contains(rangeStr, " - ") {
		parts := strings.Split(rangeStr, " - ")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid range format: %s", rangeStr)
		}

		minVersion, err := vm.ParseVersion(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid min version: %s", parts[0])
		}

		maxVersion, err := vm.ParseVersion(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid max version: %s", parts[1])
		}

		return &VersionRange{Min: minVersion, Max: maxVersion}, nil
	}

	// Check comparison operator format
	re := regexp.MustCompile(`^([<>=]+)\s*(.+)$`)
	matches := re.FindStringSubmatch(rangeStr)
	if len(matches) == 3 {
		operator := matches[1]
		versionStr := matches[2]

		version, err := vm.ParseVersion(versionStr)
		if err != nil {
			return nil, fmt.Errorf("invalid version: %s", versionStr)
		}

		switch operator {
		case ">=":
			return &VersionRange{Min: version}, nil
		case "<=":
			return &VersionRange{Max: version}, nil
		case ">":
			// Create a version one patch higher than the current version
			nextVersion := &Version{
				Major: version.Major,
				Minor: version.Minor,
				Patch: version.Patch + 1,
			}
			return &VersionRange{Min: nextVersion}, nil
		case "<":
			return &VersionRange{Max: version}, nil
		}
	}

	return nil, fmt.Errorf("unsupported range format: %s", rangeStr)
}

// IsVersionInRange checks if version is within range
func (vm *DefaultVersionManager) IsVersionInRange(version *Version, rng *VersionRange) bool {
	if rng == nil || version == nil {
		return true
	}

	if rng.Min != nil && vm.CompareVersions(version, rng.Min) < 0 {
		return false
	}

	if rng.Max != nil && vm.CompareVersions(version, rng.Max) > 0 {
		return false
	}

	return true
}

// GetVersionString gets version string representation
func (v *Version) String() string {
	if v == nil {
		return ""
	}

	result := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)

	if v.PreRelease != "" {
		result += "-" + v.PreRelease
	}

	if v.Build != "" {
		result += "+" + v.Build
	}

	return result
}

// IsStable checks if it's a stable version
func (v *Version) IsStable() bool {
	return v != nil && v.PreRelease == ""
}

// IsPreRelease checks if it's a pre-release version
func (v *Version) IsPreRelease() bool {
	return v != nil && v.PreRelease != ""
}

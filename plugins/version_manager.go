package plugins

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version 版本结构
type Version struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	Build      string
	Original   string
}

// VersionManager 版本管理器接口
type VersionManager interface {
	// ParseVersion 解析版本字符串
	ParseVersion(version string) (*Version, error)
	// CompareVersions 比较两个版本
	CompareVersions(v1, v2 *Version) int
	// SatisfiesConstraint 检查版本是否满足约束
	SatisfiesConstraint(version *Version, constraint *VersionConstraint) bool
	// ResolveVersionConflict 解决版本冲突
	ResolveVersionConflict(conflicts []VersionConflict) (map[string]string, error)
	// GetCompatibleVersions 获取兼容版本列表
	GetCompatibleVersions(required *VersionConstraint, available []*Version) []*Version
}

// DefaultVersionManager 默认版本管理器实现
type DefaultVersionManager struct{}

// NewVersionManager 创建新的版本管理器
func NewVersionManager() VersionManager {
	return &DefaultVersionManager{}
}

// ParseVersion 解析版本字符串
func (vm *DefaultVersionManager) ParseVersion(version string) (*Version, error) {
	if version == "" {
		return nil, fmt.Errorf("version string cannot be empty")
	}

	// 移除前缀v（如果存在）
	version = strings.TrimPrefix(version, "v")

	// 分割版本和预发布信息
	parts := strings.SplitN(version, "-", 2)
	versionPart := parts[0]
	var preRelease string
	if len(parts) > 1 {
		preRelease = parts[1]
	}

	// 分割版本和构建信息
	buildParts := strings.SplitN(preRelease, "+", 2)
	if len(buildParts) > 1 {
		preRelease = buildParts[0]
	}

	// 解析主要版本号
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

// CompareVersions 比较两个版本
// 返回值: -1 (v1 < v2), 0 (v1 == v2), 1 (v1 > v2)
func (vm *DefaultVersionManager) CompareVersions(v1, v2 *Version) int {
	if v1 == nil || v2 == nil {
		return 0
	}

	// 比较主要版本号
	if v1.Major != v2.Major {
		if v1.Major < v2.Major {
			return -1
		}
		return 1
	}

	// 比较次要版本号
	if v1.Minor != v2.Minor {
		if v1.Minor < v2.Minor {
			return -1
		}
		return 1
	}

	// 比较补丁版本号
	if v1.Patch != v2.Patch {
		if v1.Patch < v2.Patch {
			return -1
		}
		return 1
	}

	// 比较预发布版本
	if v1.PreRelease == "" && v2.PreRelease == "" {
		return 0
	}
	if v1.PreRelease == "" {
		return 1 // 正式版本 > 预发布版本
	}
	if v2.PreRelease == "" {
		return -1 // 预发布版本 < 正式版本
	}

	// 预发布版本比较
	return vm.comparePreRelease(v1.PreRelease, v2.PreRelease)
}

// comparePreRelease 比较预发布版本
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

		// 数字部分比较
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
			// 字符串部分比较
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

// isNumeric 检查字符串是否为数字
func (vm *DefaultVersionManager) isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// SatisfiesConstraint 检查版本是否满足约束
func (vm *DefaultVersionManager) SatisfiesConstraint(version *Version, constraint *VersionConstraint) bool {
	if constraint == nil || version == nil {
		return true
	}

	// 检查精确版本
	if constraint.ExactVersion != "" {
		exactVersion, err := vm.ParseVersion(constraint.ExactVersion)
		if err != nil {
			return false
		}
		return vm.CompareVersions(version, exactVersion) == 0
	}

	// 检查排除版本
	for _, excludedVersion := range constraint.ExcludeVersions {
		excluded, err := vm.ParseVersion(excludedVersion)
		if err != nil {
			continue
		}
		if vm.CompareVersions(version, excluded) == 0 {
			return false
		}
	}

	// 检查最小版本
	if constraint.MinVersion != "" {
		minVersion, err := vm.ParseVersion(constraint.MinVersion)
		if err != nil {
			return false
		}
		if vm.CompareVersions(version, minVersion) < 0 {
			return false
		}
	}

	// 检查最大版本
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

// ResolveVersionConflict 解决版本冲突
func (vm *DefaultVersionManager) ResolveVersionConflict(conflicts []VersionConflict) (map[string]string, error) {
	if len(conflicts) == 0 {
		return nil, nil
	}

	resolution := make(map[string]string)
	conflictGroups := vm.groupConflictsByPlugin(conflicts)

	for pluginID, pluginConflicts := range conflictGroups {
		// 为每个插件选择最佳版本
		bestVersion, err := vm.selectBestVersion(pluginConflicts)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve conflicts for plugin %s: %w", pluginID, err)
		}
		resolution[pluginID] = bestVersion
	}

	return resolution, nil
}

// groupConflictsByPlugin 按插件分组冲突
func (vm *DefaultVersionManager) groupConflictsByPlugin(conflicts []VersionConflict) map[string][]VersionConflict {
	groups := make(map[string][]VersionConflict)

	for _, conflict := range conflicts {
		groups[conflict.DependencyID] = append(groups[conflict.DependencyID], conflict)
	}

	return groups
}

// selectBestVersion 为插件选择最佳版本
func (vm *DefaultVersionManager) selectBestVersion(conflicts []VersionConflict) (string, error) {
	if len(conflicts) == 0 {
		return "", fmt.Errorf("no conflicts provided")
	}

	// 收集所有可用版本
	availableVersions := make(map[string]bool)
	for _, conflict := range conflicts {
		availableVersions[conflict.AvailableVersion] = true
	}

	// 解析所有版本
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

	// 选择最高版本（通常是最新的稳定版本）
	bestVersion := versions[0]
	for _, version := range versions[1:] {
		if vm.CompareVersions(version, bestVersion) > 0 {
			bestVersion = version
		}
	}

	return bestVersion.Original, nil
}

// GetCompatibleVersions 获取兼容版本列表
func (vm *DefaultVersionManager) GetCompatibleVersions(required *VersionConstraint, available []*Version) []*Version {
	var compatible []*Version

	for _, version := range available {
		if vm.SatisfiesConstraint(version, required) {
			compatible = append(compatible, version)
		}
	}

	return compatible
}

// VersionRange 版本范围
type VersionRange struct {
	Min *Version
	Max *Version
}

// ParseVersionRange 解析版本范围字符串
func (vm *DefaultVersionManager) ParseVersionRange(rangeStr string) (*VersionRange, error) {
	// 支持格式: ">=1.0.0", "<=2.0.0", "1.0.0 - 2.0.0"

	// 检查范围格式
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

	// 检查比较操作符格式
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
			// 创建比当前版本高一个补丁版本的版本
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

// IsVersionInRange 检查版本是否在范围内
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

// GetVersionString 获取版本字符串表示
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

// IsStable 检查是否为稳定版本
func (v *Version) IsStable() bool {
	return v != nil && v.PreRelease == ""
}

// IsPreRelease 检查是否为预发布版本
func (v *Version) IsPreRelease() bool {
	return v != nil && v.PreRelease != ""
}

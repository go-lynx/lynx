package plugins

import (
	"fmt"
	"sort"
	"strings"
)

// ConflictResolver 依赖冲突解决器接口
type ConflictResolver interface {
	// DetectConflicts 检测所有依赖冲突
	DetectConflicts(graph *DependencyGraph) ([]DependencyConflict, error)
	// ResolveConflicts 解决依赖冲突
	ResolveConflicts(conflicts []DependencyConflict) (*ConflictResolution, error)
	// SuggestAlternatives 建议替代方案
	SuggestAlternatives(conflict DependencyConflict, availablePlugins map[string][]Plugin) []ConflictAlternative
	// ValidateResolution 验证冲突解决方案
	ValidateResolution(resolution *ConflictResolution, graph *DependencyGraph) error
}

// DependencyConflict 依赖冲突信息
type DependencyConflict struct {
	ID          string             `json:"id"`
	Type        ConflictType       `json:"type"`
	Severity    ConflictSeverity   `json:"severity"`
	Description string             `json:"description"`
	Plugins     []string           `json:"plugins"`
	Details     []ConflictDetail   `json:"details"`
	Solutions   []ConflictSolution `json:"solutions"`
}

// ConflictType 冲突类型
type ConflictType string

const (
	// ConflictTypeVersion 版本冲突
	ConflictTypeVersion ConflictType = "version"
	// ConflictTypeCircular 循环依赖冲突
	ConflictTypeCircular ConflictType = "circular"
	// ConflictTypeMissing 缺失依赖冲突
	ConflictTypeMissing ConflictType = "missing"
	// ConflictTypeIncompatible 不兼容冲突
	ConflictTypeIncompatible ConflictType = "incompatible"
	// ConflictTypeResource 资源冲突
	ConflictTypeResource ConflictType = "resource"
)

// ConflictSeverity 冲突严重程度
type ConflictSeverity string

const (
	// ConflictSeverityCritical 严重冲突
	ConflictSeverityCritical ConflictSeverity = "critical"
	// ConflictSeverityHigh 高优先级冲突
	ConflictSeverityHigh ConflictSeverity = "high"
	// ConflictSeverityMedium 中等优先级冲突
	ConflictSeverityMedium ConflictSeverity = "medium"
	// ConflictSeverityLow 低优先级冲突
	ConflictSeverityLow ConflictSeverity = "low"
	// ConflictSeverityInfo 信息性冲突
	ConflictSeverityInfo ConflictSeverity = "info"
)

// ConflictDetail 冲突详细信息
type ConflictDetail struct {
	PluginID       string `json:"plugin_id"`
	DependencyID   string `json:"dependency_id"`
	RequiredValue  string `json:"required_value"`
	AvailableValue string `json:"available_value"`
	Message        string `json:"message"`
}

// ConflictSolution 冲突解决方案
type ConflictSolution struct {
	ID          string           `json:"id"`
	Type        SolutionType     `json:"type"`
	Description string           `json:"description"`
	Actions     []SolutionAction `json:"actions"`
	Risk        SolutionRisk     `json:"risk"`
	Priority    int              `json:"priority"`
}

// SolutionType 解决方案类型
type SolutionType string

const (
	// SolutionTypeUpgrade 升级版本
	SolutionTypeUpgrade SolutionType = "upgrade"
	// SolutionTypeDowngrade 降级版本
	SolutionTypeDowngrade SolutionType = "downgrade"
	// SolutionTypeReplace 替换插件
	SolutionTypeReplace SolutionType = "replace"
	// SolutionTypeRemove 移除插件
	SolutionTypeRemove SolutionType = "remove"
	// SolutionTypeConfigure 配置调整
	SolutionTypeConfigure SolutionType = "configure"
)

// SolutionAction 解决方案动作
type SolutionAction struct {
	Type        string            `json:"type"`
	Target      string            `json:"target"`
	Value       string            `json:"value"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters"`
}

// SolutionRisk 解决方案风险
type SolutionRisk string

const (
	// SolutionRiskLow 低风险
	SolutionRiskLow SolutionRisk = "low"
	// SolutionRiskMedium 中等风险
	SolutionRiskMedium SolutionRisk = "medium"
	// SolutionRiskHigh 高风险
	SolutionRiskHigh SolutionRisk = "high"
)

// ConflictAlternative 冲突替代方案
type ConflictAlternative struct {
	PluginID      string       `json:"plugin_id"`
	Name          string       `json:"name"`
	Version       string       `json:"version"`
	Description   string       `json:"description"`
	Compatibility float64      `json:"compatibility"` // 兼容性评分 0-1
	Risk          SolutionRisk `json:"risk"`
}

// ConflictResolution 冲突解决方案
type ConflictResolution struct {
	ResolvedConflicts  []string           `json:"resolved_conflicts"`
	RemainingConflicts []string           `json:"remaining_conflicts"`
	Actions            []ResolutionAction `json:"actions"`
	Summary            string             `json:"summary"`
	Risk               SolutionRisk       `json:"risk"`
}

// ResolutionAction 解决方案动作
type ResolutionAction struct {
	ConflictID string           `json:"conflict_id"`
	SolutionID string           `json:"solution_id"`
	Actions    []SolutionAction `json:"actions"`
	Status     ActionStatus     `json:"status"`
}

// ActionStatus 动作状态
type ActionStatus string

const (
	// ActionStatusPending 待执行
	ActionStatusPending ActionStatus = "pending"
	// ActionStatusInProgress 执行中
	ActionStatusInProgress ActionStatus = "in_progress"
	// ActionStatusCompleted 已完成
	ActionStatusCompleted ActionStatus = "completed"
	// ActionStatusFailed 执行失败
	ActionStatusFailed ActionStatus = "failed"
	// ActionStatusRollback 已回滚
	ActionStatusRollback ActionStatus = "rollback"
)

// DefaultConflictResolver 默认冲突解决器实现
type DefaultConflictResolver struct {
	versionManager VersionManager
}

// NewConflictResolver 创建新的冲突解决器
func NewConflictResolver(versionManager VersionManager) ConflictResolver {
	return &DefaultConflictResolver{
		versionManager: versionManager,
	}
}

// DetectConflicts 检测所有依赖冲突
func (cr *DefaultConflictResolver) DetectConflicts(graph *DependencyGraph) ([]DependencyConflict, error) {
	var conflicts []DependencyConflict

	// 检测循环依赖
	if cycle, err := graph.CheckCircularDependencies(); err != nil {
		conflicts = append(conflicts, DependencyConflict{
			ID:          "circular_dependency",
			Type:        ConflictTypeCircular,
			Severity:    ConflictSeverityCritical,
			Description: fmt.Sprintf("Circular dependency detected: %s", strings.Join(cycle, " -> ")),
			Plugins:     cycle,
			Details: []ConflictDetail{
				{
					Message: fmt.Sprintf("Circular dependency chain: %s", strings.Join(cycle, " -> ")),
				},
			},
			Solutions: cr.generateCircularDependencySolutions(cycle),
		})
	}

	// 检测版本冲突
	versionConflicts, err := graph.CheckVersionConflicts()
	if err != nil {
		return nil, fmt.Errorf("failed to check version conflicts: %w", err)
	}

	if len(versionConflicts) > 0 {
		conflicts = append(conflicts, cr.convertVersionConflicts(versionConflicts)...)
	}

	// 检测缺失依赖
	missingConflicts := cr.detectMissingDependencies(graph)
	conflicts = append(conflicts, missingConflicts...)

	// 检测资源冲突
	resourceConflicts := cr.detectResourceConflicts(graph)
	conflicts = append(conflicts, resourceConflicts...)

	return conflicts, nil
}

// convertVersionConflicts 转换版本冲突
func (cr *DefaultConflictResolver) convertVersionConflicts(versionConflicts []VersionConflict) []DependencyConflict {
	var conflicts []DependencyConflict

	// 按插件分组版本冲突
	conflictGroups := make(map[string][]VersionConflict)
	for _, vc := range versionConflicts {
		conflictGroups[vc.DependencyID] = append(conflictGroups[vc.DependencyID], vc)
	}

	for depID, vcs := range conflictGroups {
		plugins := make([]string, 0)
		details := make([]ConflictDetail, 0)

		for _, vc := range vcs {
			plugins = append(plugins, vc.PluginID)
			details = append(details, ConflictDetail{
				PluginID:       vc.PluginID,
				DependencyID:   vc.DependencyID,
				RequiredValue:  vc.RequiredVersion,
				AvailableValue: vc.AvailableVersion,
				Message:        vc.Description,
			})
		}

		conflicts = append(conflicts, DependencyConflict{
			ID:          fmt.Sprintf("version_conflict_%s", depID),
			Type:        ConflictTypeVersion,
			Severity:    ConflictSeverityHigh,
			Description: fmt.Sprintf("Version conflict for dependency %s", depID),
			Plugins:     plugins,
			Details:     details,
			Solutions:   cr.generateVersionConflictSolutions(vcs),
		})
	}

	return conflicts
}

// detectMissingDependencies 检测缺失依赖
func (cr *DefaultConflictResolver) detectMissingDependencies(graph *DependencyGraph) []DependencyConflict {
	var conflicts []DependencyConflict

	// 遍历所有插件的依赖关系
	for pluginID, deps := range graph.GetAllDependencies() {
		for _, dep := range deps {
			if dep.Type == DependencyTypeRequired {
				// 检查依赖的插件是否存在
				if !graph.HasPlugin(dep.ID) {
					conflicts = append(conflicts, DependencyConflict{
						ID:          fmt.Sprintf("missing_dependency_%s_%s", pluginID, dep.ID),
						Type:        ConflictTypeMissing,
						Severity:    ConflictSeverityHigh,
						Description: fmt.Sprintf("Required dependency %s is missing for plugin %s", dep.ID, pluginID),
						Plugins:     []string{pluginID},
						Details: []ConflictDetail{
							{
								PluginID:       pluginID,
								DependencyID:   dep.ID,
								RequiredValue:  dep.Name,
								AvailableValue: "not found",
								Message:        fmt.Sprintf("Plugin %s requires dependency %s but it's not available", pluginID, dep.ID),
							},
						},
						Solutions: cr.generateMissingDependencySolutions(pluginID, dep),
					})
				}
			}
		}
	}

	return conflicts
}

// detectResourceConflicts 检测资源冲突
func (cr *DefaultConflictResolver) detectResourceConflicts(graph *DependencyGraph) []DependencyConflict {
	var conflicts []DependencyConflict

	// 这里需要检查资源名称冲突
	// 由于当前架构中没有直接的资源注册信息，我们提供一个框架实现
	// 实际使用中需要扩展 Plugin 接口以支持资源注册信息
	
	// 检查插件名称冲突
	pluginNames := make(map[string][]string)
	for pluginID, plugin := range graph.GetAllPlugins() {
		if plugin != nil {
			// 假设插件有 Name() 方法，如果没有则需要扩展接口
			// 这里使用插件ID作为名称的替代
			name := pluginID
			pluginNames[name] = append(pluginNames[name], pluginID)
		}
	}

	// 检测名称冲突
	for name, pluginIDs := range pluginNames {
		if len(pluginIDs) > 1 {
			conflicts = append(conflicts, DependencyConflict{
				ID:          fmt.Sprintf("resource_conflict_%s", name),
				Type:        ConflictTypeResource,
				Severity:    ConflictSeverityMedium,
				Description: fmt.Sprintf("Multiple plugins with same name: %s", name),
				Plugins:     pluginIDs,
				Details: []ConflictDetail{
					{
						PluginID:       strings.Join(pluginIDs, ","),
						DependencyID:   name,
						RequiredValue:  "unique name",
						AvailableValue: fmt.Sprintf("%d plugins with same name", len(pluginIDs)),
						Message:        fmt.Sprintf("Name conflict detected for %s", name),
					},
				},
				Solutions: cr.generateResourceConflictSolutions(name, pluginIDs),
			})
		}
	}

	return conflicts
}

// generateResourceConflictSolutions 生成资源冲突解决方案
func (cr *DefaultConflictResolver) generateResourceConflictSolutions(name string, pluginIDs []string) []ConflictSolution {
	var solutions []ConflictSolution

	// 方案1: 重命名插件
	solutions = append(solutions, ConflictSolution{
		ID:          "rename_plugin",
		Type:        SolutionTypeConfigure,
		Description: fmt.Sprintf("Rename plugins to avoid name conflict: %s", name),
		Actions: []SolutionAction{
			{
				Type:        "rename",
				Target:      strings.Join(pluginIDs, ","),
				Description: fmt.Sprintf("Rename plugins to have unique names"),
			},
		},
		Risk:     SolutionRiskLow,
		Priority: 1,
	})

	// 方案2: 移除重复插件
	solutions = append(solutions, ConflictSolution{
		ID:          "remove_duplicate",
		Type:        SolutionTypeRemove,
		Description: fmt.Sprintf("Remove duplicate plugins with name: %s", name),
		Actions: []SolutionAction{
			{
				Type:        "remove",
				Target:      strings.Join(pluginIDs[1:], ","),
				Description: fmt.Sprintf("Keep first plugin, remove others with same name"),
			},
		},
		Risk:     SolutionRiskMedium,
		Priority: 2,
	})

	// 方案3: 合并插件
	solutions = append(solutions, ConflictSolution{
		ID:          "merge_plugins",
		Type:        SolutionTypeConfigure,
		Description: fmt.Sprintf("Merge plugins with same name: %s", name),
		Actions: []SolutionAction{
			{
				Type:        "merge",
				Target:      strings.Join(pluginIDs, ","),
				Description: fmt.Sprintf("Merge functionality of duplicate plugins"),
			},
		},
		Risk:     SolutionRiskHigh,
		Priority: 3,
	})

	return solutions
}

// generateCircularDependencySolutions 生成循环依赖解决方案
func (cr *DefaultConflictResolver) generateCircularDependencySolutions(cycle []string) []ConflictSolution {
	var solutions []ConflictSolution

	// 方案1: 移除其中一个依赖
	if len(cycle) > 0 {
		solutions = append(solutions, ConflictSolution{
			ID:          "remove_dependency",
			Type:        SolutionTypeRemove,
			Description: fmt.Sprintf("Remove dependency from %s to break circular dependency", cycle[0]),
			Actions: []SolutionAction{
				{
					Type:        "remove_dependency",
					Target:      cycle[0],
					Description: fmt.Sprintf("Remove dependency on %s", cycle[len(cycle)-1]),
				},
			},
			Risk:     SolutionRiskMedium,
			Priority: 1,
		})
	}

	// 方案2: 重构依赖关系
	solutions = append(solutions, ConflictSolution{
		ID:          "restructure_dependencies",
		Type:        SolutionTypeConfigure,
		Description: "Restructure dependencies to eliminate circular references",
		Actions: []SolutionAction{
			{
				Type:        "restructure",
				Target:      "all",
				Description: "Analyze and restructure dependency relationships",
			},
		},
		Risk:     SolutionRiskLow,
		Priority: 2,
	})

	return solutions
}

// generateVersionConflictSolutions 生成版本冲突解决方案
func (cr *DefaultConflictResolver) generateVersionConflictSolutions(conflicts []VersionConflict) []ConflictSolution {
	var solutions []ConflictSolution

	// 方案1: 升级到兼容版本
	solutions = append(solutions, ConflictSolution{
		ID:          "upgrade_version",
		Type:        SolutionTypeUpgrade,
		Description: "Upgrade to a compatible version",
		Actions: []SolutionAction{
			{
				Type:        "upgrade",
				Target:      "dependency",
				Description: "Upgrade dependency to compatible version",
			},
		},
		Risk:     SolutionRiskLow,
		Priority: 1,
	})

	// 方案2: 降级到兼容版本
	solutions = append(solutions, ConflictSolution{
		ID:          "downgrade_version",
		Type:        SolutionTypeDowngrade,
		Description: "Downgrade to a compatible version",
		Actions: []SolutionAction{
			{
				Type:        "downgrade",
				Target:      "dependency",
				Description: "Downgrade dependency to compatible version",
			},
		},
		Risk:     SolutionRiskMedium,
		Priority: 2,
	})

	return solutions
}

// generateMissingDependencySolutions 生成缺失依赖的解决方案
func (cr *DefaultConflictResolver) generateMissingDependencySolutions(pluginID string, dep *Dependency) []ConflictSolution {
	var solutions []ConflictSolution

	// 方案1: 安装缺失的依赖
	solutions = append(solutions, ConflictSolution{
		ID:          "install_dependency",
		Type:        SolutionTypeConfigure,
		Description: fmt.Sprintf("Install missing dependency %s", dep.ID),
		Actions: []SolutionAction{
			{
				Type:        "install",
				Target:      dep.ID,
				Description: fmt.Sprintf("Install plugin %s to satisfy dependency", dep.ID),
			},
		},
		Risk:     SolutionRiskLow,
		Priority: 1,
	})

	// 方案2: 寻找替代插件
	solutions = append(solutions, ConflictSolution{
		ID:          "find_alternative",
		Type:        SolutionTypeConfigure,
		Description: fmt.Sprintf("Find alternative plugin for %s", dep.ID),
		Actions: []SolutionAction{
			{
				Type:        "search_alternative",
				Target:      dep.ID,
				Description: "Search for compatible alternative plugins",
			},
		},
		Risk:     SolutionRiskMedium,
		Priority: 2,
	})

	// 方案3: 移除依赖此插件的插件
	solutions = append(solutions, ConflictSolution{
		ID:          "remove_dependent",
		Type:        SolutionTypeRemove,
		Description: fmt.Sprintf("Remove plugin %s that requires missing dependency", pluginID),
		Actions: []SolutionAction{
			{
				Type:        "remove",
				Target:      pluginID,
				Description: fmt.Sprintf("Remove plugin %s to avoid missing dependency", pluginID),
			},
		},
		Risk:     SolutionRiskHigh,
		Priority: 3,
	})

	return solutions
}

// ResolveConflicts 解决依赖冲突
func (cr *DefaultConflictResolver) ResolveConflicts(conflicts []DependencyConflict) (*ConflictResolution, error) {
	if len(conflicts) == 0 {
		return &ConflictResolution{
			Summary: "No conflicts to resolve",
			Risk:    SolutionRiskLow,
		}, nil
	}

	resolution := &ConflictResolution{
		Actions: make([]ResolutionAction, 0),
		Risk:    SolutionRiskLow,
	}

	// 按严重程度排序冲突
	sort.Slice(conflicts, func(i, j int) bool {
		return cr.getSeverityWeight(conflicts[i].Severity) > cr.getSeverityWeight(conflicts[j].Severity)
	})

	for _, conflict := range conflicts {
		// 选择最佳解决方案
		solution := cr.selectBestSolution(conflict)
		if solution != nil {
			resolution.ResolvedConflicts = append(resolution.ResolvedConflicts, conflict.ID)
			resolution.Actions = append(resolution.Actions, ResolutionAction{
				ConflictID: conflict.ID,
				SolutionID: solution.ID,
				Actions:    solution.Actions,
				Status:     ActionStatusPending,
			})
		} else {
			resolution.RemainingConflicts = append(resolution.RemainingConflicts, conflict.ID)
		}
	}

	// 更新整体风险等级
	resolution.Risk = cr.calculateOverallRisk(resolution.Actions)
	resolution.Summary = cr.generateResolutionSummary(resolution)

	return resolution, nil
}

// selectBestSolution 选择最佳解决方案
func (cr *DefaultConflictResolver) selectBestSolution(conflict DependencyConflict) *ConflictSolution {
	if len(conflict.Solutions) == 0 {
		return nil
	}

	// 按优先级和风险排序
	sort.Slice(conflict.Solutions, func(i, j int) bool {
		if conflict.Solutions[i].Priority != conflict.Solutions[j].Priority {
			return conflict.Solutions[i].Priority < conflict.Solutions[j].Priority
		}
		return cr.getRiskWeight(conflict.Solutions[i].Risk) < cr.getRiskWeight(conflict.Solutions[j].Risk)
	})

	return &conflict.Solutions[0]
}

// getSeverityWeight 获取严重程度权重
func (cr *DefaultConflictResolver) getSeverityWeight(severity ConflictSeverity) int {
	switch severity {
	case ConflictSeverityCritical:
		return 5
	case ConflictSeverityHigh:
		return 4
	case ConflictSeverityMedium:
		return 3
	case ConflictSeverityLow:
		return 2
	case ConflictSeverityInfo:
		return 1
	default:
		return 0
	}
}

// getRiskWeight 获取风险权重
func (cr *DefaultConflictResolver) getRiskWeight(risk SolutionRisk) int {
	switch risk {
	case SolutionRiskHigh:
		return 3
	case SolutionRiskMedium:
		return 2
	case SolutionRiskLow:
		return 1
	default:
		return 0
	}
}

// calculateOverallRisk 计算整体风险
func (cr *DefaultConflictResolver) calculateOverallRisk(actions []ResolutionAction) SolutionRisk {
	if len(actions) == 0 {
		return SolutionRiskLow
	}

	highRiskCount := 0
	mediumRiskCount := 0

	for range actions {
		// 这里需要根据解决方案ID查找风险等级
		// 简化实现，假设所有动作都是中等风险
		mediumRiskCount++
	}

	if highRiskCount > 0 {
		return SolutionRiskHigh
	}
	if mediumRiskCount > 0 {
		return SolutionRiskMedium
	}
	return SolutionRiskLow
}

// generateResolutionSummary 生成解决方案摘要
func (cr *DefaultConflictResolver) generateResolutionSummary(resolution *ConflictResolution) string {
	total := len(resolution.ResolvedConflicts) + len(resolution.RemainingConflicts)
	resolved := len(resolution.ResolvedConflicts)

	if total == 0 {
		return "No conflicts found"
	}

	if resolved == total {
		return fmt.Sprintf("All %d conflicts resolved successfully", total)
	}

	return fmt.Sprintf("Resolved %d out of %d conflicts (%d remaining)", resolved, total, len(resolution.RemainingConflicts))
}

// SuggestAlternatives 建议替代方案
func (cr *DefaultConflictResolver) SuggestAlternatives(conflict DependencyConflict, availablePlugins map[string][]Plugin) []ConflictAlternative {
	var alternatives []ConflictAlternative

	switch conflict.Type {
	case ConflictTypeVersion:
		alternatives = cr.suggestVersionAlternatives(conflict, availablePlugins)
	case ConflictTypeMissing:
		alternatives = cr.suggestReplacementAlternatives(conflict, availablePlugins)
	}

	return alternatives
}

// suggestVersionAlternatives 建议版本替代方案
func (cr *DefaultConflictResolver) suggestVersionAlternatives(conflict DependencyConflict, availablePlugins map[string][]Plugin) []ConflictAlternative {
	var alternatives []ConflictAlternative

	// 分析版本冲突，寻找兼容版本
	for _, detail := range conflict.Details {
		if detail.DependencyID != "" {
			// 查找可用的兼容版本
			if plugins, exists := availablePlugins[detail.DependencyID]; exists {
				for range plugins {
					// 这里需要实现版本兼容性检查
					// 简化实现：假设所有可用版本都是兼容的
					alternatives = append(alternatives, ConflictAlternative{
						PluginID:      detail.DependencyID,
						Name:          detail.DependencyID,
						Version:       "latest", // 实际实现中应该获取真实版本
						Description:   fmt.Sprintf("Alternative version for %s", detail.DependencyID),
						Compatibility: 0.8, // 假设兼容性评分
						Risk:          SolutionRiskLow,
					})
				}
			}
		}
	}

	return alternatives
}

// suggestReplacementAlternatives 建议替换替代方案
func (cr *DefaultConflictResolver) suggestReplacementAlternatives(conflict DependencyConflict, availablePlugins map[string][]Plugin) []ConflictAlternative {
	var alternatives []ConflictAlternative

	// 分析缺失依赖，寻找替代插件
	for _, detail := range conflict.Details {
		if detail.DependencyID != "" {
			// 查找功能相似的替代插件
			for pluginType, plugins := range availablePlugins {
				// 这里需要实现功能相似性检查
				// 简化实现：假设所有可用插件都是潜在的替代方案
				if len(plugins) > 0 {
					alternatives = append(alternatives, ConflictAlternative{
						PluginID:      pluginType,
						Name:          pluginType,
						Version:       "latest", // 实际实现中应该获取真实版本
						Description:   fmt.Sprintf("Alternative plugin for %s", detail.DependencyID),
						Compatibility: 0.6, // 假设兼容性评分较低，因为是替代方案
						Risk:          SolutionRiskMedium,
					})
				}
			}
		}
	}

	return alternatives
}

// ValidateResolution 验证冲突解决方案
func (cr *DefaultConflictResolver) ValidateResolution(resolution *ConflictResolution, graph *DependencyGraph) error {
	// 检查是否还有循环依赖
	if cycle, err := graph.CheckCircularDependencies(); err != nil {
		return fmt.Errorf("circular dependency still exists after resolution: %s", strings.Join(cycle, " -> "))
	}

	// 检查是否还有版本冲突
	versionConflicts, err := graph.CheckVersionConflicts()
	if err != nil {
		return fmt.Errorf("failed to check version conflicts: %w", err)
	}

	if len(versionConflicts) > 0 {
		return fmt.Errorf("version conflicts still exist after resolution: %d conflicts remaining", len(versionConflicts))
	}

	return nil
}

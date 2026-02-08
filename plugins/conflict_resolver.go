package plugins

import (
	"fmt"
	"sort"
	"strings"
)

// ConflictResolver dependency conflict resolver interface.
//
// ResolveConflicts only produces a ConflictResolution (suggested actions); it does not modify
// the DependencyGraph. Callers must apply the chosen actions (e.g. change plugin set or versions)
// and then re-build or update the graph. ValidateResolution checks the current graph state after
// those changes; it does not apply the resolution itself.
type ConflictResolver interface {
	// DetectConflicts detects all dependency conflicts
	DetectConflicts(graph *DependencyGraph) ([]DependencyConflict, error)
	// ResolveConflicts returns suggested actions to resolve conflicts; it does not modify the graph
	ResolveConflicts(conflicts []DependencyConflict) (*ConflictResolution, error)
	// SuggestAlternatives suggests alternative solutions
	SuggestAlternatives(conflict DependencyConflict, availablePlugins map[string][]Plugin) []ConflictAlternative
	// ValidateResolution validates that the current graph has no conflicts (e.g. after applying resolution elsewhere)
	ValidateResolution(resolution *ConflictResolution, graph *DependencyGraph) error
}

// DependencyConflict dependency conflict information
type DependencyConflict struct {
	ID          string             `json:"id"`
	Type        ConflictType       `json:"type"`
	Severity    ConflictSeverity   `json:"severity"`
	Description string             `json:"description"`
	Plugins     []string           `json:"plugins"`
	Details     []ConflictDetail   `json:"details"`
	Solutions   []ConflictSolution `json:"solutions"`
}

// ConflictType conflict type
type ConflictType string

const (
	// ConflictTypeVersion version conflict
	ConflictTypeVersion ConflictType = "version"
	// ConflictTypeCircular circular dependency conflict
	ConflictTypeCircular ConflictType = "circular"
	// ConflictTypeMissing missing dependency conflict
	ConflictTypeMissing ConflictType = "missing"
	// ConflictTypeIncompatible incompatible conflict
	ConflictTypeIncompatible ConflictType = "incompatible"
	// ConflictTypeResource resource conflict
	ConflictTypeResource ConflictType = "resource"
)

// ConflictSeverity conflict severity level
type ConflictSeverity string

const (
	// ConflictSeverityCritical critical conflict
	ConflictSeverityCritical ConflictSeverity = "critical"
	// ConflictSeverityHigh high priority conflict
	ConflictSeverityHigh ConflictSeverity = "high"
	// ConflictSeverityMedium medium priority conflict
	ConflictSeverityMedium ConflictSeverity = "medium"
	// ConflictSeverityLow low priority conflict
	ConflictSeverityLow ConflictSeverity = "low"
	// ConflictSeverityInfo informational conflict
	ConflictSeverityInfo ConflictSeverity = "info"
)

// ConflictDetail conflict detailed information
type ConflictDetail struct {
	PluginID       string `json:"plugin_id"`
	DependencyID   string `json:"dependency_id"`
	RequiredValue  string `json:"required_value"`
	AvailableValue string `json:"available_value"`
	Message        string `json:"message"`
}

// ConflictSolution conflict resolution
type ConflictSolution struct {
	ID          string           `json:"id"`
	Type        SolutionType     `json:"type"`
	Description string           `json:"description"`
	Actions     []SolutionAction `json:"actions"`
	Risk        SolutionRisk     `json:"risk"`
	Priority    int              `json:"priority"`
}

// SolutionType solution type
type SolutionType string

const (
	// SolutionTypeUpgrade upgrade version
	SolutionTypeUpgrade SolutionType = "upgrade"
	// SolutionTypeDowngrade downgrade version
	SolutionTypeDowngrade SolutionType = "downgrade"
	// SolutionTypeReplace replace plugin
	SolutionTypeReplace SolutionType = "replace"
	// SolutionTypeRemove remove plugin
	SolutionTypeRemove SolutionType = "remove"
	// SolutionTypeConfigure configuration adjustment
	SolutionTypeConfigure SolutionType = "configure"
)

// SolutionAction solution action
type SolutionAction struct {
	Type        string            `json:"type"`
	Target      string            `json:"target"`
	Value       string            `json:"value"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters"`
}

// SolutionRisk solution risk
type SolutionRisk string

const (
	// SolutionRiskLow low risk
	SolutionRiskLow SolutionRisk = "low"
	// SolutionRiskMedium medium risk
	SolutionRiskMedium SolutionRisk = "medium"
	// SolutionRiskHigh high risk
	SolutionRiskHigh SolutionRisk = "high"
)

// ConflictAlternative conflict alternative solution
type ConflictAlternative struct {
	PluginID      string       `json:"plugin_id"`
	Name          string       `json:"name"`
	Version       string       `json:"version"`
	Description   string       `json:"description"`
	Compatibility float64      `json:"compatibility"` // Compatibility score 0-1
	Risk          SolutionRisk `json:"risk"`
}

// ConflictResolution conflict resolution
type ConflictResolution struct {
	ResolvedConflicts  []string           `json:"resolved_conflicts"`
	RemainingConflicts []string           `json:"remaining_conflicts"`
	Actions            []ResolutionAction `json:"actions"`
	Summary            string             `json:"summary"`
	Risk               SolutionRisk       `json:"risk"`
}

// ResolutionAction resolution action
type ResolutionAction struct {
	ConflictID string           `json:"conflict_id"`
	SolutionID string           `json:"solution_id"`
	Actions    []SolutionAction `json:"actions"`
	Status     ActionStatus     `json:"status"`
}

// ActionStatus action status
type ActionStatus string

const (
	// ActionStatusPending pending execution
	ActionStatusPending ActionStatus = "pending"
	// ActionStatusInProgress in progress
	ActionStatusInProgress ActionStatus = "in_progress"
	// ActionStatusCompleted completed
	ActionStatusCompleted ActionStatus = "completed"
	// ActionStatusFailed execution failed
	ActionStatusFailed ActionStatus = "failed"
	// ActionStatusRollback rolled back
	ActionStatusRollback ActionStatus = "rollback"
)

// DefaultConflictResolver default conflict resolver implementation
type DefaultConflictResolver struct {
	versionManager VersionManager
}

// NewConflictResolver creates a new conflict resolver
func NewConflictResolver(versionManager VersionManager) ConflictResolver {
	return &DefaultConflictResolver{
		versionManager: versionManager,
	}
}

// DetectConflicts detects all dependency conflicts
func (cr *DefaultConflictResolver) DetectConflicts(graph *DependencyGraph) ([]DependencyConflict, error) {
	var conflicts []DependencyConflict

	// Detect circular dependencies
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

	// Detect version conflicts
	versionConflicts, err := graph.CheckVersionConflicts()
	if err != nil {
		return nil, fmt.Errorf("failed to check version conflicts: %w", err)
	}

	if len(versionConflicts) > 0 {
		conflicts = append(conflicts, cr.convertVersionConflicts(versionConflicts)...)
	}

	// Detect missing dependencies
	missingConflicts := cr.detectMissingDependencies(graph)
	conflicts = append(conflicts, missingConflicts...)

	// Detect resource conflicts
	resourceConflicts := cr.detectResourceConflicts(graph)
	conflicts = append(conflicts, resourceConflicts...)

	return conflicts, nil
}

// convertVersionConflicts converts version conflicts
func (cr *DefaultConflictResolver) convertVersionConflicts(versionConflicts []VersionConflict) []DependencyConflict {
	var conflicts []DependencyConflict

	// Group version conflicts by plugin
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

// detectMissingDependencies detects missing dependencies
func (cr *DefaultConflictResolver) detectMissingDependencies(graph *DependencyGraph) []DependencyConflict {
	var conflicts []DependencyConflict

	// Traverse all plugin dependencies
	for pluginID, deps := range graph.GetAllDependencies() {
		for _, dep := range deps {
			if dep.Type == DependencyTypeRequired {
				// Check if the dependent plugin exists
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

// detectResourceConflicts detects resource conflicts
func (cr *DefaultConflictResolver) detectResourceConflicts(graph *DependencyGraph) []DependencyConflict {
	var conflicts []DependencyConflict

	// Here we need to check for resource name conflicts
	// Since there is no direct resource registration information in the current architecture, we provide a framework implementation
	// In actual use, the Plugin interface needs to be extended to support resource registration information

	// Check for plugin name conflicts
	pluginNames := make(map[string][]string)
	for pluginID, plugin := range graph.GetAllPlugins() {
		if plugin != nil {
			// Assume the plugin has a Name() method, if not, the interface needs to be extended
			// Here we use the plugin ID as a substitute for the name
			name := pluginID
			pluginNames[name] = append(pluginNames[name], pluginID)
		}
	}

	// Detect name conflicts
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

func (cr *DefaultConflictResolver) generateResourceConflictSolutions(name string, pluginIDs []string) []ConflictSolution {
	var solutions []ConflictSolution

	// Solution 1: Rename plugins
	solutions = append(solutions, ConflictSolution{
		ID:          "rename_plugin",
		Type:        SolutionTypeConfigure,
		Description: "Rename plugins to avoid name conflict: " + name,
		Actions: []SolutionAction{
			{
				Type:        "rename",
				Target:      strings.Join(pluginIDs, ","),
				Description: "Rename plugins to have unique names",
			},
		},
		Risk:     SolutionRiskLow,
		Priority: 1,
	})

	// Solution 2: Remove duplicate plugins
	solutions = append(solutions, ConflictSolution{
		ID:          "remove_duplicate",
		Type:        SolutionTypeRemove,
		Description: "Remove duplicate plugins with name: " + name,
		Actions: []SolutionAction{
			{
				Type:        "remove",
				Target:      strings.Join(pluginIDs[1:], ","),
				Description: "Keep first plugin, remove others with same name",
			},
		},
		Risk:     SolutionRiskMedium,
		Priority: 2,
	})

	// Solution 3: Merge plugins
	solutions = append(solutions, ConflictSolution{
		ID:          "merge_plugins",
		Type:        SolutionTypeConfigure,
		Description: "Merge plugins with same name: " + name,
		Actions: []SolutionAction{
			{
				Type:        "merge",
				Target:      strings.Join(pluginIDs, ","),
				Description: "Merge functionality of duplicate plugins",
			},
		},
		Risk:     SolutionRiskHigh,
		Priority: 3,
	})

	return solutions
}
func (cr *DefaultConflictResolver) generateCircularDependencySolutions(cycle []string) []ConflictSolution {
	var solutions []ConflictSolution

	// Solution 1: Remove one of the dependencies
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

	// Solution 2: Restructure dependencies
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

// generateVersionConflictSolutions generates version conflict solutions
func (cr *DefaultConflictResolver) generateVersionConflictSolutions(conflicts []VersionConflict) []ConflictSolution {
	var solutions []ConflictSolution

	// Solution 1: Upgrade to compatible version
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

	// Solution 2: Downgrade to compatible version
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

// generateMissingDependencySolutions generates solutions for missing dependencies
func (cr *DefaultConflictResolver) generateMissingDependencySolutions(pluginID string, dep *Dependency) []ConflictSolution {
	var solutions []ConflictSolution

	// Solution 1: Install missing dependency
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

	// Solution 2: Find alternative plugin
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

	// Solution 3: Remove plugin that depends on this
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

// ResolveConflicts resolves dependency conflicts
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

	// Sort conflicts by severity
	sort.Slice(conflicts, func(i, j int) bool {
		return cr.getSeverityWeight(conflicts[i].Severity) > cr.getSeverityWeight(conflicts[j].Severity)
	})

	for _, conflict := range conflicts {
		// Select the best solution
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

	// Update overall risk level
	resolution.Risk = cr.calculateOverallRisk(resolution.Actions)
	resolution.Summary = cr.generateResolutionSummary(resolution)

	return resolution, nil
}

// selectBestSolution selects the best solution
func (cr *DefaultConflictResolver) selectBestSolution(conflict DependencyConflict) *ConflictSolution {
	if len(conflict.Solutions) == 0 {
		return nil
	}

	// Sort by priority and risk
	sort.Slice(conflict.Solutions, func(i, j int) bool {
		if conflict.Solutions[i].Priority != conflict.Solutions[j].Priority {
			return conflict.Solutions[i].Priority < conflict.Solutions[j].Priority
		}
		return cr.getRiskWeight(conflict.Solutions[i].Risk) < cr.getRiskWeight(conflict.Solutions[j].Risk)
	})

	return &conflict.Solutions[0]
}

// getSeverityWeight gets severity weight
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

// getRiskWeight gets risk weight
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

// calculateOverallRisk calculates overall risk
func (cr *DefaultConflictResolver) calculateOverallRisk(actions []ResolutionAction) SolutionRisk {
	if len(actions) == 0 {
		return SolutionRiskLow
	}

	highRiskCount := 0
	mediumRiskCount := 0

	for range actions {
		// Here we need to look up the risk level by solution ID
		// Simplified implementation, assuming all actions are medium risk
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

// generateResolutionSummary generates resolution summary
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

// SuggestAlternatives suggests alternative solutions
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

// suggestVersionAlternatives suggests version alternative solutions
func (cr *DefaultConflictResolver) suggestVersionAlternatives(conflict DependencyConflict, availablePlugins map[string][]Plugin) []ConflictAlternative {
	var alternatives []ConflictAlternative

	// Analyze version conflicts and look for compatible versions
	for _, detail := range conflict.Details {
		if detail.DependencyID != "" {
			// Look for available compatible versions
			if plugins, exists := availablePlugins[detail.DependencyID]; exists {
				for range plugins {
					// Here we need to implement version compatibility checking
					// Simplified implementation: assume all available versions are compatible
					alternatives = append(alternatives, ConflictAlternative{
						PluginID:      detail.DependencyID,
						Name:          detail.DependencyID,
						Version:       "latest", // In actual implementation, should get real version
						Description:   fmt.Sprintf("Alternative version for %s", detail.DependencyID),
						Compatibility: 0.8, // Assume compatibility score
						Risk:          SolutionRiskLow,
					})
				}
			}
		}
	}

	return alternatives
}

// suggestReplacementAlternatives suggests replacement alternative solutions
func (cr *DefaultConflictResolver) suggestReplacementAlternatives(conflict DependencyConflict, availablePlugins map[string][]Plugin) []ConflictAlternative {
	var alternatives []ConflictAlternative

	// Analyze missing dependencies and look for alternative plugins
	for _, detail := range conflict.Details {
		if detail.DependencyID != "" {
			// Look for functionally similar alternative plugins
			for pluginType, plugins := range availablePlugins {
				// Here we need to implement functional similarity checking
				// Simplified implementation: assume all available plugins are potential alternatives
				if len(plugins) > 0 {
					alternatives = append(alternatives, ConflictAlternative{
						PluginID:      pluginType,
						Name:          pluginType,
						Version:       "latest", // In actual implementation, should get real version
						Description:   fmt.Sprintf("Alternative plugin for %s", detail.DependencyID),
						Compatibility: 0.6, // Assume lower compatibility score as it's an alternative
						Risk:          SolutionRiskMedium,
					})
				}
			}
		}
	}

	return alternatives
}

// ValidateResolution validates conflict resolution
func (cr *DefaultConflictResolver) ValidateResolution(resolution *ConflictResolution, graph *DependencyGraph) error {
	// Check if there are still circular dependencies
	if cycle, err := graph.CheckCircularDependencies(); err != nil {
		return fmt.Errorf("circular dependency still exists after resolution: %s", strings.Join(cycle, " -> "))
	}

	// Check if there are still version conflicts
	versionConflicts, err := graph.CheckVersionConflicts()
	if err != nil {
		return fmt.Errorf("failed to check version conflicts: %w", err)
	}

	if len(versionConflicts) > 0 {
		return fmt.Errorf("version conflicts still exist after resolution: %d conflicts remaining", len(versionConflicts))
	}

	return nil
}

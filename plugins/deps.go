package plugins

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	semver "github.com/Masterminds/semver/v3"
)

// DependencyType defines dependency types
type DependencyType string

const (
	// DependencyTypeRequired required dependency
	DependencyTypeRequired DependencyType = "required"
	// DependencyTypeOptional optional dependency
	DependencyTypeOptional DependencyType = "optional"
	// DependencyTypeConflicts conflicting dependency
	DependencyTypeConflicts DependencyType = "conflicts"
	// DependencyTypeProvides provided dependency
	DependencyTypeProvides DependencyType = "provides"
)

// VersionConstraint version constraint
type VersionConstraint struct {
	MinVersion      string   `json:"min_version"`      // Minimum version
	MaxVersion      string   `json:"max_version"`      // Maximum version
	ExactVersion    string   `json:"exact_version"`    // Exact version
	ExcludeVersions []string `json:"exclude_versions"` // Excluded versions
}

// Dependency describes dependency relationships between plugins
type Dependency struct {
	ID                string             `json:"id"`                 // Unique identifier of the dependent plugin
	Name              string             `json:"name"`               // Name of the dependent plugin
	Type              DependencyType     `json:"type"`               // Dependency type
	VersionConstraint *VersionConstraint `json:"version_constraint"` // Version constraint
	Required          bool               `json:"required"`           // Whether it's a required dependency
	Checker           DependencyChecker  `json:"-"`                  // Dependency validator
	Metadata          map[string]any     `json:"metadata"`           // Additional dependency information
	Description       string             `json:"description"`        // Dependency description
}

// DependencyChecker defines the interface for dependency validation
type DependencyChecker interface {
	// Check validates whether dependency conditions are met
	Check(plugin Plugin) bool
	// Description returns a human-readable description of the condition
	Description() string
}

// DependencyManager dependency manager interface
type DependencyManager interface {
	// AddDependency adds a dependency relationship
	AddDependency(pluginID string, dependency *Dependency) error
	// RemoveDependency removes a dependency relationship
	RemoveDependency(pluginID string, dependencyID string) error
	// GetDependencies gets all dependencies of a plugin
	GetDependencies(pluginID string) []*Dependency
	// GetDependents gets all plugins that depend on this plugin
	GetDependents(pluginID string) []string
	// CheckCircularDependencies checks for circular dependencies
	CheckCircularDependencies() ([]string, error)
	// ResolveDependencies resolves dependency relationships and returns the correct loading order
	ResolveDependencies() ([]string, error)
	// CheckVersionConflicts checks for version conflicts
	CheckVersionConflicts() ([]VersionConflict, error)
	// ValidateDependencies validates whether all dependencies are satisfied
	ValidateDependencies(plugins map[string]Plugin) ([]DependencyError, error)
}

// VersionConflict version conflict information
type VersionConflict struct {
	PluginID         string `json:"plugin_id"`
	DependencyID     string `json:"dependency_id"`
	RequiredVersion  string `json:"required_version"`
	AvailableVersion string `json:"available_version"`
	ConflictType     string `json:"conflict_type"`
	Description      string `json:"description"`
}

// DependencyError dependency error information
type DependencyError struct {
	PluginID     string `json:"plugin_id"`
	DependencyID string `json:"dependency_id"`
	ErrorType    string `json:"error_type"`
	Message      string `json:"message"`
	Severity     string `json:"severity"` // "error", "warning", "info"
}

// DependencyGraph dependency graph structure
type DependencyGraph struct {
	// Plugin ID -> dependency list
	dependencies map[string][]*Dependency
	// Plugin ID -> list of plugins that depend on it
	dependents map[string][]string
	// Plugin ID -> plugin information
	plugins map[string]Plugin
	// Mutex
	mu sync.RWMutex
}

// NewDependencyGraph creates a new dependency graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		dependencies: make(map[string][]*Dependency),
		dependents:   make(map[string][]string),
		plugins:      make(map[string]Plugin),
	}
}

// AddPlugin adds a plugin to the dependency graph
func (dg *DependencyGraph) AddPlugin(plugin Plugin) {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	pluginID := plugin.ID()
	dg.plugins[pluginID] = plugin
}

// RemovePlugin removes a plugin from the dependency graph
func (dg *DependencyGraph) RemovePlugin(pluginID string) {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	delete(dg.plugins, pluginID)
	delete(dg.dependencies, pluginID)

	// Remove from all dependent lists
	delete(dg.dependents, pluginID)
	for id, deps := range dg.dependents {
		for i, dep := range deps {
			if dep == pluginID {
				dg.dependents[id] = append(deps[:i], deps[i+1:]...)
				break
			}
		}
	}
}

// AddDependency adds a dependency relationship
func (dg *DependencyGraph) AddDependency(pluginID string, dependency *Dependency) error {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	// Check if plugin exists
	if _, exists := dg.plugins[pluginID]; !exists {
		return fmt.Errorf("plugin %s not found", pluginID)
	}

	// Check if dependent plugin exists
	if _, exists := dg.plugins[dependency.ID]; !exists {
		return fmt.Errorf("dependency plugin %s not found", dependency.ID)
	}

	// Add dependency relationship
	dg.dependencies[pluginID] = append(dg.dependencies[pluginID], dependency)

	// Update dependent relationship
	dg.dependents[dependency.ID] = append(dg.dependents[dependency.ID], pluginID)

	return nil
}

// RemoveDependency removes a dependency relationship
func (dg *DependencyGraph) RemoveDependency(pluginID string, dependencyID string) error {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	deps, exists := dg.dependencies[pluginID]
	if !exists {
		return fmt.Errorf("plugin %s has no dependencies", pluginID)
	}

	// Remove all occurrences of the dependency from the list
	filtered := deps[:0]
	removed := 0
	for _, dep := range deps {
		if dep != nil && dep.ID == dependencyID {
			removed++
			continue
		}
		filtered = append(filtered, dep)
	}

	if removed == 0 {
		return fmt.Errorf("dependency %s not found for plugin %s", dependencyID, pluginID)
	}

	if len(filtered) == 0 {
		delete(dg.dependencies, pluginID)
	} else {
		dg.dependencies[pluginID] = filtered
	}

	// Update dependent relationship: remove all occurrences of pluginID
	if dependents, ok := dg.dependents[dependencyID]; ok {
		newDeps := dependents[:0]
		for _, d := range dependents {
			if d != pluginID {
				newDeps = append(newDeps, d)
			}
		}
		if len(newDeps) == 0 {
			delete(dg.dependents, dependencyID)
		} else {
			dg.dependents[dependencyID] = newDeps
		}
	}
	return nil
}

// GetDependencies gets all dependencies of a plugin
func (dg *DependencyGraph) GetDependencies(pluginID string) []*Dependency {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	if deps, exists := dg.dependencies[pluginID]; exists {
		result := make([]*Dependency, len(deps))
		copy(result, deps)
		return result
	}
	return nil
}

// GetDependents gets all plugins that depend on this plugin
func (dg *DependencyGraph) GetDependents(pluginID string) []string {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	if deps, exists := dg.dependents[pluginID]; exists {
		result := make([]string, len(deps))
		copy(result, deps)
		return result
	}

	return nil
}

// CheckCircularDependencies checks for circular dependencies
func (dg *DependencyGraph) CheckCircularDependencies() ([]string, error) {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	cycle := make([]string, 0)

	// Depth-first search to detect cycles
	var dfs func(pluginID string) bool
	dfs = func(pluginID string) bool {
		if recStack[pluginID] {
			// Found circular dependency
			cycle = append(cycle, pluginID)
			return true
		}

		if visited[pluginID] {
			return false
		}

		visited[pluginID] = true
		recStack[pluginID] = true

		deps := dg.dependencies[pluginID]
		for _, dep := range deps {
			if dep.Type == DependencyTypeRequired {
				if dfs(dep.ID) {
					if len(cycle) == 1 || cycle[len(cycle)-1] != pluginID {
						cycle = append(cycle, pluginID)
					}
					return true
				}
			}
		}

		recStack[pluginID] = false
		return false
	}

	// Check all plugins
	for id := range dg.plugins {
		if !visited[id] {
			if dfs(id) {
				// Reverse cycle path
				for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
					cycle[i], cycle[j] = cycle[j], cycle[i]
				}
				return cycle, fmt.Errorf("circular dependency detected: %s", strings.Join(cycle, " -> "))
			}
		}
	}

	return nil, nil
}

// ResolveDependencies resolves dependency relationships and returns the correct loading order
func (dg *DependencyGraph) ResolveDependencies() ([]string, error) {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	// First check for circular dependencies
	if _, err := dg.CheckCircularDependencies(); err != nil {
		return nil, err
	}

	// Topological sort
	inDegree := make(map[string]int)
	graph := make(map[string][]string)

	// Initialize in-degrees
	for pluginID := range dg.plugins {
		inDegree[pluginID] = 0
	}

	// Build graph and calculate in-degrees
	// Use edges: dependency -> plugin (so that dependencies are before dependents)
	for pluginID, deps := range dg.dependencies {
		for _, dep := range deps {
			if dep.Type == DependencyTypeRequired {
				graph[dep.ID] = append(graph[dep.ID], pluginID)
				inDegree[pluginID]++
			}
		}
	}

	// Topological sort
	var result []string
	queue := make([]string, 0)

	// Find all nodes with in-degree 0 (no required dependencies)
	for pluginID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, pluginID)
		}
	}

	// Process queue
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Update in-degrees of nodes that depend on current
		for _, next := range graph[current] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	// Check if all nodes were processed
	if len(result) != len(dg.plugins) {
		return nil, fmt.Errorf("dependency resolution failed: some plugins have unresolved dependencies")
	}

	return result, nil
}

// CheckVersionConflicts checks for version conflicts
func (dg *DependencyGraph) CheckVersionConflicts() ([]VersionConflict, error) {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	var conflicts []VersionConflict

	for pluginID, deps := range dg.dependencies {
		plugin := dg.plugins[pluginID]
		if plugin == nil {
			continue
		}

		for _, dep := range deps {
			if dep.VersionConstraint == nil {
				continue
			}

			depPlugin := dg.plugins[dep.ID]
			if depPlugin == nil {
				continue
			}

			// Check version constraint
			if conflict := dg.checkVersionConstraint(plugin, dep, depPlugin); conflict != nil {
				conflicts = append(conflicts, *conflict)
			}
		}
	}

	return conflicts, nil
}

// checkVersionConstraint checks a single version constraint
func (dg *DependencyGraph) checkVersionConstraint(plugin Plugin, dep *Dependency, depPlugin Plugin) *VersionConflict {
	constraint := dep.VersionConstraint
	depVersionStr := depPlugin.Version()

	// Try parse dependency version as semver; if fail, fall back to string compares
	depVer, depErr := semver.NewVersion(normalizeSemver(depVersionStr))

	// Check exact version
	if constraint.ExactVersion != "" {
		exactVer, exactErr := semver.NewVersion(normalizeSemver(constraint.ExactVersion))
		if depErr == nil && exactErr == nil {
			if !depVer.Equal(exactVer) {
				return &VersionConflict{
					PluginID:         plugin.ID(),
					DependencyID:     dep.ID,
					RequiredVersion:  constraint.ExactVersion,
					AvailableVersion: depVersionStr,
					ConflictType:     "exact_version_mismatch",
					Description: fmt.Sprintf("Plugin %s requires exact version %s of %s, but %s is available",
						plugin.ID(), constraint.ExactVersion, dep.ID, depVersionStr),
				}
			}
		} else {
			if cmp, ok := compareVersionNumeric(depVersionStr, constraint.ExactVersion); ok {
				if cmp != 0 {
					return &VersionConflict{
						PluginID:         plugin.ID(),
						DependencyID:     dep.ID,
						RequiredVersion:  constraint.ExactVersion,
						AvailableVersion: depVersionStr,
						ConflictType:     "exact_version_mismatch",
						Description: fmt.Sprintf("Plugin %s requires exact version %s of %s, but %s is available",
							plugin.ID(), constraint.ExactVersion, dep.ID, depVersionStr),
					}
				}
			} else if constraint.ExactVersion != depVersionStr { // fallback
				return &VersionConflict{
					PluginID:         plugin.ID(),
					DependencyID:     dep.ID,
					RequiredVersion:  constraint.ExactVersion,
					AvailableVersion: depVersionStr,
					ConflictType:     "exact_version_mismatch",
					Description: fmt.Sprintf("Plugin %s requires exact version %s of %s, but %s is available",
						plugin.ID(), constraint.ExactVersion, dep.ID, depVersionStr),
				}
			}
		}
	}

	// Check excluded versions
	for _, excludedVersion := range constraint.ExcludeVersions {
		if excludedVersion == "" {
			continue
		}
		if depErr == nil {
			if exVer, exErr := semver.NewVersion(normalizeSemver(excludedVersion)); exErr == nil {
				if depVer.Equal(exVer) {
					return &VersionConflict{
						PluginID:         plugin.ID(),
						DependencyID:     dep.ID,
						RequiredVersion:  "any version except " + excludedVersion,
						AvailableVersion: depVersionStr,
						ConflictType:     "excluded_version",
						Description: fmt.Sprintf("Plugin %s excludes version %s of %s",
							plugin.ID(), excludedVersion, dep.ID),
					}
				}
			} else if cmp, ok := compareVersionNumeric(depVersionStr, excludedVersion); ok {
				if cmp == 0 {
					return &VersionConflict{
						PluginID:         plugin.ID(),
						DependencyID:     dep.ID,
						RequiredVersion:  "any version except " + excludedVersion,
						AvailableVersion: depVersionStr,
						ConflictType:     "excluded_version",
						Description: fmt.Sprintf("Plugin %s excludes version %s of %s",
							plugin.ID(), excludedVersion, dep.ID),
					}
				}
			} else if excludedVersion == depVersionStr { // fallback
				return &VersionConflict{
					PluginID:         plugin.ID(),
					DependencyID:     dep.ID,
					RequiredVersion:  "any version except " + excludedVersion,
					AvailableVersion: depVersionStr,
					ConflictType:     "excluded_version",
					Description: fmt.Sprintf("Plugin %s excludes version %s of %s",
						plugin.ID(), excludedVersion, dep.ID),
				}
			}
		} else if cmp, ok := compareVersionNumeric(depVersionStr, excludedVersion); ok {
			if cmp == 0 {
				return &VersionConflict{
					PluginID:         plugin.ID(),
					DependencyID:     dep.ID,
					RequiredVersion:  "any version except " + excludedVersion,
					AvailableVersion: depVersionStr,
					ConflictType:     "excluded_version",
					Description: fmt.Sprintf("Plugin %s excludes version %s of %s",
						plugin.ID(), excludedVersion, dep.ID),
				}
			}
		} else if excludedVersion == depVersionStr { // fallback
			return &VersionConflict{
				PluginID:         plugin.ID(),
				DependencyID:     dep.ID,
				RequiredVersion:  "any version except " + excludedVersion,
				AvailableVersion: depVersionStr,
				ConflictType:     "excluded_version",
				Description: fmt.Sprintf("Plugin %s excludes version %s of %s",
					plugin.ID(), excludedVersion, dep.ID),
			}
		}
	}

	// Check MinVersion
	if constraint.MinVersion != "" {
		if depErr == nil {
			if minVer, minErr := semver.NewVersion(normalizeSemver(constraint.MinVersion)); minErr == nil {
				if depVer.LessThan(minVer) {
					return &VersionConflict{
						PluginID:         plugin.ID(),
						DependencyID:     dep.ID,
						RequiredVersion:  ">= " + constraint.MinVersion,
						AvailableVersion: depVersionStr,
						ConflictType:     "version_too_low",
						Description: fmt.Sprintf("Plugin %s requires version >= %s of %s, but %s is available",
							plugin.ID(), constraint.MinVersion, dep.ID, depVersionStr),
					}
				}
			} else if depVersionStr < constraint.MinVersion { // fallback
				return &VersionConflict{
					PluginID:         plugin.ID(),
					DependencyID:     dep.ID,
					RequiredVersion:  ">= " + constraint.MinVersion,
					AvailableVersion: depVersionStr,
					ConflictType:     "version_too_low",
					Description: fmt.Sprintf("Plugin %s requires version >= %s of %s, but %s is available",
						plugin.ID(), constraint.MinVersion, dep.ID, depVersionStr),
				}
			}
		} else if depVersionStr < constraint.MinVersion { // fallback
			return &VersionConflict{
				PluginID:         plugin.ID(),
				DependencyID:     dep.ID,
				RequiredVersion:  ">= " + constraint.MinVersion,
				AvailableVersion: depVersionStr,
				ConflictType:     "version_too_low",
				Description: fmt.Sprintf("Plugin %s requires version >= %s of %s, but %s is available",
					plugin.ID(), constraint.MinVersion, dep.ID, depVersionStr),
			}
		}
	}

	// Check MaxVersion
	if constraint.MaxVersion != "" {
		if depErr == nil {
			if maxVer, maxErr := semver.NewVersion(normalizeSemver(constraint.MaxVersion)); maxErr == nil {
				if depVer.GreaterThan(maxVer) {
					return &VersionConflict{
						PluginID:         plugin.ID(),
						DependencyID:     dep.ID,
						RequiredVersion:  "<= " + constraint.MaxVersion,
						AvailableVersion: depVersionStr,
						ConflictType:     "version_too_high",
						Description: fmt.Sprintf("Plugin %s requires version <= %s of %s, but %s is available",
							plugin.ID(), constraint.MaxVersion, dep.ID, depVersionStr),
					}
				}
			} else if depVersionStr > constraint.MaxVersion { // fallback
				return &VersionConflict{
					PluginID:         plugin.ID(),
					DependencyID:     dep.ID,
					RequiredVersion:  "<= " + constraint.MaxVersion,
					AvailableVersion: depVersionStr,
					ConflictType:     "version_too_high",
					Description: fmt.Sprintf("Plugin %s requires version <= %s of %s, but %s is available",
						plugin.ID(), constraint.MaxVersion, dep.ID, depVersionStr),
				}
			}
		} else if depVersionStr > constraint.MaxVersion { // fallback
			return &VersionConflict{
				PluginID:         plugin.ID(),
				DependencyID:     dep.ID,
				RequiredVersion:  "<= " + constraint.MaxVersion,
				AvailableVersion: depVersionStr,
				ConflictType:     "version_too_high",
				Description: fmt.Sprintf("Plugin %s requires version <= %s of %s, but %s is available",
					plugin.ID(), constraint.MaxVersion, dep.ID, depVersionStr),
			}
		}
	}

	return nil
}

// normalizeSemver normalizes version string to semver-like form for better parsing robustness.
// It appends missing .0 components when only major or major.minor provided.
func normalizeSemver(ver string) string {
	s := strings.TrimSpace(ver)
	if s == "" {
		return s
	}
	t := s
	if strings.HasPrefix(t, "v") || strings.HasPrefix(t, "V") {
		t = t[1:]
	}
	dotCnt := strings.Count(t, ".")
	switch dotCnt {
	case 0:
		t += ".0.0"
	case 1:
		t += ".0"
	}
	return t
}

// compareVersionNumeric compares two version strings numerically.
func compareVersionNumeric(a, b string) (int, bool) {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := 0; i < len(aParts) || i < len(bParts); i++ {
		aPart, aOk := aParts[i], i < len(aParts)
		bPart, bOk := bParts[i], i < len(bParts)

		if !aOk {
			return -1, true
		}
		if !bOk {
			return 1, true
		}

		aNum, aErr := strconv.Atoi(aPart)
		bNum, bErr := strconv.Atoi(bPart)

		if aErr != nil || bErr != nil {
			return 0, false
		}

		if aNum != bNum {
			return aNum - bNum, true
		}
	}

	return 0, true
}

// ValidateDependencies validates whether all dependencies are satisfied
func (dg *DependencyGraph) ValidateDependencies(plugins map[string]Plugin) ([]DependencyError, error) {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	var errors []DependencyError

	for pluginID, deps := range dg.dependencies {
		plugin := dg.plugins[pluginID]
		if plugin == nil {
			continue
		}

		for _, dep := range deps {
			if dep.Type == DependencyTypeRequired {
				// Check if dependent plugin exists
				depPlugin, exists := plugins[dep.ID]
				if !exists {
					errors = append(errors, DependencyError{
						PluginID:     pluginID,
						DependencyID: dep.ID,
						ErrorType:    "missing_dependency",
						Message:      fmt.Sprintf("Required dependency %s not found", dep.ID),
						Severity:     "error",
					})
					continue
				}

				// Check version constraint
				if dep.VersionConstraint != nil {
					if conflict := dg.checkVersionConstraint(plugin, dep, depPlugin); conflict != nil {
						errors = append(errors, DependencyError{
							PluginID:     pluginID,
							DependencyID: dep.ID,
							ErrorType:    "version_conflict",
							Message:      conflict.Description,
							Severity:     "error",
						})
					}
				}

				// Check dependency checker
				if dep.Checker != nil {
					if !dep.Checker.Check(depPlugin) {
						errors = append(errors, DependencyError{
							PluginID:     pluginID,
							DependencyID: dep.ID,
							ErrorType:    "dependency_check_failed",
							Message:      fmt.Sprintf("Dependency check failed: %s", dep.Checker.Description()),
							Severity:     "error",
						})
					}
				}
			}
		}
	}

	return errors, nil
}

// GetDependencyTree gets the dependency tree structure
func (dg *DependencyGraph) GetDependencyTree(pluginID string) map[string]interface{} {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	return dg.buildDependencyTree(pluginID, make(map[string]bool))
}

// buildDependencyTree recursively builds the dependency tree
func (dg *DependencyGraph) buildDependencyTree(pluginID string, visited map[string]bool) map[string]interface{} {
	if visited[pluginID] {
		return map[string]interface{}{
			"id":       pluginID,
			"circular": true,
		}
	}

	visited[pluginID] = true
	defer delete(visited, pluginID)

	tree := map[string]interface{}{
		"id":           pluginID,
		"dependencies": []map[string]interface{}{},
	}

	if deps, exists := dg.dependencies[pluginID]; exists {
		for _, dep := range deps {
			depTree := dg.buildDependencyTree(dep.ID, visited)
			depInfo := map[string]interface{}{
				"id":          dep.ID,
				"type":        dep.Type,
				"required":    dep.Required,
				"description": dep.Description,
				"subtree":     depTree,
			}

			if dep.VersionConstraint != nil {
				depInfo["version_constraint"] = dep.VersionConstraint
			}

			tree["dependencies"] = append(tree["dependencies"].([]map[string]interface{}), depInfo)
		}
	}

	return tree
}

// GetDependencyStats gets dependency statistics
func (dg *DependencyGraph) GetDependencyStats() map[string]interface{} {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	stats := map[string]interface{}{
		"total_plugins":        len(dg.plugins),
		"total_dependencies":   0,
		"required_deps":        0,
		"optional_deps":        0,
		"conflict_deps":        0,
		"plugins_with_deps":    0,
		"plugins_without_deps": 0,
	}

	for _, deps := range dg.dependencies {
		if len(deps) > 0 {
			if count, ok := stats["plugins_with_deps"].(int); ok {
				stats["plugins_with_deps"] = count + 1
			}
			if total, ok := stats["total_dependencies"].(int); ok {
				stats["total_dependencies"] = total + len(deps)
			}

			for _, dep := range deps {
				switch dep.Type {
				case DependencyTypeRequired:
					if count, ok := stats["required_deps"].(int); ok {
						stats["required_deps"] = count + 1
					}
				case DependencyTypeOptional:
					if count, ok := stats["optional_deps"].(int); ok {
						stats["optional_deps"] = count + 1
					}
				case DependencyTypeConflicts:
					if count, ok := stats["conflict_deps"].(int); ok {
						stats["conflict_deps"] = count + 1
					}
				}
			}
		} else {
			if count, ok := stats["plugins_without_deps"].(int); ok {
				stats["plugins_without_deps"] = count + 1
			}
		}
	}

	return stats
}

// HasPlugin checks if a plugin exists
func (dg *DependencyGraph) HasPlugin(pluginID string) bool {
	dg.mu.RLock()
	defer dg.mu.RUnlock()
	_, exists := dg.plugins[pluginID]
	return exists
}

// GetAllDependencies gets all plugin dependency relationships
func (dg *DependencyGraph) GetAllDependencies() map[string][]*Dependency {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	// Create a copy to avoid external modifications
	result := make(map[string][]*Dependency)
	for pluginID, deps := range dg.dependencies {
		depsCopy := make([]*Dependency, len(deps))
		copy(depsCopy, deps)
		result[pluginID] = depsCopy
	}
	return result
}

// GetAllPlugins gets all plugins
func (dg *DependencyGraph) GetAllPlugins() map[string]Plugin {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	// Create a copy to avoid external modifications
	result := make(map[string]Plugin)
	for pluginID, plugin := range dg.plugins {
		result[pluginID] = plugin
	}
	return result
}

// CleanupOrphanedDependencies cleans up orphaned dependency relationships
func (dg *DependencyGraph) CleanupOrphanedDependencies() int {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	cleaned := 0

	// Check all dependency relationships and remove dependencies pointing to non-existent plugins
	for pluginID, deps := range dg.dependencies {
		validDeps := make([]*Dependency, 0)

		for _, dep := range deps {
			if _, exists := dg.plugins[dep.ID]; exists {
				validDeps = append(validDeps, dep)
			} else {
				cleaned++
			}
		}

		if len(validDeps) != len(deps) {
			dg.dependencies[pluginID] = validDeps
		}
	}

	// Clean up dependent relationships
	for depID, dependents := range dg.dependents {
		if _, exists := dg.plugins[depID]; !exists {
			delete(dg.dependents, depID)
			cleaned += len(dependents)
		}
	}

	return cleaned
}

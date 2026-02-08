// Package lynx provides the core application framework for building microservices.
//
// This file (topology.go) contains plugin dependency resolution utilities:
//   - TopologicalSort: Sort plugins based on dependencies
//   - Dependency graph construction and validation
//   - Level calculation for parallel plugin loading
package lynx

import (
	"fmt"
	"strings"

	"github.com/go-lynx/lynx/plugins"
)

// PluginWithLevel represents a plugin with its dependency level.
// The level indicates how deep in the dependency chain this plugin is,
// with 0 being plugins with no dependencies.
type PluginWithLevel struct {
	plugins.Plugin
	level int
}

// TopologicalSort sorts a subset of plugins based on required dependencies
// and calculates an integer level for each plugin.
// It performs a topological sort on the plugin dependency graph and assigns
// each plugin a level based on the maximum depth of its dependencies.
//
// Dependency timing: GetDependencies() is called on each plugin before any
// plugin is initialized. Required dependencies that affect load order must
// therefore be declared in the plugin constructor (or before this sort runs),
// not only in InitializeResources. See plugins package README "Dependency declaration timing".
//
// Parameters:
//   - plugs: slice of plugins to sort and calculate levels for
//
// Returns:
//   - []PluginWithLevel: sorted plugins with their dependency levels
//   - error: if there's a circular dependency or other resolution error
func (m *DefaultPluginManager[T]) TopologicalSort(plugs []plugins.Plugin) ([]PluginWithLevel, error) {
	// Return early if no plugins to process
	if len(plugs) == 0 {
		return nil, nil
	}

	// Create a map of plugin ID to plugin for quick lookup
	id2plugin := make(map[string]plugins.Plugin, len(plugs))
	for _, p := range plugs {
		if p == nil {
			continue
		}
		id := p.ID()
		if existing, exists := id2plugin[id]; exists {
			// Duplicate ID detected
			return nil, fmt.Errorf("duplicate plugin ID detected: %s (plugins: %s and %s)",
				id, existing.Name(), p.Name())
		}
		id2plugin[id] = p
	}

	// Check if we have any valid plugins
	if len(id2plugin) == 0 {
		return nil, fmt.Errorf("no valid plugins to sort (all plugins are nil)")
	}

	// Create a dependency graph to manage plugin relationships
	dg := plugins.NewDependencyGraph()
	for _, p := range id2plugin {
		dg.AddPlugin(p)
	}

	// Map to store required dependencies for each plugin
	requiredDeps := make(map[string][]string, len(id2plugin))
	// Track missing required dependencies
	missingDeps := make(map[string][]string) // pluginID -> []missingDepID

	// Process each plugin's dependencies
	for _, p := range id2plugin {
		deps := p.GetDependencies()
		if len(deps) == 0 {
			continue
		}

		// Process required dependencies
		for _, dep := range deps {
			if dep.Type != plugins.DependencyTypeRequired {
				continue
			}

			// Check if dependency exists in our plugin set
			if _, ok := id2plugin[dep.ID]; !ok {
				// Track missing required dependency
				missingDeps[p.ID()] = append(missingDeps[p.ID()], dep.ID)
				continue
			}

			d := dep
			// Add the dependency to the graph
			if err := dg.AddDependency(p.ID(), &d); err != nil {
				return nil, fmt.Errorf("failed to add dependency %s -> %s: %w", p.ID(), dep.ID, err)
			}
			// Track this required dependency
			requiredDeps[p.ID()] = append(requiredDeps[p.ID()], dep.ID)
		}
	}

	// Report missing required dependencies
	if len(missingDeps) > 0 {
		var missingMsgs []string
		for pluginID, missing := range missingDeps {
			missingMsgs = append(missingMsgs, fmt.Sprintf("plugin %s requires missing dependencies: [%s]",
				pluginID, strings.Join(missing, ", ")))
		}
		return nil, fmt.Errorf("missing required dependencies: %s", strings.Join(missingMsgs, "; "))
	}

	// Resolve the dependency order using topological sort
	orderedIDs, err := dg.ResolveDependencies()
	if err != nil {
		return nil, fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Validate completeness: ensure all plugins are in the ordered result
	if len(orderedIDs) != len(id2plugin) {
		orderedSet := make(map[string]bool, len(orderedIDs))
		// Also check for duplicates in orderedIDs
		var duplicates []string
		for _, id := range orderedIDs {
			if orderedSet[id] {
				duplicates = append(duplicates, id)
			}
			orderedSet[id] = true
		}

		var missing []string
		for id := range id2plugin {
			if !orderedSet[id] {
				missing = append(missing, id)
			}
		}

		errMsg := fmt.Sprintf("topological sort incomplete: %d plugins in result, %d expected",
			len(orderedIDs), len(id2plugin))
		if len(missing) > 0 {
			errMsg += fmt.Sprintf("; missing plugins: [%s]", strings.Join(missing, ", "))
		}
		if len(duplicates) > 0 {
			errMsg += fmt.Sprintf("; duplicate IDs in result: [%s]", strings.Join(duplicates, ", "))
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	// Memoization cache for calculated levels
	memo := make(map[string]int, len(id2plugin))
	// Tracking currently visiting nodes to detect cycles
	visiting := make(map[string]bool, len(id2plugin))
	// Stack to track DFS path for accurate cycle reporting
	pathStack := make([]string, 0, len(id2plugin))

	// Depth-first search function to calculate dependency level
	var dfs func(string) (int, error)
	dfs = func(id string) (int, error) {
		// Return cached result if already calculated
		if lv, ok := memo[id]; ok {
			return lv, nil
		}

		// Detect cycles - this should not happen if ResolveDependencies worked correctly,
		// but we check defensively
		if visiting[id] {
			// Build accurate cycle path from stack
			cycleStart := -1
			for i, nodeID := range pathStack {
				if nodeID == id {
					cycleStart = i
					break
				}
			}
			var cyclePath []string
			if cycleStart >= 0 {
				// Include the cycle portion of the path
				cyclePath = append(cyclePath, pathStack[cycleStart:]...)
				cyclePath = append(cyclePath, id) // Complete the cycle
			} else {
				// Fallback: include entire path
				cyclePath = append(cyclePath, pathStack...)
				cyclePath = append(cyclePath, id)
			}

			// Get plugin names for better error message
			var cycleNames []string
			for _, nodeID := range cyclePath {
				if p, ok := id2plugin[nodeID]; ok {
					cycleNames = append(cycleNames, fmt.Sprintf("%s(%s)", p.Name(), nodeID))
				} else {
					cycleNames = append(cycleNames, nodeID)
				}
			}

			return 0, fmt.Errorf("circular dependency detected in level calculation: %s",
				strings.Join(cycleNames, " -> "))
		}

		// Verify plugin exists (defensive check)
		p, ok := id2plugin[id]
		if !ok {
			return 0, fmt.Errorf("plugin %s not found in plugin set during level calculation", id)
		}

		visiting[id] = true
		pathStack = append(pathStack, id)
		defer func() {
			visiting[id] = false
			// Remove from path stack (find and remove, not just pop)
			// This handles cases where the stack might have been modified
			for i := len(pathStack) - 1; i >= 0; i-- {
				if pathStack[i] == id {
					pathStack = append(pathStack[:i], pathStack[i+1:]...)
					break
				}
			}
		}()

		// Calculate the maximum level from all dependencies
		best := 0
		// Get dependencies (may be nil if no deps, but range handles nil safely)
		for _, depID := range requiredDeps[id] {
			// Defensive check: ensure dependency ID exists
			if _, ok := id2plugin[depID]; !ok {
				return 0, fmt.Errorf("dependency %s of plugin %s (%s) not found in plugin set",
					depID, id, p.Name())
			}

			lv, err := dfs(depID)
			if err != nil {
				return 0, err
			}
			if lv+1 > best {
				best = lv + 1
			}
		}

		// Cache the result
		memo[id] = best
		return best, nil
	}

	// Build the final result with plugins and their calculated levels
	result := make([]PluginWithLevel, 0, len(id2plugin))
	for _, id := range orderedIDs {
		p, ok := id2plugin[id]
		if !ok || p == nil {
			// This should not happen after completeness check, but handle defensively
			return nil, fmt.Errorf("plugin %s not found in id2plugin map after completeness check", id)
		}

		level, err := dfs(id)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate level for plugin %s (%s): %w",
				p.Name(), id, err)
		}

		result = append(result, PluginWithLevel{Plugin: p, level: level})
	}

	// Final validation: ensure result size matches input
	if len(result) != len(id2plugin) {
		return nil, fmt.Errorf("result size mismatch: %d plugins in result, %d expected",
			len(result), len(id2plugin))
	}

	return result, nil
}

// UnloadOrder returns a best-effort unload order (dependents first, then dependencies).
// Only considers required dependencies that exist in plugs; ignores missing deps.
// Used when TopologicalSort fails so unload order is still dependency-aware.
// Builds load-order graph (dep -> dependent), runs Kahn; unload order = reverse(load order).
func (m *DefaultPluginManager[T]) UnloadOrder(plugs []plugins.Plugin) []plugins.Plugin {
	if len(plugs) == 0 {
		return nil
	}
	id2plugin := make(map[string]plugins.Plugin)
	for _, p := range plugs {
		if p == nil {
			continue
		}
		id2plugin[p.ID()] = p
	}
	if len(id2plugin) == 0 {
		return plugs
	}
	// Graph: dependency -> dependents. Load order = deps first; unload = reverse.
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	for id := range id2plugin {
		inDegree[id] = 0
	}
	for _, p := range id2plugin {
		deps := p.GetDependencies()
		for _, dep := range deps {
			if dep.Type != plugins.DependencyTypeRequired {
				continue
			}
			if _, ok := id2plugin[dep.ID]; !ok {
				continue
			}
			graph[dep.ID] = append(graph[dep.ID], p.ID())
			inDegree[p.ID()]++
		}
	}
	var loadOrder []string
	queue := make([]string, 0)
	for id, d := range inDegree {
		if d == 0 {
			queue = append(queue, id)
		}
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		loadOrder = append(loadOrder, current)
		for _, next := range graph[current] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	// Append any remaining (cycle or disconnected) in deterministic order
	for id, d := range inDegree {
		if d > 0 {
			loadOrder = append(loadOrder, id)
		}
	}
	// Unload order = reverse load order (dependents first)
	unloadIDs := make([]string, 0, len(loadOrder))
	for i := len(loadOrder) - 1; i >= 0; i-- {
		unloadIDs = append(unloadIDs, loadOrder[i])
	}
	// Map back to plugins; preserve order
	order := make([]plugins.Plugin, 0, len(unloadIDs))
	for _, id := range unloadIDs {
		if p, ok := id2plugin[id]; ok {
			order = append(order, p)
		}
	}
	return order
}

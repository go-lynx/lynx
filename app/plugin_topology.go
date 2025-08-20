// Package app: plugin dependency topology utilities.
package app

import (
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
		id2plugin[p.ID()] = p
	}

	// Create a dependency graph to manage plugin relationships
	dg := plugins.NewDependencyGraph()
	for _, p := range id2plugin {
		dg.AddPlugin(p)
	}

	// Map to store required dependencies for each plugin
	requiredDeps := make(map[string][]string, len(id2plugin))

	// Process each plugin's dependencies
	for _, p := range id2plugin {
		deps := p.GetDependencies()
		if len(deps) == 0 {
			continue
		}

		// Only process required dependencies that exist in our plugin set
		for _, dep := range deps {
			if dep.Type != plugins.DependencyTypeRequired {
				continue
			}
			if _, ok := id2plugin[dep.ID]; !ok {
				continue
			}

			d := dep
			// Add the dependency to the graph
			if err := dg.AddDependency(p.ID(), &d); err != nil {
				return nil, err
			}
			// Track this required dependency
			requiredDeps[p.ID()] = append(requiredDeps[p.ID()], dep.ID)
		}
	}

	// Resolve the dependency order using topological sort
	orderedIDs, err := dg.ResolveDependencies()
	if err != nil {
		return nil, err
	}

	// Memoization cache for calculated levels
	memo := make(map[string]int, len(id2plugin))
	// Tracking currently visiting nodes to detect cycles
	visiting := make(map[string]bool, len(id2plugin))

	// Depth-first search function to calculate dependency level
	var dfs func(string) int
	dfs = func(id string) int {
		// Return cached result if already calculated
		if lv, ok := memo[id]; ok {
			return lv
		}
		// Prevent infinite recursion in case of cycles
		if visiting[id] {
			return 0
		}
		visiting[id] = true

		// Calculate the maximum level from all dependencies
		best := 0
		for _, depID := range requiredDeps[id] {
			lv := dfs(depID)
			if lv+1 > best {
				best = lv + 1
			}
		}

		// Mark as no longer visiting and cache the result
		visiting[id] = false
		memo[id] = best
		return best
	}

	// Build the final result with plugins and their calculated levels
	result := make([]PluginWithLevel, 0, len(id2plugin))
	for _, id := range orderedIDs {
		p, ok := id2plugin[id]
		if !ok || p == nil {
			continue
		}
		result = append(result, PluginWithLevel{Plugin: p, level: dfs(id)})
	}
	return result, nil
}

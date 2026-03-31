package catalog

import (
	"fmt"

	"github.com/lleontor705/cortex-ia/internal/model"
)

// ParallelGroups represents components grouped by dependency level.
// Groups at the same level can be executed in parallel.
type ParallelGroups [][]model.ComponentID

// TopoSort performs a topological sort using Kahn's algorithm.
// Returns components ordered by dependency level, with cycle detection.
// Components at the same level have no inter-dependencies and can run in parallel.
func TopoSort(selected []model.ComponentID) (ParallelGroups, error) {
	cmap := ComponentMap()

	// Filter to only selected + their transitive deps.
	resolved := ResolveDeps(selected)
	resolvedSet := make(map[model.ComponentID]bool, len(resolved))
	for _, id := range resolved {
		resolvedSet[id] = true
	}

	// Build in-degree map and adjacency list.
	inDegree := make(map[model.ComponentID]int)
	children := make(map[model.ComponentID][]model.ComponentID)

	for _, id := range resolved {
		if _, ok := inDegree[id]; !ok {
			inDegree[id] = 0
		}
		info, ok := cmap[id]
		if !ok {
			continue
		}
		for _, dep := range info.Deps {
			if resolvedSet[dep] {
				children[dep] = append(children[dep], id)
				inDegree[id]++
			}
		}
	}

	// Kahn's algorithm: process nodes with in-degree 0 level by level.
	var groups ParallelGroups
	processed := 0

	for {
		// Collect all nodes with in-degree 0.
		var level []model.ComponentID
		for _, id := range resolved {
			if inDegree[id] == 0 {
				level = append(level, id)
			}
		}

		if len(level) == 0 {
			break
		}

		groups = append(groups, level)
		processed += len(level)

		// Remove processed nodes and decrement in-degrees.
		for _, id := range level {
			inDegree[id] = -1 // mark as processed
			for _, child := range children[id] {
				inDegree[child]--
			}
		}
	}

	if processed != len(resolved) {
		return nil, fmt.Errorf("dependency cycle detected: processed %d of %d components", processed, len(resolved))
	}

	return groups, nil
}

// Flatten converts parallel groups back to a flat ordered slice.
func (g ParallelGroups) Flatten() []model.ComponentID {
	var result []model.ComponentID
	for _, group := range g {
		result = append(result, group...)
	}
	return result
}

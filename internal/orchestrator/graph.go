package orchestrator

import (
	"fmt"
	"sort"
	"strings"
)

// Tier is a group of services that can start in parallel.
type Tier []string

// BuildTiers returns services grouped into startup tiers using Kahn's algorithm.
// Services in tier N have all their dependencies satisfied by tiers 0..N-1.
// Returns an error if a dependency cycle is detected.
// Complexity: O(V+E) where V = services and E = dependency edges.
func BuildTiers(deps map[string][]string) ([]Tier, error) {
	// Build adjacency list (reverse edges) and compute in-degrees.
	inDegree := make(map[string]int, len(deps))
	dependents := make(map[string][]string, len(deps))

	for svc := range deps {
		if _, ok := inDegree[svc]; !ok {
			inDegree[svc] = 0
		}
		for _, dep := range deps[svc] {
			if _, ok := inDegree[dep]; !ok {
				inDegree[dep] = 0
			}
			inDegree[svc]++
			dependents[dep] = append(dependents[dep], svc)
		}
	}

	// Validate that every listed dependency is itself a declared service.
	for svc, depList := range deps {
		for _, dep := range depList {
			if _, ok := deps[dep]; !ok {
				return nil, fmt.Errorf("service %q depends on %q which is not declared", svc, dep)
			}
		}
	}

	total := len(inDegree)
	var tiers []Tier
	processed := 0

	// Seed the first frontier with all zero in-degree nodes.
	var frontier []string
	for svc, deg := range inDegree {
		if deg == 0 {
			frontier = append(frontier, svc)
		}
	}

	for len(frontier) > 0 {
		sort.Strings(frontier)
		tier := make(Tier, len(frontier))
		copy(tier, frontier)
		tiers = append(tiers, tier)
		processed += len(frontier)

		// Compute next frontier by decrementing dependents' in-degrees.
		var next []string
		for _, svc := range frontier {
			for _, dep := range dependents[svc] {
				inDegree[dep]--
				if inDegree[dep] == 0 {
					next = append(next, dep)
				}
			}
		}
		frontier = next
	}

	if processed != total {
		remaining := make([]string, 0, total-processed)
		for svc, deg := range inDegree {
			if deg > 0 {
				remaining = append(remaining, svc)
			}
		}
		sort.Strings(remaining)
		return nil, fmt.Errorf("cycle detected in service dependencies involving: %s", strings.Join(remaining, ", "))
	}

	return tiers, nil
}

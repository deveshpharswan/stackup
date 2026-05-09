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
func BuildTiers(deps map[string][]string) ([]Tier, error) {
	inDegree := make(map[string]int, len(deps))
	for svc := range deps {
		if _, ok := inDegree[svc]; !ok {
			inDegree[svc] = 0
		}
		for _, dep := range deps[svc] {
			if _, ok := inDegree[dep]; !ok {
				inDegree[dep] = 0
			}
			inDegree[svc]++
		}
	}

	var tiers []Tier
	for len(inDegree) > 0 {
		var tier Tier
		for svc, deg := range inDegree {
			if deg == 0 {
				tier = append(tier, svc)
			}
		}
		if len(tier) == 0 {
			remaining := make([]string, 0, len(inDegree))
			for svc := range inDegree {
				remaining = append(remaining, svc)
			}
			sort.Strings(remaining)
			return nil, fmt.Errorf("cycle detected in service dependencies involving: %s", strings.Join(remaining, ", "))
		}
		sort.Strings(tier)
		tiers = append(tiers, tier)
		for _, svc := range tier {
			delete(inDegree, svc)
		}
		for svc := range inDegree {
			count := 0
			for _, dep := range deps[svc] {
				if _, still := inDegree[dep]; still {
					count++
				}
			}
			inDegree[svc] = count
		}
	}
	return tiers, nil
}

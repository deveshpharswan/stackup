package orchestrator_test

import (
	"fmt"
	"testing"

	"github.com/deveshpharswan/stackup/internal/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTiers_NoDeps(t *testing.T) {
	t.Parallel()
	deps := map[string][]string{
		"web":      {},
		"worker":   {},
		"postgres": {},
	}
	tiers, err := orchestrator.BuildTiers(deps)
	require.NoError(t, err)
	assert.Len(t, tiers, 1)
	assert.ElementsMatch(t, []string{"web", "worker", "postgres"}, tiers[0])
}

func TestBuildTiers_LinearChain(t *testing.T) {
	t.Parallel()
	deps := map[string][]string{
		"postgres": {},
		"api":      {"postgres"},
		"web":      {"api"},
	}
	tiers, err := orchestrator.BuildTiers(deps)
	require.NoError(t, err)
	require.Len(t, tiers, 3)
	assert.Equal(t, orchestrator.Tier{"postgres"}, tiers[0])
	assert.Equal(t, orchestrator.Tier{"api"}, tiers[1])
	assert.Equal(t, orchestrator.Tier{"web"}, tiers[2])
}

func TestBuildTiers_SharedDep(t *testing.T) {
	t.Parallel()
	deps := map[string][]string{
		"postgres": {},
		"redis":    {},
		"api":      {"postgres", "redis"},
	}
	tiers, err := orchestrator.BuildTiers(deps)
	require.NoError(t, err)
	assert.Len(t, tiers, 2)
	assert.ElementsMatch(t, []string{"postgres", "redis"}, tiers[0])
	assert.Equal(t, orchestrator.Tier{"api"}, tiers[1])
}

func TestBuildTiers_CycleDetected(t *testing.T) {
	t.Parallel()
	deps := map[string][]string{
		"a": {"b"},
		"b": {"a"},
	}
	_, err := orchestrator.BuildTiers(deps)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
	assert.Contains(t, err.Error(), "a")
	assert.Contains(t, err.Error(), "b")
}

func TestBuildTiers_DiamondDependency(t *testing.T) {
	t.Parallel()
	deps := map[string][]string{
		"db":      {},
		"cache":   {},
		"api":     {"db", "cache"},
		"worker":  {"db", "cache"},
		"gateway": {"api", "worker"},
	}
	tiers, err := orchestrator.BuildTiers(deps)
	require.NoError(t, err)
	require.Len(t, tiers, 3)
	assert.ElementsMatch(t, []string{"cache", "db"}, tiers[0])
	assert.ElementsMatch(t, []string{"api", "worker"}, tiers[1])
	assert.Equal(t, orchestrator.Tier{"gateway"}, tiers[2])
}

func TestBuildTiers_LargeGraph(t *testing.T) {
	t.Parallel()
	// 100 services in a 10-tier linear chain (10 services per tier)
	deps := make(map[string][]string)
	for tier := 0; tier < 10; tier++ {
		for i := 0; i < 10; i++ {
			name := fmt.Sprintf("svc-%d-%d", tier, i)
			if tier == 0 {
				deps[name] = nil
			} else {
				prev := fmt.Sprintf("svc-%d-0", tier-1)
				deps[name] = []string{prev}
			}
		}
	}
	tiers, err := orchestrator.BuildTiers(deps)
	require.NoError(t, err)
	assert.Len(t, tiers, 10)
	assert.Len(t, tiers[0], 10)
}

func TestBuildTiers_UnknownDependency(t *testing.T) {
	t.Parallel()
	deps := map[string][]string{
		"api": {"postgress"}, // typo — "postgress" is not a declared service
	}
	_, err := orchestrator.BuildTiers(deps)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "postgress")
}

package orchestrator_test

import (
	"testing"

	"github.com/stackup-dev/stackup/internal/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTiers_NoDeps(t *testing.T) {
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
	deps := map[string][]string{
		"a": {"b"},
		"b": {"a"},
	}
	_, err := orchestrator.BuildTiers(deps)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

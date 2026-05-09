package env_test

import (
	"testing"

	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/env"
	"github.com/stretchr/testify/assert"
)

var testSchema = map[string]config.EnvVar{
	"DATABASE_URL": {Type: "url", Required: true},
	"PORT":         {Type: "int", Required: true},
}

func TestValidate_AllPresent_ValidTypes(t *testing.T) {
	t.Parallel()
	result := env.Validate("../../testdata/.env.valid", "../../testdata/.env.example", testSchema)
	assert.True(t, result.Valid())
	assert.Empty(t, result.Errors)
}

func TestValidate_MissingKey(t *testing.T) {
	t.Parallel()
	result := env.Validate("../../testdata/.env.missing-key", "../../testdata/.env.example", nil)
	assert.False(t, result.Valid())
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "API_KEY", result.Errors[0].Key)
}

func TestValidate_BadTypes(t *testing.T) {
	t.Parallel()
	result := env.Validate("../../testdata/.env.bad-type", "../../testdata/.env.example", testSchema)
	assert.False(t, result.Valid())
	keys := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		keys[i] = e.Key
	}
	assert.Contains(t, keys, "DATABASE_URL")
	assert.Contains(t, keys, "PORT")
}

func TestValidate_NoExampleFile(t *testing.T) {
	t.Parallel()
	result := env.Validate("../../testdata/.env.valid", "nonexistent", nil)
	assert.True(t, result.Valid())
}

func TestValidateWithDefaults_InjectsDefaults(t *testing.T) {
	t.Parallel()
	schema := map[string]config.EnvVar{
		"DATABASE_URL": {Type: "url", Required: true},
		"PORT":         {Type: "int", Required: true},
		"LOG_LEVEL":    {Type: "", Required: false, Default: "info"},
		"TIMEOUT":      {Type: "int", Required: false, Default: "30"},
	}
	// .env.valid has DATABASE_URL, PORT, API_KEY but not LOG_LEVEL or TIMEOUT
	result, injected := env.ValidateWithDefaults("../../testdata/.env.valid", "../../testdata/.env.example", schema)
	assert.True(t, result.Valid())
	assert.Equal(t, "info", injected["LOG_LEVEL"])
	assert.Equal(t, "30", injected["TIMEOUT"])
	assert.NotContains(t, injected, "DATABASE_URL") // already present, not injected
	assert.NotContains(t, injected, "PORT")         // already present, not injected
}

func TestValidateWithDefaults_NoDefaultWhenPresent(t *testing.T) {
	t.Parallel()
	schema := map[string]config.EnvVar{
		"PORT": {Type: "int", Required: true, Default: "8080"},
	}
	// .env.valid already has PORT=3000, so default should NOT be injected
	result, injected := env.ValidateWithDefaults("../../testdata/.env.valid", "../../testdata/.env.example", schema)
	assert.True(t, result.Valid())
	assert.Empty(t, injected)
}

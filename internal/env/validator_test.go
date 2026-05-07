package env_test

import (
	"testing"

	"github.com/stackup-dev/stackup/internal/config"
	"github.com/stackup-dev/stackup/internal/env"
	"github.com/stretchr/testify/assert"
)

var testSchema = map[string]config.EnvVar{
	"DATABASE_URL": {Type: "url", Required: true},
	"PORT":         {Type: "int", Required: true},
}

func TestValidate_AllPresent_ValidTypes(t *testing.T) {
	result := env.Validate("../../testdata/.env.valid", "../../testdata/.env.example", testSchema)
	assert.True(t, result.Valid())
	assert.Empty(t, result.Errors)
}

func TestValidate_MissingKey(t *testing.T) {
	result := env.Validate("../../testdata/.env.missing-key", "../../testdata/.env.example", nil)
	assert.False(t, result.Valid())
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "API_KEY", result.Errors[0].Key)
}

func TestValidate_BadTypes(t *testing.T) {
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
	result := env.Validate("../../testdata/.env.valid", "nonexistent", nil)
	assert.True(t, result.Valid())
}

package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCommand_Services(t *testing.T) {
	cmd, arg := parseCommand("services")
	assert.Equal(t, "services", cmd)
	assert.Equal(t, "", arg)
}

func TestParseCommand_LogsWithArg(t *testing.T) {
	cmd, arg := parseCommand("logs api")
	assert.Equal(t, "logs", cmd)
	assert.Equal(t, "api", arg)
}

func TestParseCommand_Aliases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"svc", "services"},
		{"l api", "logs"},
		{"doc", "doctor"},
		{"g", "graph"},
		{"desc api", "describe"},
		{"q", "quit"},
	}
	for _, tt := range tests {
		cmd, _ := parseCommand(tt.input)
		resolved := resolveAlias(cmd)
		assert.Equal(t, tt.want, resolved, "input: %s", tt.input)
	}
}

func TestTabComplete(t *testing.T) {
	names := []string{"api", "api-worker", "postgres", "redis"}
	result := tabComplete("ap", names)
	assert.Equal(t, "api", result)

	result = tabComplete("api-", names)
	assert.Equal(t, "api-worker", result)

	result = tabComplete("post", names)
	assert.Equal(t, "postgres", result)
}

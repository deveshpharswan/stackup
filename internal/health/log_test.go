package health

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestScanForPattern_Found(t *testing.T) {
	input := "starting up\ndatabase system is ready to accept connections\nlistening on port 5432"
	found, err := ScanForPattern(strings.NewReader(input), "ready to accept connections")
	assert.NoError(t, err)
	assert.True(t, found)
}

func TestScanForPattern_NotFound(t *testing.T) {
	input := "starting up\ninitializing shared memory\n"
	found, err := ScanForPattern(strings.NewReader(input), "ready to accept connections")
	assert.NoError(t, err)
	assert.False(t, found)
}

func TestScanForPattern_EmptyInput(t *testing.T) {
	found, err := ScanForPattern(strings.NewReader(""), "anything")
	assert.NoError(t, err)
	assert.False(t, found)
}

func TestNewLogChecker(t *testing.T) {
	// Just verify construction doesn't panic
	checker := NewLogChecker(nil, "postgres", "ready", 30*time.Second, 2*time.Second)
	assert.NotNil(t, checker)
}

package printer_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stackup-dev/stackup/internal/printer"
	"github.com/stretchr/testify/assert"
)

func TestPrinter_Phase(t *testing.T) {
	buf := new(bytes.Buffer)
	p := printer.New(buf)
	p.Phase("Pre-flight")
	assert.Contains(t, buf.String(), "Pre-flight")
}

func TestPrinter_ServiceHealthy(t *testing.T) {
	buf := new(bytes.Buffer)
	p := printer.New(buf)
	p.ServiceHealthy("postgres", "tcp:5432", 2300*time.Millisecond)
	out := buf.String()
	assert.Contains(t, out, "postgres")
	assert.Contains(t, out, "tcp:5432")
	assert.Contains(t, out, "2.3s")
}

func TestPrinter_ServiceFailed(t *testing.T) {
	buf := new(bytes.Buffer)
	p := printer.New(buf)
	p.ServiceFailed("api", fmt.Errorf("connection refused"))
	assert.Contains(t, buf.String(), "api")
	assert.Contains(t, buf.String(), "connection refused")
}

func TestPrinter_ValidationError(t *testing.T) {
	buf := new(bytes.Buffer)
	p := printer.New(buf)
	p.ValidationError("DATABASE_URL", `expected valid URL, got "notaurl"`)
	out := buf.String()
	assert.Contains(t, out, "DATABASE_URL")
	assert.Contains(t, out, "expected valid URL")
}

func TestPrinter_Ready(t *testing.T) {
	buf := new(bytes.Buffer)
	p := printer.New(buf)
	p.Ready(8100 * time.Millisecond)
	assert.Contains(t, buf.String(), "8.1s")
}

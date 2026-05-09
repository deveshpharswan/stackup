package health_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/deveshpharswan/stackup/internal/health"
	"github.com/stretchr/testify/assert"
)

func TestTCPChecker_Open(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer ln.Close()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	checker := health.NewTCPChecker(host, port, 5*time.Second, 100*time.Millisecond)
	assert.NoError(t, checker.Check(context.Background()))
}

func TestTCPChecker_Closed(t *testing.T) {
	checker := health.NewTCPChecker("127.0.0.1", "19998", 300*time.Millisecond, 50*time.Millisecond)
	assert.Error(t, checker.Check(context.Background()))
}

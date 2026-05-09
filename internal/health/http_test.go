package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deveshpharswan/stackup/internal/health"
	"github.com/stretchr/testify/assert"
)

func TestHTTPChecker_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	checker := health.NewHTTPChecker(srv.URL, 5*time.Second, 100*time.Millisecond)
	assert.NoError(t, checker.Check(context.Background()))
}

func TestHTTPChecker_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	checker := health.NewHTTPChecker(srv.URL, 300*time.Millisecond, 50*time.Millisecond)
	assert.Error(t, checker.Check(context.Background()))
}

func TestHTTPChecker_ServerDown(t *testing.T) {
	checker := health.NewHTTPChecker("http://127.0.0.1:19999", 300*time.Millisecond, 50*time.Millisecond)
	assert.Error(t, checker.Check(context.Background()))
}

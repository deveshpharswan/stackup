package health

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type HTTPChecker struct {
	url      string
	timeout  time.Duration
	interval time.Duration
}

func NewHTTPChecker(url string, timeout, interval time.Duration) *HTTPChecker {
	return &HTTPChecker{url: url, timeout: timeout, interval: interval}
}

func (c *HTTPChecker) Check(ctx context.Context) error {
	deadline := time.Now().Add(c.timeout)
	client := &http.Client{Timeout: c.interval}
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		resp, err := client.Get(c.url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.interval):
		}
	}
	return fmt.Errorf("http check timed out after %s: %s", c.timeout, c.url)
}

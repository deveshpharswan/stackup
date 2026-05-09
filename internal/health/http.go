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
	reqTimeout := c.interval
	if reqTimeout < 5*time.Second {
		reqTimeout = 5 * time.Second
	}
	client := &http.Client{Timeout: reqTimeout}

	err := Poll(ctx, c.timeout, c.interval, func() error {
		resp, err := client.Get(c.url)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		return fmt.Errorf("status %d", resp.StatusCode)
	})
	if err != nil && err != ctx.Err() {
		return fmt.Errorf("http check timed out after %s: %s", c.timeout, c.url)
	}
	return err
}

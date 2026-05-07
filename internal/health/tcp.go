package health

import (
	"context"
	"fmt"
	"net"
	"time"
)

type TCPChecker struct {
	host     string
	port     string
	timeout  time.Duration
	interval time.Duration
}

func NewTCPChecker(host, port string, timeout, interval time.Duration) *TCPChecker {
	return &TCPChecker{host: host, port: port, timeout: timeout, interval: interval}
}

func (c *TCPChecker) Check(ctx context.Context) error {
	deadline := time.Now().Add(c.timeout)
	addr := net.JoinHostPort(c.host, c.port)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		conn, err := net.DialTimeout("tcp", addr, c.interval)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.interval):
		}
	}
	return fmt.Errorf("tcp check timed out after %s: %s", c.timeout, addr)
}

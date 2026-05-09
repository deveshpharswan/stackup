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
	addr := net.JoinHostPort(c.host, c.port)

	err := Poll(ctx, c.timeout, c.interval, func() error {
		conn, err := net.DialTimeout("tcp", addr, c.interval)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	})
	if err != nil && err != ctx.Err() {
		return fmt.Errorf("tcp check timed out after %s: %s", c.timeout, addr)
	}
	return err
}

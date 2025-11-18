package strategy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type NormalHTTP struct {
	client            *http.Client
	timeout           time.Duration
	activeConnections int64
	localAddr         *net.TCPAddr
}

func NewNormalHTTP(timeout time.Duration, bindIP string) *NormalHTTP {
	n := &NormalHTTP{
		timeout:   timeout,
		localAddr: newLocalTCPAddr(bindIP),
	}

	transport := &http.Transport{
		MaxIdleConns:          0,
		MaxIdleConnsPerHost:   0,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		ExpectContinueTimeout: 1 * time.Second,
	}

	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			LocalAddr: n.localAddr,
		}
		conn, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}

		atomic.AddInt64(&n.activeConnections, 1)

		return &trackedConn{
			Conn: conn,
			onClose: func() {
				atomic.AddInt64(&n.activeConnections, -1)
			},
		}, nil
	}

	n.client = &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}

	return n
}

func (n *NormalHTTP) ActiveConnections() int64 {
	return atomic.LoadInt64(&n.activeConnections)
}

func (n *NormalHTTP) Execute(ctx context.Context, target Target) error {
	ctx, cancel := context.WithTimeout(ctx, n.timeout)
	defer cancel()

	var body io.Reader
	if len(target.Body) > 0 {
		body = bytes.NewReader(target.Body)
	}

	req, err := http.NewRequestWithContext(ctx, target.Method, target.URL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range target.Headers {
		req.Header.Set(k, v)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http error: %d", resp.StatusCode)
	}

	return nil
}

func (n *NormalHTTP) Name() string {
	return "normal-http"
}

type trackedConn struct {
	net.Conn
	onClose func()
	once    sync.Once
}

func (c *trackedConn) Close() error {
	c.once.Do(func() {
		if c.onClose != nil {
			c.onClose()
		}
	})
	return c.Conn.Close()
}

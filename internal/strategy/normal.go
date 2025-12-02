package strategy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"loadtestforge/internal/netutil"
)

type NormalHTTP struct {
	client            *http.Client
	timeout           time.Duration
	activeConnections int64
}

func NewNormalHTTP(timeout time.Duration, bindIP string) *NormalHTTP {
	n := &NormalHTTP{
		timeout: timeout,
	}

	dialerCfg := netutil.DialerConfig{
		Timeout:       30 * time.Second,
		KeepAlive:     30 * time.Second,
		LocalAddr:     netutil.NewLocalTCPAddr(bindIP),
		TLSSkipVerify: false,
	}

	transport := netutil.NewTrackedTransport(dialerCfg, &n.activeConnections)
	transport.MaxIdleConns = 0
	transport.MaxIdleConnsPerHost = 0
	transport.MaxConnsPerHost = 0
	transport.DisableKeepAlives = false

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

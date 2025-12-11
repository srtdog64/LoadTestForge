package strategy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jdw/loadtestforge/internal/config"
	"github.com/jdw/loadtestforge/internal/netutil"
)

// NormalHTTP implements standard HTTP request strategy.
// Each request creates a new connection (Connection: close behavior).
type NormalHTTP struct {
	BaseStrategy
	client  *http.Client
	timeout time.Duration
}

// NewNormalHTTP creates a new NormalHTTP strategy.
func NewNormalHTTP(timeout time.Duration, bindIP string) *NormalHTTP {
	common := DefaultCommonConfig()
	common.ConnectTimeout = timeout

	n := &NormalHTTP{
		BaseStrategy: NewBaseStrategy(bindIP, common),
		timeout:      timeout,
	}

	dialerCfg := netutil.DialerConfig{
		Timeout:       30 * time.Second,
		KeepAlive:     30 * time.Second,
		LocalAddr:     netutil.NewLocalTCPAddr(bindIP),
		BindConfig:    netutil.NewBindConfig(bindIP),
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

// NewNormalHTTPWithConfig creates a NormalHTTP strategy from StrategyConfig.
func NewNormalHTTPWithConfig(cfg *config.StrategyConfig, bindIP string) *NormalHTTP {
	return NewNormalHTTP(cfg.Timeout, bindIP)
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

	startTime := time.Now()
	resp, err := n.client.Do(req)
	latency := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http error: %d", resp.StatusCode)
	}

	n.RecordLatency(latency)

	return nil
}

func (n *NormalHTTP) Name() string {
	return "normal-http"
}

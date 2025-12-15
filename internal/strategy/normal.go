package strategy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/srtdog64/loadtestforge/internal/config"
	"github.com/srtdog64/loadtestforge/internal/errors"
	"github.com/srtdog64/loadtestforge/internal/netutil"
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

	// Use standardized DialerConfig with OnDial hook from BaseStrategy
	dialerCfg := n.GetDialerConfig()
	dialerCfg.Timeout = config.DefaultDialerTimeout
	dialerCfg.KeepAlive = config.DefaultDialerKeepAlive

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
	n := NewNormalHTTP(cfg.Timeout, bindIP)
	// Apply session lifetime from config (0 = unlimited, hold until server closes)
	n.Common.SessionLifetime = cfg.SessionLifetime
	return n
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
		return errors.ClassifyAndWrap(err, "failed to create request")
	}

	for k, v := range target.Headers {
		req.Header.Set(k, v)
	}

	startTime := time.Now()
	resp, err := n.client.Do(req)
	latency := time.Since(startTime)

	if err != nil {
		return errors.ClassifyAndWrap(err, "request failed")
	}
	defer resp.Body.Close()

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return errors.ClassifyAndWrap(err, "failed to read response body")
	}

	if resp.StatusCode >= 400 {
		return errors.NewHTTPError(resp.StatusCode, resp.Status, "")
	}

	n.RecordLatency(latency)

	return nil
}

func (n *NormalHTTP) Name() string {
	return "normal-http"
}

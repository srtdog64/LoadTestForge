package strategy

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

type KeepAliveHTTP struct {
	pingInterval      time.Duration
	connectionTimeout time.Duration
	maxSessionLife    time.Duration
	userAgents        []string
	metricsCallback   MetricsCallback
	activeConnections int64
}

func NewKeepAliveHTTP(pingInterval time.Duration) *KeepAliveHTTP {
	return &KeepAliveHTTP{
		pingInterval:      pingInterval,
		connectionTimeout: 10 * time.Second,
		maxSessionLife:    5 * time.Minute,
		userAgents:        defaultUserAgents,
	}
}

func (k *KeepAliveHTTP) SetMetricsCallback(callback MetricsCallback) {
	k.metricsCallback = callback
}

func (k *KeepAliveHTTP) ActiveConnections() int64 {
	return atomic.LoadInt64(&k.activeConnections)
}

func (k *KeepAliveHTTP) Execute(ctx context.Context, target Target) error {
	parsedURL, host, useTLS, err := parseTargetURL(target.URL)
	if err != nil {
		return err
	}

	sessionCtx, cancel := context.WithTimeout(ctx, k.maxSessionLife)
	defer cancel()

	connID := generateConnID()
	
	var conn net.Conn
	dialer := &net.Dialer{
		Timeout: k.connectionTimeout,
	}

	if useTLS {
		tlsConfig := &tls.Config{
			ServerName:         parsedURL.Hostname(),
			InsecureSkipVerify: false,
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", host, tlsConfig)
	} else {
		conn, err = dialer.DialContext(sessionCtx, "tcp", host)
	}

	if err != nil {
		if k.metricsCallback != nil {
			k.metricsCallback.RecordSocketTimeout()
		}
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() {
		atomic.AddInt64(&k.activeConnections, -1)
		conn.Close()
		if k.metricsCallback != nil {
			k.metricsCallback.RecordConnectionEnd(connID)
		}
	}()

	atomic.AddInt64(&k.activeConnections, 1)

	if k.metricsCallback != nil {
		k.metricsCallback.RecordConnectionStart(connID, conn.RemoteAddr().String())
	}

	userAgent := k.userAgents[rand.Intn(len(k.userAgents))]
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}
	if parsedURL.RawQuery != "" {
		path += "?" + parsedURL.RawQuery
	}

	request := fmt.Sprintf(
		"GET %s HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"User-Agent: %s\r\n"+
			"Accept: */*\r\n"+
			"Connection: keep-alive\r\n"+
			"\r\n",
		path, parsedURL.Host, userAgent,
	)

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(request)); err != nil {
		if k.metricsCallback != nil {
			k.metricsCallback.RecordSocketTimeout()
		}
		return fmt.Errorf("failed to send initial request: %w", err)
	}

	if k.metricsCallback != nil {
		k.metricsCallback.RecordConnectionActivity(connID)
	}

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)

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
		case <-sessionCtx.Done():
			return nil
		case <-ticker.C:
			pingCount++
			
			pingRequest := fmt.Sprintf(
				"GET %s HTTP/1.1\r\n"+
					"Host: %s\r\n"+
					"User-Agent: %s\r\n"+
					"Connection: keep-alive\r\n"+
					"X-Keep-Alive-Ping: %d\r\n"+
					"\r\n",
				path, parsedURL.Host, userAgent, pingCount,
			)

			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := conn.Write([]byte(pingRequest)); err != nil {
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

			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
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

func generateConnID() string {
	return fmt.Sprintf("conn-%d-%d", time.Now().UnixNano(), rand.Int63())
}

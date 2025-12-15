package netutil

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// ConnConfig holds connection configuration options.
type ConnConfig struct {
	Timeout        time.Duration
	MaxSessionLife time.Duration // 0 = unlimited (hold until server closes)
	LocalAddr      *net.TCPAddr  // Legacy single IP
	BindConfig     *BindConfig   // Multi-IP support
	WindowSize     int           // TCP receive buffer size (0 = default)
	TLSSkipVerify  bool          // Skip TLS certificate verification
}

// DefaultConnConfig returns sensible defaults.
// MaxSessionLife=0 means unlimited (hold connection until server closes).
func DefaultConnConfig(bindIP string) ConnConfig {
	return ConnConfig{
		Timeout:        10 * time.Second,
		MaxSessionLife: 0, // unlimited by default
		LocalAddr:      NewLocalTCPAddr(bindIP),
		BindConfig:     NewBindConfig(bindIP),
		WindowSize:     0,
		TLSSkipVerify:  true, // Default to true for load testing
	}
}

// GetLocalAddr returns the next local address for binding.
// Supports both legacy single IP and multi-IP pool.
func (c *ConnConfig) GetLocalAddr() *net.TCPAddr {
	if c.BindConfig != nil && c.BindConfig.HasMultipleIPs() {
		return c.BindConfig.GetLocalAddr()
	}
	return c.LocalAddr
}

// ManagedConn wraps a net.Conn with automatic connection tracking.
type ManagedConn struct {
	net.Conn
	counter    *int64
	sessionCtx context.Context
	cancel     context.CancelFunc
}

// DialManaged establishes a managed TCP connection with optional TLS.
// It automatically increments the counter on success and decrements on Close.
// If cfg.MaxSessionLife is 0, connection is held until server closes or parent context cancels.
func DialManaged(
	ctx context.Context,
	targetURL string,
	cfg ConnConfig,
	counter *int64,
) (*ManagedConn, *url.URL, error) {
	parsedURL, host, useTLS, err := ParseTargetURL(targetURL)
	if err != nil {
		return nil, nil, err
	}

	// Create session context: unlimited if MaxSessionLife=0, otherwise with timeout
	var sessionCtx context.Context
	var cancel context.CancelFunc
	if cfg.MaxSessionLife > 0 {
		sessionCtx, cancel = context.WithTimeout(ctx, cfg.MaxSessionLife)
	} else {
		sessionCtx, cancel = context.WithCancel(ctx)
	}

	dialer := &net.Dialer{
		Timeout:   cfg.Timeout,
		LocalAddr: cfg.GetLocalAddr(),
	}

	var conn net.Conn

	if useTLS {
		tlsConfig := &tls.Config{
			ServerName:         parsedURL.Hostname(),
			InsecureSkipVerify: cfg.TLSSkipVerify,
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", host, tlsConfig)
	} else {
		conn, err = dialer.DialContext(sessionCtx, "tcp", host)
	}

	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("connection failed: %w", err)
	}

	// Set TCP receive buffer if specified
	if cfg.WindowSize > 0 {
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			tcpConn.SetReadBuffer(cfg.WindowSize)
		}
	}

	atomic.AddInt64(counter, 1)

	mc := &ManagedConn{
		Conn:       conn,
		counter:    counter,
		sessionCtx: sessionCtx,
		cancel:     cancel,
	}

	return mc, parsedURL, nil
}

// Close closes the connection and decrements the counter.
func (mc *ManagedConn) Close() error {
	mc.cancel()
	atomic.AddInt64(mc.counter, -1)
	return mc.Conn.Close()
}

// Context returns the session context with timeout.
func (mc *ManagedConn) Context() context.Context {
	return mc.sessionCtx
}

// SetWriteTimeout sets write deadline relative to now.
func (mc *ManagedConn) SetWriteTimeout(d time.Duration) {
	mc.Conn.SetWriteDeadline(time.Now().Add(d))
}

// SetReadTimeout sets read deadline relative to now.
func (mc *ManagedConn) SetReadTimeout(d time.Duration) {
	mc.Conn.SetReadDeadline(time.Now().Add(d))
}

// WriteWithTimeout writes data with the specified timeout.
func (mc *ManagedConn) WriteWithTimeout(data []byte, timeout time.Duration) (int, error) {
	mc.SetWriteTimeout(timeout)
	return mc.Conn.Write(data)
}

// ReadWithTimeout reads data with the specified timeout.
func (mc *ManagedConn) ReadWithTimeout(buf []byte, timeout time.Duration) (int, error) {
	mc.SetReadTimeout(timeout)
	return mc.Conn.Read(buf)
}

// TrackedConn wraps net.Conn with a callback on close.
// Thread-safe: onClose is called exactly once.
type TrackedConn struct {
	net.Conn
	onClose func()
	closed  int32
}

// NewTrackedConn creates a connection with close callback.
func NewTrackedConn(conn net.Conn, onClose func()) *TrackedConn {
	return &TrackedConn{
		Conn:    conn,
		onClose: onClose,
	}
}

// Close closes the connection and calls the onClose callback once.
func (c *TrackedConn) Close() error {
	if atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		if c.onClose != nil {
			c.onClose()
		}
	}
	return c.Conn.Close()
}

// ParseTargetURL parses a URL and returns parsed URL, host:port, useTLS flag.
func ParseTargetURL(targetURL string) (*url.URL, string, bool, error) {
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

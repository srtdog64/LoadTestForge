package netutil

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// DialerConfig holds configuration for creating custom dialers.
type DialerConfig struct {
	Timeout       time.Duration
	KeepAlive     time.Duration
	LocalAddr     *net.TCPAddr // Legacy single IP
	BindConfig    *BindConfig  // Multi-IP support
	TLSSkipVerify bool
}

// DefaultDialerConfig returns sensible defaults for dialer configuration.
func DefaultDialerConfig(bindIP string) DialerConfig {
	return DialerConfig{
		Timeout:       30 * time.Second,
		KeepAlive:     30 * time.Second,
		LocalAddr:     NewLocalTCPAddr(bindIP),
		BindConfig:    NewBindConfig(bindIP),
		TLSSkipVerify: true,
	}
}

// GetLocalAddr returns the next local address for binding.
// Supports both legacy single IP and multi-IP pool.
func (c *DialerConfig) GetLocalAddr() *net.TCPAddr {
	if c.BindConfig != nil && c.BindConfig.HasMultipleIPs() {
		return c.BindConfig.GetLocalAddr()
	}
	return c.LocalAddr
}

// NewDialer creates a net.Dialer with the given configuration.
func NewDialer(cfg DialerConfig) *net.Dialer {
	return &net.Dialer{
		Timeout:   cfg.Timeout,
		KeepAlive: cfg.KeepAlive,
		LocalAddr: cfg.GetLocalAddr(),
	}
}

// NewTLSConfig creates a TLS configuration.
func NewTLSConfig(skipVerify bool) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: skipVerify,
	}
}

// NewTrackedTransport creates an http.Transport with connection tracking.
// The counter is incremented when a connection is established and
// decremented when it is closed.
// Supports multi-IP round-robin binding.
func NewTrackedTransport(cfg DialerConfig, counter *int64) *http.Transport {
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       NewTLSConfig(cfg.TLSSkipVerify),
	}

	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := &net.Dialer{
			Timeout:   cfg.Timeout,
			KeepAlive: cfg.KeepAlive,
			LocalAddr: cfg.GetLocalAddr(),
		}

		conn, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}

		atomic.AddInt64(counter, 1)

		return NewTrackedConn(conn, func() {
			atomic.AddInt64(counter, -1)
		}), nil
	}

	return transport
}

// DialTLS establishes a TLS connection using the provided dialer.
func DialTLS(ctx context.Context, host, serverName string, dialer *net.Dialer) (net.Conn, error) {
	tlsConfig := &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: true,
	}

	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, err
	}

	tlsConn := tls.Client(conn, tlsConfig)

	handshakeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- tlsConn.Handshake()
	}()

	select {
	case <-handshakeCtx.Done():
		conn.Close()
		return nil, handshakeCtx.Err()
	case err := <-done:
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	return tlsConn, nil
}

package netutil

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/srtdog64/loadtestforge/internal/config"
)

// DialerConfig holds configuration for creating custom dialers.
type DialerConfig struct {
	Timeout       time.Duration
	KeepAlive     time.Duration
	LocalAddr     *net.TCPAddr // Legacy single IP
	BindConfig    *BindConfig  // Multi-IP support
	TLSSkipVerify bool
	OnDial        func() // Callback for connection attempts
}

// DefaultDialerConfig returns sensible defaults for dialer configuration.
func DefaultDialerConfig(bindIP string) DialerConfig {
	return DialerConfig{
		Timeout:       config.DefaultConnectTimeout,
		KeepAlive:     config.DefaultTCPKeepAlive,
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

		if cfg.OnDial != nil {
			cfg.OnDial()
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

// =============================================================================
// Unified Dial Functions
// =============================================================================

// DialWithConfig establishes a TCP connection using BindConfig.
// This is the primary dial function that supports multi-IP round-robin.
func DialWithConfig(ctx context.Context, network, address string, timeout time.Duration, bindCfg *BindConfig) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: config.DefaultTCPKeepAlive,
	}

	if bindCfg != nil {
		dialer.LocalAddr = bindCfg.GetLocalAddr()
	}

	return dialer.DialContext(ctx, network, address)
}

// DialTCPWithBind establishes a TCP connection with optional IP binding (legacy).
func DialTCPWithBind(ctx context.Context, address string, timeout time.Duration, bindIP string) (net.Conn, error) {
	return DialWithConfig(ctx, "tcp", address, timeout, NewBindConfig(bindIP))
}

// DialTLSWithConfig establishes a TLS connection using BindConfig.
func DialTLSWithConfig(ctx context.Context, address, serverName string, timeout time.Duration, bindCfg *BindConfig) (net.Conn, error) {
	// First establish TCP connection
	conn, err := DialWithConfig(ctx, "tcp", address, timeout, bindCfg)
	if err != nil {
		return nil, err
	}

	// Upgrade to TLS
	tlsConfig := &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: true,
	}

	tlsConn := tls.Client(conn, tlsConfig)

	// Perform handshake with timeout
	handshakeCtx, cancel := context.WithTimeout(ctx, config.DefaultConnectTimeout)
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

// =============================================================================
// Connection Factory
// =============================================================================

// ConnectionFactory provides a unified interface for creating connections.
type ConnectionFactory struct {
	BindConfig *BindConfig
	Timeout    time.Duration
	KeepAlive  time.Duration
	TLSConfig  *tls.Config
}

// NewConnectionFactory creates a new connection factory.
func NewConnectionFactory(bindIP string) *ConnectionFactory {
	return &ConnectionFactory{
		BindConfig: NewBindConfig(bindIP),
		Timeout:    config.DefaultConnectTimeout,
		KeepAlive:  config.DefaultTCPKeepAlive,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
}

// NewConnectionFactoryWithConfig creates a connection factory from DialerConfig.
func NewConnectionFactoryWithConfig(cfg DialerConfig) *ConnectionFactory {
	return &ConnectionFactory{
		BindConfig: cfg.BindConfig,
		Timeout:    cfg.Timeout,
		KeepAlive:  cfg.KeepAlive,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: cfg.TLSSkipVerify,
		},
	}
}

// Dial establishes a TCP connection.
func (f *ConnectionFactory) Dial(ctx context.Context, address string) (net.Conn, error) {
	return DialWithConfig(ctx, "tcp", address, f.Timeout, f.BindConfig)
}

// DialTLS establishes a TLS connection.
func (f *ConnectionFactory) DialTLS(ctx context.Context, address, serverName string) (net.Conn, error) {
	return DialTLSWithConfig(ctx, address, serverName, f.Timeout, f.BindConfig)
}

// CreateDialer returns a configured net.Dialer.
func (f *ConnectionFactory) CreateDialer() *net.Dialer {
	dialer := &net.Dialer{
		Timeout:   f.Timeout,
		KeepAlive: f.KeepAlive,
	}

	if f.BindConfig != nil {
		dialer.LocalAddr = f.BindConfig.GetLocalAddr()
	}

	return dialer
}

// CreateHTTPClient creates an http.Client with connection tracking.
func (f *ConnectionFactory) CreateHTTPClient(counter *int64) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       f.TLSConfig,
	}

	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := f.Dial(ctx, addr)
		if err != nil {
			return nil, err
		}

		if counter != nil {
			atomic.AddInt64(counter, 1)
			return NewTrackedConn(conn, func() {
				atomic.AddInt64(counter, -1)
			}), nil
		}

		return conn, nil
	}

	return &http.Client{
		Transport: transport,
		Timeout:   f.Timeout,
	}
}

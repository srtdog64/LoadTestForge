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
	LocalAddr     *net.TCPAddr
	TLSSkipVerify bool
}

// DefaultDialerConfig returns sensible defaults for dialer configuration.
func DefaultDialerConfig(bindIP string) DialerConfig {
	return DialerConfig{
		Timeout:       30 * time.Second,
		KeepAlive:     30 * time.Second,
		LocalAddr:     NewLocalTCPAddr(bindIP),
		TLSSkipVerify: true,
	}
}

// NewDialer creates a net.Dialer with the given configuration.
func NewDialer(cfg DialerConfig) *net.Dialer {
	return &net.Dialer{
		Timeout:   cfg.Timeout,
		KeepAlive: cfg.KeepAlive,
		LocalAddr: cfg.LocalAddr,
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
		dialer := NewDialer(cfg)
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

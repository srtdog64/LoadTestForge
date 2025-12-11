package strategy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jdw/loadtestforge/internal/netutil"
)

// TCPFloodConfig holds configuration for TCP Connection Flood attack.
type TCPFloodConfig struct {
	ConnectTimeout time.Duration
	HoldTime       time.Duration // 0 = infinite (hold until server closes)
	SendData       bool          // Send a byte after connection
	KeepAlive      bool          // Enable TCP keep-alive
}

// DefaultTCPFloodConfig returns sensible defaults for TCP Flood.
func DefaultTCPFloodConfig() TCPFloodConfig {
	return TCPFloodConfig{
		ConnectTimeout: 10 * time.Second,
		HoldTime:       0, // infinite by default
		SendData:       false,
		KeepAlive:      true,
	}
}

// TCPFloodStats tracks detailed statistics.
type TCPFloodStats struct {
	Active      int64
	Created     int64
	Successful  int64
	Failed      int64
	ServerDrops int64
	Reconnects  int64
	Errors      int64
	PeakActive  int64

	connectionDurations []float64
	errorTypes          map[string]int64
	errorSamples        []string
	mu                  sync.Mutex
	maxSamples          int
}

// NewTCPFloodStats creates a new stats tracker.
func NewTCPFloodStats() *TCPFloodStats {
	return &TCPFloodStats{
		connectionDurations: make([]float64, 0, 1000),
		errorTypes:          make(map[string]int64),
		errorSamples:        make([]string, 0, 50),
		maxSamples:          50,
	}
}

// RecordError records an error with context.
func (s *TCPFloodStats) RecordError(err error, context string) {
	atomic.AddInt64(&s.Errors, 1)

	s.mu.Lock()
	defer s.mu.Unlock()

	errorKey := fmt.Sprintf("%s:%T", context, err)
	s.errorTypes[errorKey]++

	if len(s.errorSamples) < s.maxSamples {
		timestamp := time.Now().Format("15:04:05")
		errMsg := err.Error()
		if len(errMsg) > 100 {
			errMsg = errMsg[:100]
		}
		s.errorSamples = append(s.errorSamples, fmt.Sprintf("[%s] %s: %s", timestamp, errorKey, errMsg))
	}
}

// RecordDuration records connection duration.
func (s *TCPFloodStats) RecordDuration(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.connectionDurations = append(s.connectionDurations, duration.Seconds())
	if len(s.connectionDurations) > 1000 {
		s.connectionDurations = s.connectionDurations[len(s.connectionDurations)-1000:]
	}
}

// GetAvgDuration returns average connection duration in seconds.
func (s *TCPFloodStats) GetAvgDuration() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.connectionDurations) == 0 {
		return 0
	}

	sum := 0.0
	for _, d := range s.connectionDurations {
		sum += d
	}
	return sum / float64(len(s.connectionDurations))
}

// UpdatePeak updates peak active connections.
func (s *TCPFloodStats) UpdatePeak() {
	current := atomic.LoadInt64(&s.Active)
	for {
		peak := atomic.LoadInt64(&s.PeakActive)
		if current <= peak {
			break
		}
		if atomic.CompareAndSwapInt64(&s.PeakActive, peak, current) {
			break
		}
	}
}

// TCPFlood implements TCP Connection Flood (L7 Full Open) attack.
// It rapidly creates TCP connections to exhaust server connection limits
// and holds them until the server closes or context is cancelled.
type TCPFlood struct {
	config            TCPFloodConfig
	stats             *TCPFloodStats
	activeConnections int64
	metricsCallback   MetricsCallback
	bindConfig        *netutil.BindConfig
}

// NewTCPFlood creates a new TCP Flood attack strategy.
func NewTCPFlood(cfg TCPFloodConfig, bindIP string) *TCPFlood {
	return &TCPFlood{
		config:     cfg,
		stats:      NewTCPFloodStats(),
		bindConfig: netutil.NewBindConfig(bindIP),
	}
}

// Execute performs a single TCP Flood attack cycle.
// It connects, holds the connection until server drops or context cancels,
// then returns (allowing session manager to restart).
func (t *TCPFlood) Execute(ctx context.Context, target Target) error {
	parsedURL, host, useTLS, err := netutil.ParseTargetURL(target.URL)
	if err != nil {
		return err
	}

	conn, err := t.dialWithOptions(ctx, host, useTLS, parsedURL.Hostname())
	if err != nil {
		t.stats.RecordError(err, "connect")
		atomic.AddInt64(&t.stats.Failed, 1)
		return err
	}

	connectTime := time.Now()
	atomic.AddInt64(&t.activeConnections, 1)
	atomic.AddInt64(&t.stats.Active, 1)
	atomic.AddInt64(&t.stats.Created, 1)
	atomic.AddInt64(&t.stats.Successful, 1)
	t.stats.UpdatePeak()

	defer func() {
		conn.Close()
		atomic.AddInt64(&t.activeConnections, -1)
		atomic.AddInt64(&t.stats.Active, -1)
		t.stats.RecordDuration(time.Since(connectTime))
	}()

	// Optional: send a byte after connection
	if t.config.SendData {
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := conn.Write([]byte{0x00}); err != nil {
			// Ignore error, connection may still be valid
		}
	}

	// Hold connection until server drops or context cancels
	if t.config.HoldTime > 0 {
		// Timed hold mode
		return t.holdForDuration(ctx, conn)
	}

	// Infinite hold mode - wait until server closes
	return t.holdUntilServerDrops(ctx, conn)
}

func (t *TCPFlood) dialWithOptions(ctx context.Context, host string, useTLS bool, hostname string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout:   t.config.ConnectTimeout,
		LocalAddr: t.bindConfig.GetLocalAddr(),
	}

	dialCtx, cancel := context.WithTimeout(ctx, t.config.ConnectTimeout)
	defer cancel()

	var conn net.Conn
	var err error

	if useTLS {
		tlsConfig := &tls.Config{
			ServerName:         hostname,
			InsecureSkipVerify: true,
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", host, tlsConfig)
	} else {
		conn, err = dialer.DialContext(dialCtx, "tcp", host)
	}

	if err != nil {
		return nil, err
	}

	// Configure TCP options
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)

		if t.config.KeepAlive {
			tcpConn.SetKeepAlive(true)
			tcpConn.SetKeepAlivePeriod(60 * time.Second)
		}
	}

	return conn, nil
}

// holdUntilServerDrops holds the connection until server closes it.
func (t *TCPFlood) holdUntilServerDrops(ctx context.Context, conn net.Conn) error {
	buf := make([]byte, 1)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Try to read with short timeout to check if server closed
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, err := conn.Read(buf)

		if err != nil {
			// Check if it's a timeout (expected) or actual close
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout is expected, continue holding
				continue
			}

			// Server closed the connection or error occurred
			atomic.AddInt64(&t.stats.ServerDrops, 1)
			atomic.AddInt64(&t.stats.Reconnects, 1)
			return nil // Return nil to allow session manager to reconnect
		}

		// If we received data, server is still alive, continue
	}
}

// holdForDuration holds the connection for the specified duration.
func (t *TCPFlood) holdForDuration(ctx context.Context, conn net.Conn) error {
	timer := time.NewTimer(t.config.HoldTime)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil
	case <-timer.C:
		atomic.AddInt64(&t.stats.Reconnects, 1)
		return nil
	}
}

// Name returns the strategy name.
func (t *TCPFlood) Name() string {
	return "tcp-flood"
}

// ActiveConnections returns the current number of active connections.
func (t *TCPFlood) ActiveConnections() int64 {
	return atomic.LoadInt64(&t.activeConnections)
}

// Stats returns the detailed statistics.
func (t *TCPFlood) Stats() *TCPFloodStats {
	return t.stats
}

// SetMetricsCallback sets the metrics callback for reporting.
func (t *TCPFlood) SetMetricsCallback(callback MetricsCallback) {
	t.metricsCallback = callback
}

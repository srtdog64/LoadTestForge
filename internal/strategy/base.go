package strategy

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/srtdog64/loadtestforge/internal/config"
	"github.com/srtdog64/loadtestforge/internal/httpdata"
	"github.com/srtdog64/loadtestforge/internal/netutil"
)

// =============================================================================
// CommonConfig - Shared configuration for all strategies
// =============================================================================

// CommonConfig holds configuration options shared across all attack strategies.
// Embed this in strategy-specific configs to inherit common options.
type CommonConfig struct {
	// Connection settings
	ConnectTimeout  time.Duration // Timeout for establishing connections
	SessionLifetime time.Duration // 0 = unlimited (hold until server closes)

	// Keep-alive settings
	KeepAliveInterval time.Duration // Interval for keep-alive/ping packets
	TCPKeepAlive      bool          // Enable TCP-level keep-alive

	// TLS settings
	TLSSkipVerify bool // Skip TLS certificate verification

	// Evasion settings
	EnableStealth bool // Browser fingerprint headers (Sec-Fetch-*)
	RandomizePath bool // Realistic query strings for cache bypass
}

// DefaultCommonConfig returns sensible defaults for CommonConfig.
func DefaultCommonConfig() CommonConfig {
	return CommonConfig{
		ConnectTimeout:    config.DefaultConnectTimeout,
		SessionLifetime:   config.DefaultSessionLifetime, // 0 = unlimited
		KeepAliveInterval: config.DefaultKeepAliveInterval,
		TCPKeepAlive:      true,
		TLSSkipVerify:     true, // Default to true for load testing
		EnableStealth:     false,
		RandomizePath:     false,
	}
}

// CommonConfigFromStrategyConfig creates CommonConfig from config.StrategyConfig.
func CommonConfigFromStrategyConfig(cfg *config.StrategyConfig) CommonConfig {
	return CommonConfig{
		ConnectTimeout:    cfg.Timeout,
		SessionLifetime:   cfg.SessionLifetime,
		KeepAliveInterval: cfg.KeepAliveInterval,
		TCPKeepAlive:      cfg.TCPKeepAlive,
		TLSSkipVerify:     cfg.TLSSkipVerify,
		EnableStealth:     cfg.EnableStealth,
		RandomizePath:     cfg.RandomizePath,
	}
}

// ToConnConfig converts CommonConfig to netutil.ConnConfig for a given bindIP.
func (c CommonConfig) ToConnConfig(bindIP string) netutil.ConnConfig {
	return netutil.ConnConfig{
		Timeout:        c.ConnectTimeout,
		MaxSessionLife: c.SessionLifetime,
		LocalAddr:      netutil.NewLocalTCPAddr(bindIP),
		BindConfig:     netutil.NewBindConfig(bindIP),
		WindowSize:     0,
		TLSSkipVerify:  c.TLSSkipVerify,
	}
}

// =============================================================================
// BaseStrategy - Common functionality for all strategies
// =============================================================================

// BaseStrategy provides common functionality for all attack strategies.
// Embed this struct in specific strategy implementations to get:
// - Connection tracking (active connection count)
// - Metrics callback support
// - Multi-IP binding configuration
// - Header randomization
// - Common configuration
type BaseStrategy struct {
	// Common configuration (shared across all strategies)
	Common CommonConfig

	// Connection binding configuration (single or multi-IP)
	BindConfig *netutil.BindConfig

	// Cached ConnConfig for DialManaged
	connConfig netutil.ConnConfig

	// Active connection counter (thread-safe)
	activeConnections int64

	// Metrics callback for telemetry
	metricsCallback MetricsCallback

	// Header randomizer for evasion
	headerRandomizer *httpdata.HeaderRandomizer
}

// NewBaseStrategy creates a new BaseStrategy with the given configuration.
func NewBaseStrategy(bindIP string, common CommonConfig) BaseStrategy {
	return BaseStrategy{
		Common:           common,
		BindConfig:       netutil.NewBindConfig(bindIP),
		connConfig:       common.ToConnConfig(bindIP),
		headerRandomizer: httpdata.DefaultHeaderRandomizer(),
	}
}

// NewBaseStrategySimple creates a BaseStrategy with minimal config (for backward compatibility).
func NewBaseStrategySimple(bindIP string, enableStealth, randomizePath bool) BaseStrategy {
	common := DefaultCommonConfig()
	common.EnableStealth = enableStealth
	common.RandomizePath = randomizePath
	return NewBaseStrategy(bindIP, common)
}

// NewBaseStrategyFromConfig creates a BaseStrategy from StrategyConfig.
func NewBaseStrategyFromConfig(cfg *config.StrategyConfig, bindIP string) BaseStrategy {
	b := NewBaseStrategy(bindIP, CommonConfigFromStrategyConfig(cfg))
	if b.BindConfig != nil {
		b.BindConfig.Random = cfg.BindRandom
	}
	return b
}

// SetMetricsCallback sets the metrics callback for telemetry.
// Implements MetricsAware interface.
func (b *BaseStrategy) SetMetricsCallback(callback MetricsCallback) {
	b.metricsCallback = callback
}

// GetMetricsCallback returns the current metrics callback.
func (b *BaseStrategy) GetMetricsCallback() MetricsCallback {
	return b.metricsCallback
}

// ActiveConnections returns the current number of active connections.
// Implements ConnectionTracker interface.
func (b *BaseStrategy) ActiveConnections() int64 {
	return atomic.LoadInt64(&b.activeConnections)
}

// IncrementConnections atomically increments the active connection count.
func (b *BaseStrategy) IncrementConnections() {
	atomic.AddInt64(&b.activeConnections, 1)
}

// DecrementConnections atomically decrements the active connection count.
func (b *BaseStrategy) DecrementConnections() {
	atomic.AddInt64(&b.activeConnections, -1)
}

// GetLocalAddr returns the next local address for binding (round-robin for multi-IP).
func (b *BaseStrategy) GetLocalAddr() *net.TCPAddr {
	if b.BindConfig == nil {
		return nil
	}
	return b.BindConfig.GetLocalAddr()
}

// GetHeaderRandomizer returns the header randomizer.
func (b *BaseStrategy) GetHeaderRandomizer() *httpdata.HeaderRandomizer {
	return b.headerRandomizer
}

// IsStealthEnabled returns whether stealth mode is enabled.
func (b *BaseStrategy) IsStealthEnabled() bool {
	return b.Common.EnableStealth
}

// IsPathRandomized returns whether path randomization is enabled.
func (b *BaseStrategy) IsPathRandomized() bool {
	return b.Common.RandomizePath
}

// OnDial is the standard hook for recording connection attempts.
func (b *BaseStrategy) OnDial() {
	if b.metricsCallback != nil {
		b.metricsCallback.RecordConnectionAttempt()
	}
}

// GetConnConfig returns the ConnConfig for DialManaged with OnDial hook.
func (b *BaseStrategy) GetConnConfig() netutil.ConnConfig {
	cfg := b.connConfig
	// Add OnDial hook for CPS tracking if metrics callback is set
	if b.metricsCallback != nil {
		cfg.OnDial = b.OnDial
	}
	return cfg
}

// GetDialerConfig returns a DialerConfig populated from the strategy's configuration and hooks.
func (b *BaseStrategy) GetDialerConfig() netutil.DialerConfig {
	return netutil.DialerConfig{
		Timeout:       b.Common.ConnectTimeout,
		KeepAlive:     b.Common.KeepAliveInterval,
		LocalAddr:     b.connConfig.LocalAddr,
		BindConfig:    b.BindConfig,
		TLSSkipVerify: b.Common.TLSSkipVerify,
		OnDial:        b.OnDial,
	}
}

// GetKeepAliveInterval returns the keep-alive interval.
func (b *BaseStrategy) GetKeepAliveInterval() time.Duration {
	return b.Common.KeepAliveInterval
}

// GetSessionLifetime returns the session lifetime (0 = unlimited).
func (b *BaseStrategy) GetSessionLifetime() time.Duration {
	return b.Common.SessionLifetime
}

// RecordLatency records a successful request with latency if metrics callback is set.
func (b *BaseStrategy) RecordLatency(duration time.Duration) {
	if b.metricsCallback != nil {
		b.metricsCallback.RecordSuccessWithLatency(duration)
	}
}

// RecordConnectionStart records the start of a new connection.
func (b *BaseStrategy) RecordConnectionStart(connID, remoteAddr string) {
	if b.metricsCallback != nil {
		b.metricsCallback.RecordConnectionStart(connID, remoteAddr)
	}
}

// RecordConnectionActivity records activity on an existing connection.
func (b *BaseStrategy) RecordConnectionActivity(connID string) {
	if b.metricsCallback != nil {
		b.metricsCallback.RecordConnectionActivity(connID)
	}
}

// RecordConnectionEnd records the end of a connection.
func (b *BaseStrategy) RecordConnectionEnd(connID string) {
	if b.metricsCallback != nil {
		b.metricsCallback.RecordConnectionEnd(connID)
	}
}

// RecordTimeout records a socket timeout event.
func (b *BaseStrategy) RecordTimeout() {
	if b.metricsCallback != nil {
		b.metricsCallback.RecordSocketTimeout()
	}
}

// RecordReconnect records a socket reconnection event.
func (b *BaseStrategy) RecordReconnect() {
	if b.metricsCallback != nil {
		b.metricsCallback.RecordSocketReconnect()
	}
}

// =============================================================================
// Connection Helpers
// =============================================================================

// DialTCP establishes a TCP connection with the configured binding.
func (b *BaseStrategy) DialTCP(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
	// Call OnDial hook for CPS tracking
	b.OnDial()
	return netutil.DialWithConfig(ctx, network, address, timeout, b.BindConfig)
}

// DialTCPWithDeadline establishes a TCP connection with an absolute deadline.
func (b *BaseStrategy) DialTCPWithDeadline(network, address string, deadline time.Time) (net.Conn, error) {
	timeout := time.Until(deadline)
	if timeout <= 0 {
		return nil, fmt.Errorf("deadline already passed")
	}
	return netutil.DialWithConfig(context.Background(), network, address, timeout, b.BindConfig)
}

// =============================================================================
// Request Building Helpers
// =============================================================================

// BuildGETRequest builds a GET request with randomized headers.
func (b *BaseStrategy) BuildGETRequest(parsedURL *url.URL, userAgent string) string {
	if b.headerRandomizer != nil {
		return b.headerRandomizer.BuildGETRequest(parsedURL, userAgent)
	}
	return buildSimpleGETRequest(parsedURL, userAgent)
}

// BuildPOSTRequest builds a POST request with randomized headers.
func (b *BaseStrategy) BuildPOSTRequest(parsedURL *url.URL, userAgent string, contentLength int, contentType string) string {
	if b.headerRandomizer != nil {
		return b.headerRandomizer.BuildPOSTRequest(parsedURL, userAgent, contentLength, contentType)
	}
	return buildSimplePOSTRequest(parsedURL, userAgent, contentLength, contentType)
}

// BuildIncompleteRequest builds an incomplete request for Slowloris attacks.
func (b *BaseStrategy) BuildIncompleteRequest(parsedURL *url.URL, userAgent string) string {
	if b.headerRandomizer != nil {
		return b.headerRandomizer.BuildIncompleteRequest(parsedURL, userAgent)
	}
	return buildSimpleIncompleteRequest(parsedURL, userAgent)
}

// GetRandomizedPath returns the path with optional randomization.
func (b *BaseStrategy) GetRandomizedPath(basePath string) string {
	if !b.Common.RandomizePath {
		if basePath == "" {
			return "/"
		}
		return basePath
	}

	randomizer := httpdata.DefaultPathRandomizer()
	return randomizer.RandomizePath(basePath)
}

// =============================================================================
// Connection ID Generation
// =============================================================================

// generateConnID generates a unique connection ID for logging.
func generateConnID() string {
	return httpdata.GenerateSessionID()[:8]
}

// =============================================================================
// HTTP Response Helpers
// =============================================================================

// IsHTTPSuccess checks if the status code indicates success.
func IsHTTPSuccess(statusCode int) bool {
	return statusCode > 0 && statusCode < config.HTTPSuccessThreshold
}

// IsHTTPError checks if the status code indicates an error.
func IsHTTPError(statusCode int) bool {
	return statusCode >= config.HTTPSuccessThreshold
}

// =============================================================================
// Simple Request Builders (fallback when randomizer is nil)
// =============================================================================

func buildSimpleGETRequest(parsedURL *url.URL, userAgent string) string {
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}
	return fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUser-Agent: %s\r\nAccept: */*\r\nConnection: keep-alive\r\n\r\n",
		path, parsedURL.Host, userAgent)
}

func buildSimplePOSTRequest(parsedURL *url.URL, userAgent string, contentLength int, contentType string) string {
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}
	return fmt.Sprintf("POST %s HTTP/1.1\r\nHost: %s\r\nUser-Agent: %s\r\nContent-Type: %s\r\nContent-Length: %d\r\nAccept: */*\r\nConnection: keep-alive\r\n\r\n",
		path, parsedURL.Host, userAgent, contentType, contentLength)
}

func buildSimpleIncompleteRequest(parsedURL *url.URL, userAgent string) string {
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}
	return fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUser-Agent: %s\r\nAccept: */*\r\nConnection: keep-alive\r\n",
		path, parsedURL.Host, userAgent)
}

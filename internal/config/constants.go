package config

import "time"

// =============================================================================
// Network Constants
// =============================================================================

const (
	// DefaultConnectTimeout is the default timeout for establishing connections
	DefaultConnectTimeout = 10 * time.Second

	// DefaultReadTimeout is the default timeout for read operations
	DefaultReadTimeout = 30 * time.Second

	// DefaultWriteTimeout is the default timeout for write operations
	DefaultWriteTimeout = 10 * time.Second

	// DefaultKeepAliveInterval is the default interval for keep-alive pings
	DefaultKeepAliveInterval = 10 * time.Second

	// DefaultTCPKeepAlive is the TCP keep-alive period
	DefaultTCPKeepAlive = 30 * time.Second
)

// =============================================================================
// Session Management Constants
// =============================================================================

const (
	// DefaultMaxConsecutiveFailures is the default number of consecutive failures
	// before a session is terminated
	DefaultMaxConsecutiveFailures = 5

	// DefaultSessionsPerSec is the default rate of session creation
	DefaultSessionsPerSec = 10

	// DefaultTargetSessions is the default number of target concurrent sessions
	DefaultTargetSessions = 100

	// SessionTickInterval is the interval for session management ticks
	SessionTickInterval = 100 * time.Millisecond

	// PulseTickInterval is the interval for pulse mode ticks
	PulseTickInterval = 50 * time.Millisecond

	// ConnectionTrackInterval is the interval for tracking active connections
	ConnectionTrackInterval = 500 * time.Millisecond

	// SpawnBurstMultiplier is the multiplier for max sessions creatable per tick
	SpawnBurstMultiplier = 1.5

	// PruneDampingFactor is the damping factor for pruning sessions (50%)
	PruneDampingFactor = 0.5
)

// =============================================================================
// HTTP Constants
// =============================================================================

const (
	// DefaultContentLength is the default Content-Length for slow POST attacks
	DefaultContentLength = 100000

	// DefaultPostDataSize is the default POST data size for HTTP flood
	DefaultPostDataSize = 1024

	// DefaultRequestsPerConn is the default number of requests per connection
	DefaultRequestsPerConn = 100

	// HTTPSuccessThreshold is the HTTP status code threshold for success (< 400)
	HTTPSuccessThreshold = 400

	// DefaultUserAgent is the default User-Agent header
	DefaultUserAgent = "LoadTestForge/1.0"
)

// =============================================================================
// Slow Attack Constants
// =============================================================================

const (
	// DefaultReadSize is the default bytes to read per iteration for slow-read
	DefaultReadSize = 1

	// DefaultWindowSize is the default TCP window size for slow-read
	DefaultWindowSize = 64

	// MinSlowDelay is the minimum delay for slow attacks
	MinSlowDelay = 50 * time.Millisecond

	// MaxSlowDelay is the maximum delay for slow attacks
	MaxSlowDelay = 200 * time.Millisecond

	// SlowlorisHeaderDelay is the delay between slowloris header sends
	SlowlorisHeaderDelay = 10 * time.Second
)

// =============================================================================
// HTTP/2 Constants
// =============================================================================

const (
	// DefaultMaxStreams is the default max concurrent streams per connection
	DefaultMaxStreams = 100

	// DefaultBurstSize is the default stream burst size
	DefaultBurstSize = 10

	// H2StreamResetThreshold is the threshold for stream failures before reconnect
	H2StreamResetThreshold = 10
)

// =============================================================================
// Heavy Payload Constants
// =============================================================================

const (
	// DefaultPayloadDepth is the default nesting depth for heavy payloads
	DefaultPayloadDepth = 50

	// DefaultPayloadSize is the default payload size
	DefaultPayloadSize = 10000

	// PayloadTypeDeepJSON is the deep JSON payload type
	PayloadTypeDeepJSON = "deep-json"

	// PayloadTypeReDoS is the ReDoS payload type
	PayloadTypeReDoS = "redos"

	// PayloadTypeNestedXML is the nested XML payload type
	PayloadTypeNestedXML = "nested-xml"

	// PayloadTypeQueryFlood is the query flood payload type
	PayloadTypeQueryFlood = "query-flood"

	// PayloadTypeMultipart is the multipart payload type
	PayloadTypeMultipart = "multipart"
)

// =============================================================================
// RUDY Constants
// =============================================================================

const (
	// DefaultChunkDelayMin is the minimum delay between chunks
	DefaultChunkDelayMin = 1 * time.Second

	// DefaultChunkDelayMax is the maximum delay between chunks
	DefaultChunkDelayMax = 5 * time.Second

	// DefaultChunkSizeMin is the minimum chunk size in bytes
	DefaultChunkSizeMin = 1

	// DefaultChunkSizeMax is the maximum chunk size in bytes
	DefaultChunkSizeMax = 100

	// DefaultMaxReqPerSession is the default max requests per session
	DefaultMaxReqPerSession = 10

	// DefaultKeepAliveTimeout is the default keep-alive timeout
	DefaultKeepAliveTimeout = 600 * time.Second

	// DefaultSessionLifetime is the default session lifetime
	DefaultSessionLifetime = 3600 * time.Second

	// DefaultSendBufferSize is the default TCP send buffer size
	DefaultSendBufferSize = 1024

	// EvasionLevelBasic is the basic evasion level
	EvasionLevelBasic = 1

	// EvasionLevelNormal is the normal evasion level
	EvasionLevelNormal = 2

	// EvasionLevelAggressive is the aggressive evasion level
	EvasionLevelAggressive = 3
)

// =============================================================================
// Pulse Mode Constants
// =============================================================================

const (
	// DefaultPulseHighTime is the default duration of high load phase
	DefaultPulseHighTime = 30 * time.Second

	// DefaultPulseLowTime is the default duration of low load phase
	DefaultPulseLowTime = 30 * time.Second

	// DefaultPulseLowRatio is the default session ratio during low phase
	DefaultPulseLowRatio = 0.1

	// WaveTypeSquare is the square wave type
	WaveTypeSquare = "square"

	// WaveTypeSine is the sine wave type
	WaveTypeSine = "sine"

	// WaveTypeSawtooth is the sawtooth wave type
	WaveTypeSawtooth = "sawtooth"
)

// =============================================================================
// Metrics Constants
// =============================================================================

const (
	// DefaultReportInterval is the default interval for metrics reporting
	DefaultReportInterval = 2 * time.Second

	// SuccessRateThreshold is the minimum success rate for passing (90%)
	SuccessRateThreshold = 0.90

	// DeviationThreshold is the maximum allowed rate deviation (20%)
	DeviationThreshold = 0.20

	// P99LatencyThreshold is the maximum p99 latency in milliseconds
	P99LatencyThreshold = 5000.0

	// TimeoutRateThreshold is the maximum timeout rate (10%)
	TimeoutRateThreshold = 0.10

	// LatencySampleSize is the number of latency samples to keep
	LatencySampleSize = 10000
)

// =============================================================================
// Backoff Constants
// =============================================================================

const (
	// BaseBackoffDelay is the base delay for exponential backoff
	BaseBackoffDelay = 1 * time.Second

	// MaxBackoffDelay is the maximum backoff delay
	MaxBackoffDelay = 30 * time.Second

	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier = 2.0

	// BackoffJitterRatio is the jitter ratio for backoff (0-1)
	BackoffJitterRatio = 0.1
)

// =============================================================================
// Buffer Size Constants
// =============================================================================

const (
	// DefaultReadBufferSize is the default buffer size for reading
	DefaultReadBufferSize = 4096

	// DefaultWriteBufferSize is the default buffer size for writing
	DefaultWriteBufferSize = 4096

	// SessionIDLength is the length of session ID in bytes
	SessionIDLength = 8

	// ChunkBufferSize is the buffer size for chunked encoding
	ChunkBufferSize = 1024
)

// =============================================================================
// Retry Constants
// =============================================================================

const (
	// QuickRetryDelay is the delay after successful request
	QuickRetryDelay = 50 * time.Millisecond

	// ReconnectDelay is the delay before reconnection attempt
	ReconnectDelay = 100 * time.Millisecond

	// MaxReconnectAttempts is the maximum number of reconnection attempts
	MaxReconnectAttempts = 3
)

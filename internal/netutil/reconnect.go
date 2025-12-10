package netutil

import (
	"context"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/jdw/loadtestforge/internal/httpdata"
)

// ReconnectConfig defines reconnection behavior parameters.
type ReconnectConfig struct {
	BaseBackoff           time.Duration
	MaxBackoff            time.Duration
	BackoffMultiplier     float64
	JitterFactor          float64
	MaxConsecutiveErrors  int
	ErrorThrottleInterval time.Duration
}

// DefaultReconnectConfig returns sensible defaults for reconnection.
func DefaultReconnectConfig() ReconnectConfig {
	return ReconnectConfig{
		BaseBackoff:           500 * time.Millisecond,
		MaxBackoff:            5 * time.Second,
		BackoffMultiplier:     1.5,
		JitterFactor:          0.3,
		MaxConsecutiveErrors:  10,
		ErrorThrottleInterval: 60 * time.Second,
	}
}

// ReconnectState tracks reconnection state for a single worker.
type ReconnectState struct {
	ConsecutiveErrors int
	CurrentBackoff    time.Duration
	LastErrorLogTime  time.Time
	Config            ReconnectConfig
}

// NewReconnectState creates a new reconnect state with the given config.
func NewReconnectState(cfg ReconnectConfig) *ReconnectState {
	return &ReconnectState{
		ConsecutiveErrors: 0,
		CurrentBackoff:    cfg.BaseBackoff,
		Config:            cfg,
	}
}

// RecordSuccess resets the reconnect state on successful operation.
func (s *ReconnectState) RecordSuccess() {
	s.ConsecutiveErrors = 0
	s.CurrentBackoff = s.Config.BaseBackoff
}

// RecordError records an error and returns whether the worker should terminate.
func (s *ReconnectState) RecordError() bool {
	s.ConsecutiveErrors++
	return s.ConsecutiveErrors >= s.Config.MaxConsecutiveErrors
}

// ShouldLogError returns true if enough time has passed to log another error.
func (s *ReconnectState) ShouldLogError() bool {
	now := time.Now()
	if now.Sub(s.LastErrorLogTime) > s.Config.ErrorThrottleInterval {
		s.LastErrorLogTime = now
		return true
	}
	return false
}

// CalculateBackoff returns the next backoff duration with jitter.
func (s *ReconnectState) CalculateBackoff() time.Duration {
	backoff := s.CurrentBackoff

	jitter := time.Duration(float64(backoff) * s.Config.JitterFactor * (rand.Float64()*2 - 1))
	backoff += jitter

	s.CurrentBackoff = time.Duration(float64(s.CurrentBackoff) * s.Config.BackoffMultiplier)
	if s.CurrentBackoff > s.Config.MaxBackoff {
		s.CurrentBackoff = s.Config.MaxBackoff
	}

	return backoff
}

// WaitBackoff waits for the calculated backoff duration or until context is cancelled.
// Returns true if context was cancelled during wait.
func (s *ReconnectState) WaitBackoff(ctx context.Context) bool {
	backoff := s.CalculateBackoff()
	select {
	case <-ctx.Done():
		return true
	case <-time.After(backoff):
		return false
	}
}

// ReconnectMetrics provides atomic counters for reconnection statistics.
type ReconnectMetrics struct {
	Timeouts   int64
	Reconnects int64
	Errors     int64
}

// IncrementTimeout atomically increments the timeout counter.
func (m *ReconnectMetrics) IncrementTimeout() {
	atomic.AddInt64(&m.Timeouts, 1)
}

// IncrementReconnect atomically increments the reconnect counter.
func (m *ReconnectMetrics) IncrementReconnect() {
	atomic.AddInt64(&m.Reconnects, 1)
}

// IncrementError atomically increments the error counter.
func (m *ReconnectMetrics) IncrementError() {
	atomic.AddInt64(&m.Errors, 1)
}

// Snapshot returns current values without modifying them.
func (m *ReconnectMetrics) Snapshot() (timeouts, reconnects, errors int64) {
	return atomic.LoadInt64(&m.Timeouts),
		atomic.LoadInt64(&m.Reconnects),
		atomic.LoadInt64(&m.Errors)
}

// SessionPersistence manages session state for persistent connections.
type SessionPersistence struct {
	SessionID    string
	Cookies      []string
	RequestCount int
	MaxRequests  int
	CreatedAt    time.Time
	LastActivity time.Time
}

// NewSessionPersistence creates a new session with the given maximum request count.
func NewSessionPersistence(maxRequests int) *SessionPersistence {
	now := time.Now()
	return &SessionPersistence{
		SessionID:    httpdata.GenerateSessionID(),
		Cookies:      make([]string, 0, 4),
		RequestCount: 0,
		MaxRequests:  maxRequests,
		CreatedAt:    now,
		LastActivity: now,
	}
}

// IncrementRequests increments the request count and updates last activity.
// Returns true if session should be renewed.
func (s *SessionPersistence) IncrementRequests() bool {
	s.RequestCount++
	s.LastActivity = time.Now()
	return s.RequestCount >= s.MaxRequests
}

// AddCookie adds a cookie if it is not already present.
func (s *SessionPersistence) AddCookie(cookie string) {
	for _, c := range s.Cookies {
		if c == cookie {
			return
		}
	}
	s.Cookies = append(s.Cookies, cookie)
}

// Reset resets the session for reuse with a new ID.
func (s *SessionPersistence) Reset() {
	s.SessionID = httpdata.GenerateSessionID()
	s.Cookies = s.Cookies[:0]
	s.RequestCount = 0
	s.CreatedAt = time.Now()
	s.LastActivity = s.CreatedAt
}

// Duration returns how long the session has been alive.
func (s *SessionPersistence) Duration() time.Duration {
	return time.Since(s.CreatedAt)
}

// IdleDuration returns how long since last activity.
func (s *SessionPersistence) IdleDuration() time.Duration {
	return time.Since(s.LastActivity)
}

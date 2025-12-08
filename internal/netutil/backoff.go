package netutil

import (
	"math"
	"math/rand"
	"time"

	"github.com/jdw/loadtestforge/internal/config"
)

// Backoff provides exponential backoff with jitter for retry operations.
type Backoff struct {
	// Base delay for first retry
	BaseDelay time.Duration

	// Maximum delay cap
	MaxDelay time.Duration

	// Multiplier for each retry (typically 2.0)
	Multiplier float64

	// Jitter ratio (0.0-1.0) for randomization
	JitterRatio float64

	// Current retry count
	attempt int
}

// DefaultBackoff returns a backoff with default configuration.
func DefaultBackoff() *Backoff {
	return &Backoff{
		BaseDelay:   config.BaseBackoffDelay,
		MaxDelay:    config.MaxBackoffDelay,
		JitterRatio: config.BackoffJitterRatio,
		Multiplier:  config.BackoffMultiplier,
		attempt:     0,
	}
}

// NewBackoff creates a backoff with custom configuration.
func NewBackoff(baseDelay, maxDelay time.Duration, multiplier, jitterRatio float64) *Backoff {
	return &Backoff{
		BaseDelay:   baseDelay,
		MaxDelay:    maxDelay,
		Multiplier:  multiplier,
		JitterRatio: jitterRatio,
		attempt:     0,
	}
}

// LinearBackoff creates a simple linear backoff (delay * attempt).
func LinearBackoff(baseDelay, maxDelay time.Duration) *Backoff {
	return &Backoff{
		BaseDelay:   baseDelay,
		MaxDelay:    maxDelay,
		Multiplier:  1.0, // Linear
		JitterRatio: 0.0,
		attempt:     0,
	}
}

// Next returns the next backoff delay and increments the attempt counter.
func (b *Backoff) Next() time.Duration {
	b.attempt++
	return b.Calculate(b.attempt)
}

// Calculate returns the backoff delay for a specific attempt number.
func (b *Backoff) Calculate(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Calculate base delay with exponential growth
	delay := float64(b.BaseDelay) * math.Pow(b.Multiplier, float64(attempt-1))

	// Apply jitter
	if b.JitterRatio > 0 {
		jitter := delay * b.JitterRatio * (rand.Float64()*2 - 1) // -jitter to +jitter
		delay += jitter
	}

	// Cap at max delay
	if delay > float64(b.MaxDelay) {
		delay = float64(b.MaxDelay)
	}

	// Ensure positive
	if delay < 0 {
		delay = float64(b.BaseDelay)
	}

	return time.Duration(delay)
}

// Reset resets the attempt counter.
func (b *Backoff) Reset() {
	b.attempt = 0
}

// Attempt returns the current attempt number.
func (b *Backoff) Attempt() int {
	return b.attempt
}

// =============================================================================
// Convenience Functions
// =============================================================================

// CalculateBackoff returns the backoff delay for consecutive failures.
// This is a simple function for one-off calculations.
func CalculateBackoff(consecutiveFailures int) time.Duration {
	if consecutiveFailures <= 0 {
		return 0
	}

	delay := time.Duration(consecutiveFailures) * config.BaseBackoffDelay
	if delay > config.MaxBackoffDelay {
		delay = config.MaxBackoffDelay
	}
	return delay
}

// CalculateExponentialBackoff returns exponential backoff delay.
func CalculateExponentialBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	if attempt <= 0 {
		return 0
	}

	delay := float64(baseDelay) * math.Pow(2.0, float64(attempt-1))
	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}

	// Add jitter (10%)
	jitter := delay * 0.1 * rand.Float64()
	delay += jitter

	return time.Duration(delay)
}

// RandomDelay returns a random delay between min and max.
func RandomDelay(min, max time.Duration) time.Duration {
	if min >= max {
		return min
	}
	return min + time.Duration(rand.Int63n(int64(max-min)))
}

// RandomDelayWithJitter returns a base delay with random jitter.
func RandomDelayWithJitter(base time.Duration, jitterRatio float64) time.Duration {
	if jitterRatio <= 0 {
		return base
	}

	jitter := float64(base) * jitterRatio * (rand.Float64()*2 - 1)
	result := float64(base) + jitter

	if result < 0 {
		return base
	}
	return time.Duration(result)
}

// =============================================================================
// Retry Helper
// =============================================================================

// RetryConfig holds configuration for retry operations.
type RetryConfig struct {
	MaxAttempts int
	Backoff     *Backoff
}

// DefaultRetryConfig returns a retry configuration with defaults.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts: config.MaxReconnectAttempts,
		Backoff:     DefaultBackoff(),
	}
}

// ShouldRetry returns whether another retry should be attempted.
func (r *RetryConfig) ShouldRetry() bool {
	return r.Backoff.Attempt() < r.MaxAttempts
}

// NextDelay returns the delay before the next retry.
func (r *RetryConfig) NextDelay() time.Duration {
	return r.Backoff.Next()
}

// Reset resets the retry state.
func (r *RetryConfig) Reset() {
	r.Backoff.Reset()
}

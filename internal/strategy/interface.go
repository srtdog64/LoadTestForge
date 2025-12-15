package strategy

import (
	"context"
	"time"
)

// Target represents the attack target configuration.
type Target struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    []byte
}

// AttackStrategy defines the interface for all attack strategies.
type AttackStrategy interface {
	Execute(ctx context.Context, target Target) error
	Name() string
}

// MetricsCallback provides callbacks for metrics collection.
type MetricsCallback interface {
	RecordConnectionStart(connID, remoteAddr string)
	RecordConnectionActivity(connID string)
	RecordConnectionEnd(connID string)
	RecordSocketTimeout()
	RecordSocketReconnect()
	RecordConnectionAttempt()
	RecordSuccessWithLatency(duration time.Duration)
}

// MetricsAware indicates a strategy supports metrics callbacks.
type MetricsAware interface {
	SetMetricsCallback(callback MetricsCallback)
}

// ConnectionTracker indicates a strategy tracks active connections.
type ConnectionTracker interface {
	ActiveConnections() int64
}

// Result represents the outcome of a single request.
type Result struct {
	Success      bool
	Error        error
	ResponseTime int64
	StatusCode   int
}

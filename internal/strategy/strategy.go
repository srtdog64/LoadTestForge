package strategy

import (
	"context"
	"net"
	"time"
)

type Target struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    []byte
}

type AttackStrategy interface {
	Execute(ctx context.Context, target Target) error
	Name() string
}

type MetricsCallback interface {
	RecordConnectionStart(connID, remoteAddr string)
	RecordConnectionActivity(connID string)
	RecordConnectionEnd(connID string)
	RecordSocketTimeout()
	RecordSocketReconnect()
	RecordSuccessWithLatency(duration time.Duration)
}

type MetricsAware interface {
	SetMetricsCallback(callback MetricsCallback)
}

type ConnectionTracker interface {
	ActiveConnections() int64
}

type Result struct {
	Success      bool
	Error        error
	ResponseTime int64
	StatusCode   int
}

func newLocalTCPAddr(bindIP string) *net.TCPAddr {
	if bindIP == "" {
		return nil
	}

	ip := net.ParseIP(bindIP)
	if ip == nil {
		return nil
	}

	return &net.TCPAddr{IP: ip}
}

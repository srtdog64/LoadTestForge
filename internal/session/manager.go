package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jdw/loadtestforge/internal/metrics"
	"github.com/jdw/loadtestforge/internal/strategy"
	"golang.org/x/time/rate"
)

type Manager struct {
	strategy       strategy.AttackStrategy
	target         strategy.Target
	targetSessions int
	sessionsPerSec int
	rampUpDuration time.Duration
	limiter        *rate.Limiter
	metrics        *metrics.Collector

	activeSessions int32
	mu             sync.Mutex
	sessions       map[string]context.CancelFunc
}

func NewManager(
	strat strategy.AttackStrategy,
	target strategy.Target,
	targetSessions int,
	sessionsPerSec int,
	rampUpDuration time.Duration,
	metricsCollector *metrics.Collector,
) *Manager {
	return &Manager{
		strategy:       strat,
		target:         target,
		targetSessions: targetSessions,
		sessionsPerSec: sessionsPerSec,
		rampUpDuration: rampUpDuration,
		limiter:        rate.NewLimiter(rate.Limit(sessionsPerSec), sessionsPerSec),
		metrics:        metricsCollector,
		sessions:       make(map[string]context.CancelFunc),
	}
}

func (m *Manager) Run(ctx context.Context) error {
	if m.rampUpDuration > 0 {
		return m.runWithRampUp(ctx)
	}
	return m.runSteadyState(ctx)
}

func (m *Manager) runWithRampUp(ctx context.Context) error {
	startTime := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.shutdownAll()
			return ctx.Err()
		case <-ticker.C:
			elapsed := time.Since(startTime)
			
			var currentTarget int
			if elapsed < m.rampUpDuration {
				progress := float64(elapsed) / float64(m.rampUpDuration)
				currentTarget = int(float64(m.targetSessions) * progress)
				if currentTarget < 1 {
					currentTarget = 1
				}
			} else {
				currentTarget = m.targetSessions
			}

			current := atomic.LoadInt32(&m.activeSessions)
			if int(current) < currentTarget {
				if err := m.limiter.Wait(ctx); err != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					continue
				}
				go m.launchSession(ctx)
			}
		}
	}
}

func (m *Manager) runSteadyState(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.shutdownAll()
			return ctx.Err()
		case <-ticker.C:
			current := atomic.LoadInt32(&m.activeSessions)
			if int(current) < m.targetSessions {
				if err := m.limiter.Wait(ctx); err != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					continue
				}
				go m.launchSession(ctx)
			}
		}
	}
}

func (m *Manager) launchSession(parentCtx context.Context) {
	sessionID := generateSessionID()
	ctx, cancel := context.WithCancel(parentCtx)

	m.mu.Lock()
	m.sessions[sessionID] = cancel
	m.mu.Unlock()

	atomic.AddInt32(&m.activeSessions, 1)
	m.metrics.IncrementActive()

	defer func() {
		atomic.AddInt32(&m.activeSessions, -1)
		m.metrics.DecrementActive()

		m.mu.Lock()
		delete(m.sessions, sessionID)
		m.mu.Unlock()
	}()

	consecutiveFailures := 0
	maxConsecutiveFailures := 5

	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := m.strategy.Execute(ctx, m.target)
			if err != nil {
				m.metrics.RecordFailure()
				consecutiveFailures++

				if consecutiveFailures >= maxConsecutiveFailures {
					return
				}

				backoff := time.Duration(consecutiveFailures) * time.Second
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
					continue
				}
			} else {
				m.metrics.RecordSuccess()
				consecutiveFailures = 0
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
}

func (m *Manager) shutdownAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cancel := range m.sessions {
		cancel()
	}
}

func (m *Manager) GetMetrics() *metrics.Collector {
	return m.metrics
}

func generateSessionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

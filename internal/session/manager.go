package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
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

	// Pulsing pattern support
	pulseEnabled   bool
	pulseHighTime  time.Duration
	pulseLowTime   time.Duration
	pulseLowRatio  float64 // e.g., 0.1 means 10% of target during low phase
	pulseWaveType  string  // "square", "sine", "sawtooth"

	activeSessions int32
	mu             sync.Mutex
	sessions       map[string]context.CancelFunc
}

type PulseConfig struct {
	Enabled  bool
	HighTime time.Duration
	LowTime  time.Duration
	LowRatio float64
	WaveType string
}

func NewManager(
	strat strategy.AttackStrategy,
	target strategy.Target,
	targetSessions int,
	sessionsPerSec int,
	rampUpDuration time.Duration,
	metricsCollector *metrics.Collector,
) *Manager {
	return NewManagerWithPulse(strat, target, targetSessions, sessionsPerSec, rampUpDuration, metricsCollector, PulseConfig{})
}

func NewManagerWithPulse(
	strat strategy.AttackStrategy,
	target strategy.Target,
	targetSessions int,
	sessionsPerSec int,
	rampUpDuration time.Duration,
	metricsCollector *metrics.Collector,
	pulse PulseConfig,
) *Manager {
	m := &Manager{
		strategy:       strat,
		target:         target,
		targetSessions: targetSessions,
		sessionsPerSec: sessionsPerSec,
		rampUpDuration: rampUpDuration,
		limiter:        rate.NewLimiter(rate.Limit(sessionsPerSec), sessionsPerSec),
		metrics:        metricsCollector,
		sessions:       make(map[string]context.CancelFunc),
		pulseEnabled:   pulse.Enabled,
		pulseHighTime:  pulse.HighTime,
		pulseLowTime:   pulse.LowTime,
		pulseLowRatio:  pulse.LowRatio,
		pulseWaveType:  pulse.WaveType,
	}

	if m.pulseLowRatio <= 0 {
		m.pulseLowRatio = 0.1
	}
	if m.pulseWaveType == "" {
		m.pulseWaveType = "square"
	}

	if metricsAware, ok := strat.(strategy.MetricsAware); ok {
		metricsAware.SetMetricsCallback(metricsCollector)
	}

	return m
}

func (m *Manager) Run(ctx context.Context) error {
	if tracker, ok := m.strategy.(strategy.ConnectionTracker); ok {
		go m.trackConnections(ctx, tracker)
	}

	if m.pulseEnabled {
		return m.runWithPulse(ctx)
	}
	if m.rampUpDuration > 0 {
		return m.runWithRampUp(ctx)
	}
	return m.runSteadyState(ctx)
}

func (m *Manager) trackConnections(ctx context.Context, tracker strategy.ConnectionTracker) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.metrics.SetTCPConnections(tracker.ActiveConnections())
		}
	}
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

func (m *Manager) runWithPulse(ctx context.Context) error {
	cycleStart := time.Now()
	isHighPhase := true

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.shutdownAll()
			return ctx.Err()
		case <-ticker.C:
			elapsed := time.Since(cycleStart)

			// Phase transition check
			if isHighPhase && elapsed > m.pulseHighTime {
				isHighPhase = false
				cycleStart = time.Now()
			} else if !isHighPhase && elapsed > m.pulseLowTime {
				isHighPhase = true
				cycleStart = time.Now()
			}

			// Calculate current target based on wave type
			currentTarget := m.calculatePulseTarget(isHighPhase, elapsed)
			current := int(atomic.LoadInt32(&m.activeSessions))

			// Scale UP: create sessions if below target
			if current < currentTarget {
				needed := currentTarget - current
				for i := 0; i < needed; i++ {
					if err := m.limiter.Wait(ctx); err != nil {
						if ctx.Err() != nil {
							return ctx.Err()
						}
						break
					}
					go m.launchSession(ctx)
				}
			}

			// Scale DOWN: prune sessions if above target (Hard Kill)
			// Apply damping factor (50%) to prevent overshooting
			if current > currentTarget {
				excess := current - currentTarget
				pruneCount := (excess + 1) / 2 // 50% damping to prevent overshooting
				if pruneCount < 1 {
					pruneCount = 1
				}
				m.pruneSessions(pruneCount)
			}
		}
	}
}

// pruneSessions forcefully terminates the specified number of sessions.
// This simulates client disconnection (Hard Kill) for stress testing.
func (m *Manager) pruneSessions(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pruned := 0
	for id, cancel := range m.sessions {
		if pruned >= count {
			break
		}
		cancel()
		// Note: Actual map deletion and counter decrement happens in launchSession's defer.
		// We only send the cancel signal here.
		_ = id // suppress unused warning
		pruned++
	}
}

func (m *Manager) calculatePulseTarget(isHigh bool, elapsed time.Duration) int {
	highTarget := m.targetSessions
	lowTarget := int(float64(m.targetSessions) * m.pulseLowRatio)
	if lowTarget < 1 {
		lowTarget = 1
	}

	switch m.pulseWaveType {
	case "sine":
		// Sine wave oscillation
		var phaseDuration time.Duration
		if isHigh {
			phaseDuration = m.pulseHighTime
		} else {
			phaseDuration = m.pulseLowTime
		}

		progress := float64(elapsed) / float64(phaseDuration)
		if progress > 1 {
			progress = 1
		}

		// Sine interpolation between high and low
		if isHigh {
			// High phase: start high, potentially dip slightly
			return highTarget
		} else {
			// Low phase: use sine to smoothly transition
			sineValue := (1 + math.Sin(progress*math.Pi-math.Pi/2)) / 2
			return lowTarget + int(float64(highTarget-lowTarget)*sineValue)
		}

	case "sawtooth":
		// Sawtooth: gradual rise, sudden drop
		if isHigh {
			progress := float64(elapsed) / float64(m.pulseHighTime)
			if progress > 1 {
				progress = 1
			}
			return lowTarget + int(float64(highTarget-lowTarget)*progress)
		}
		return lowTarget

	case "square":
		fallthrough
	default:
		// Square wave: instant transition
		if isHigh {
			return highTarget
		}
		return lowTarget
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

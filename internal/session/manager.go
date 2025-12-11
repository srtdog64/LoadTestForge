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

	"github.com/srtdog64/loadtestforge/internal/config"
	"github.com/srtdog64/loadtestforge/internal/metrics"
	"github.com/srtdog64/loadtestforge/internal/strategy"
	"golang.org/x/time/rate"
)

type Manager struct {
	strategy strategy.AttackStrategy
	target   strategy.Target
	perf     config.PerformanceConfig
	limiter  *rate.Limiter
	metrics  *metrics.Collector

	activeSessions int32
	mu             sync.Mutex
	sessions       map[string]context.CancelFunc
}

func NewManager(
	strat strategy.AttackStrategy,
	target strategy.Target,
	perf config.PerformanceConfig,
	metricsCollector *metrics.Collector,
) *Manager {
	m := &Manager{
		strategy: strat,
		target:   target,
		perf:     perf,
		limiter:  rate.NewLimiter(rate.Limit(perf.SessionsPerSec), perf.SessionsPerSec),
		metrics:  metricsCollector,
		sessions: make(map[string]context.CancelFunc),
	}

	if m.perf.Pulse.LowRatio <= 0 {
		m.perf.Pulse.LowRatio = config.DefaultPulseLowRatio
	}
	if m.perf.Pulse.WaveType == "" {
		m.perf.Pulse.WaveType = config.WaveTypeSquare
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

	if m.perf.Pulse.Enabled {
		return m.runWithPulse(ctx)
	}
	if m.perf.RampUpDuration > 0 {
		return m.runWithRampUp(ctx)
	}
	return m.runSteadyState(ctx)
}

func (m *Manager) trackConnections(ctx context.Context, tracker strategy.ConnectionTracker) {
	ticker := time.NewTicker(config.ConnectionTrackInterval)
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
	tickInterval := config.SessionTickInterval
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.shutdownAll()
			return ctx.Err()
		case <-ticker.C:
			elapsed := time.Since(startTime)

			var currentTarget int
			if elapsed < m.perf.RampUpDuration {
				progress := float64(elapsed) / float64(m.perf.RampUpDuration)
				currentTarget = int(float64(m.perf.TargetSessions) * progress)
				if currentTarget < 1 {
					currentTarget = 1
				}
			} else {
				currentTarget = m.perf.TargetSessions
			}

			current := int(atomic.LoadInt32(&m.activeSessions))
			if current < currentTarget {
				m.spawnSessions(ctx, currentTarget-current, tickInterval)
			}
		}
	}
}

func (m *Manager) runWithPulse(ctx context.Context) error {
	cycleStart := time.Now()
	isHighPhase := true

	tickInterval := config.PulseTickInterval
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.shutdownAll()
			return ctx.Err()
		case <-ticker.C:
			elapsed := time.Since(cycleStart)

			// Phase transition check
			if isHighPhase && elapsed > m.perf.Pulse.HighTime {
				isHighPhase = false
				cycleStart = time.Now()
			} else if !isHighPhase && elapsed > m.perf.Pulse.LowTime {
				isHighPhase = true
				cycleStart = time.Now()
			}

			// Calculate current target based on wave type
			currentTarget := m.calculatePulseTarget(isHighPhase, elapsed)
			current := int(atomic.LoadInt32(&m.activeSessions))

			// Scale UP: non-blocking spawn (limit per tick to prevent control loop blocking)
			if current < currentTarget {
				m.spawnSessions(ctx, currentTarget-current, tickInterval)
			}

			// Scale DOWN: prune sessions if above target (Hard Kill)
			// Apply damping factor (50%) to prevent overshooting
			if current > currentTarget {
				excess := current - currentTarget
				pruneCount := (excess + 1) / 2
				if pruneCount < 1 {
					pruneCount = 1
				}
				m.pruneSessions(pruneCount)
			}
		}
	}
}

// spawnSessions creates sessions up to the limit allowed per tick interval.
// This prevents blocking the control loop when needed count is large.
func (m *Manager) spawnSessions(ctx context.Context, needed int, tickInterval time.Duration) {
	// Calculate max sessions creatable in this tick (with burst allowance)
	maxPerTick := int(float64(m.perf.SessionsPerSec) * tickInterval.Seconds() * config.SpawnBurstMultiplier)
	if maxPerTick < 1 {
		maxPerTick = 1
	}

	spawnCount := needed
	if spawnCount > maxPerTick {
		spawnCount = maxPerTick
	}

	for i := 0; i < spawnCount; i++ {
		if err := m.limiter.Wait(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			break
		}
		go m.launchSession(ctx)
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
		_ = id
		pruned++
	}
}

func (m *Manager) calculatePulseTarget(isHigh bool, elapsed time.Duration) int {
	highTarget := m.perf.TargetSessions
	lowTarget := int(float64(m.perf.TargetSessions) * m.perf.Pulse.LowRatio)
	if lowTarget < 1 {
		lowTarget = 1
	}

	switch m.perf.Pulse.WaveType {
	case config.WaveTypeSine:
		var phaseDuration time.Duration
		if isHigh {
			phaseDuration = m.perf.Pulse.HighTime
		} else {
			phaseDuration = m.perf.Pulse.LowTime
		}

		progress := float64(elapsed) / float64(phaseDuration)
		if progress > 1 {
			progress = 1
		}

		if isHigh {
			return highTarget
		}
		// Low phase: use sine to smoothly transition
		sineValue := (1 + math.Sin(progress*math.Pi-math.Pi/2)) / 2
		return lowTarget + int(float64(highTarget-lowTarget)*sineValue)

	case config.WaveTypeSawtooth:
		if isHigh {
			progress := float64(elapsed) / float64(m.perf.Pulse.HighTime)
			if progress > 1 {
				progress = 1
			}
			return lowTarget + int(float64(highTarget-lowTarget)*progress)
		}
		return lowTarget

	case config.WaveTypeSquare:
		fallthrough
	default:
		if isHigh {
			return highTarget
		}
		return lowTarget
	}
}

func (m *Manager) runSteadyState(ctx context.Context) error {
	// No ramp-up: spawn all sessions immediately without rate limiting
	m.spawnSessionsImmediate(ctx, m.perf.TargetSessions)

	tickInterval := config.SessionTickInterval
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.shutdownAll()
			return ctx.Err()
		case <-ticker.C:
			// Maintain target sessions (replace dead ones)
			current := int(atomic.LoadInt32(&m.activeSessions))
			if current < m.perf.TargetSessions {
				m.spawnSessionsImmediate(ctx, m.perf.TargetSessions-current)
			}
		}
	}
}

// spawnSessionsImmediate spawns sessions without rate limiting.
// Used when no ramp-up is configured for immediate target achievement.
func (m *Manager) spawnSessionsImmediate(ctx context.Context, count int) {
	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			return
		default:
			go m.launchSession(ctx)
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
	maxConsecutiveFailures := m.perf.MaxConsecutiveFailures
	if maxConsecutiveFailures <= 0 {
		maxConsecutiveFailures = config.DefaultMaxConsecutiveFailures
	}

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

				backoff := time.Duration(consecutiveFailures) * config.BaseBackoffDelay
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

			// Quick retry after success
			select {
			case <-ctx.Done():
				return
			case <-time.After(config.QuickRetryDelay):
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
	b := make([]byte, config.SessionIDLength)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

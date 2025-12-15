package metrics

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type Collector struct {
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	activeSessions  int32
	tcpConnections  int64

	socketTimeouts   int64
	socketReconnects int64

	mu                sync.RWMutex
	requestsPerSecond []int
	currentSecond     int64
	currentCount      int

	connectionsPerSecond []int // To track CPS
	currentConnCount     int   // Current second connection attempts

	connectionLifetimes []time.Duration
	activeConnections   map[string]*ConnectionInfo

	analyzeLatency bool
	latencies      []int64
	latencyMu      sync.Mutex

	stopChan chan struct{}
}

type ConnectionInfo struct {
	StartTime        time.Time
	LastActivityTime time.Time
	ReconnectCount   int
	RemoteAddr       string
}

func NewCollector() *Collector {
	c := &Collector{
		requestsPerSecond:    make([]int, 0, 3600),
		connectionsPerSecond: make([]int, 0, 3600),
		connectionLifetimes:  make([]time.Duration, 0, 10000),
		activeConnections:    make(map[string]*ConnectionInfo),
		latencies:            make([]int64, 0, 100000),
		stopChan:             make(chan struct{}),
	}
	go c.recordLoop()
	return c
}

func (c *Collector) SetAnalyzeLatency(enabled bool) {
	c.analyzeLatency = enabled
}

func (c *Collector) RecordSuccess() {
	atomic.AddInt64(&c.totalRequests, 1)
	atomic.AddInt64(&c.successRequests, 1)

	c.mu.Lock()
	c.currentCount++
	c.mu.Unlock()
}

func (c *Collector) RecordSuccessWithLatency(duration time.Duration) {
	atomic.AddInt64(&c.totalRequests, 1)
	atomic.AddInt64(&c.successRequests, 1)

	c.mu.Lock()
	c.currentCount++
	c.mu.Unlock()

	if c.analyzeLatency {
		c.recordLatency(duration)
	}
}

func (c *Collector) recordLatency(duration time.Duration) {
	c.latencyMu.Lock()
	defer c.latencyMu.Unlock()

	c.latencies = append(c.latencies, duration.Microseconds())

	// Sliding window: keep last 10,000 samples
	if len(c.latencies) > 10000 {
		c.latencies = c.latencies[len(c.latencies)-10000:]
	}
}

func (c *Collector) RecordFailure() {
	atomic.AddInt64(&c.totalRequests, 1)
	atomic.AddInt64(&c.failedRequests, 1)
}

func (c *Collector) IncrementActive() {
	atomic.AddInt32(&c.activeSessions, 1)
}

func (c *Collector) DecrementActive() {
	atomic.AddInt32(&c.activeSessions, -1)
}

func (c *Collector) SetTCPConnections(count int64) {
	atomic.StoreInt64(&c.tcpConnections, count)
}

func (c *Collector) RecordSocketTimeout() {
	atomic.AddInt64(&c.socketTimeouts, 1)
}

func (c *Collector) RecordSocketReconnect() {
	atomic.AddInt64(&c.socketReconnects, 1)
}

// RecordConnectionAttempt records a new connection attempt for CPS tracking.
func (c *Collector) RecordConnectionAttempt() {
	c.mu.Lock()
	c.currentConnCount++
	c.mu.Unlock()
}

func (c *Collector) RecordConnectionStart(connID, remoteAddr string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.activeConnections[connID] = &ConnectionInfo{
		StartTime:        time.Now(),
		LastActivityTime: time.Now(),
		RemoteAddr:       remoteAddr,
	}
}

func (c *Collector) RecordConnectionActivity(connID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if info, exists := c.activeConnections[connID]; exists {
		info.LastActivityTime = time.Now()
	}
}

func (c *Collector) RecordConnectionEnd(connID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if info, exists := c.activeConnections[connID]; exists {
		lifetime := time.Since(info.StartTime)
		c.connectionLifetimes = append(c.connectionLifetimes, lifetime)
		// Windowing: Keep last 10000 connections
		if len(c.connectionLifetimes) > 10000 {
			// Remove oldest (simple truncation to avoid O(N) shift if we just want samples)
			// Or shift if we want strict history. Slicing is O(1) effectively but allocation might happen on append.
			// Let's keep strict window.
			c.connectionLifetimes = c.connectionLifetimes[1:]
		}
		delete(c.activeConnections, connID)
	}
}

func (c *Collector) recordLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.mu.Lock()
			// Record RPS
			c.requestsPerSecond = append(c.requestsPerSecond, c.currentCount)
			// Windowing: Keep fast 3600 seconds (1 hour)
			if len(c.requestsPerSecond) > 3600 {
				c.requestsPerSecond = c.requestsPerSecond[len(c.requestsPerSecond)-3600:]
			}
			c.currentCount = 0

			// Record CPS
			c.connectionsPerSecond = append(c.connectionsPerSecond, c.currentConnCount)
			// Windowing: Keep last 3600 seconds
			if len(c.connectionsPerSecond) > 3600 {
				c.connectionsPerSecond = c.connectionsPerSecond[len(c.connectionsPerSecond)-3600:]
			}
			c.currentConnCount = 0
			c.mu.Unlock()
		}
	}
}

func (c *Collector) Stop() {
	close(c.stopChan)
}

type Stats struct {
	Total            int64
	Success          int64
	Failed           int64
	Active           int32
	TCPConnections   int64
	SocketTimeouts   int64
	SocketReconnects int64
	ActiveConnCount  int
	AvgConnLifetime  time.Duration
	MinConnLifetime  time.Duration
	MaxConnLifetime  time.Duration
	AvgPerSec        float64
	StdDev           float64
	MinPerSec        int
	MaxPerSec        int
	P50              int
	P95              int
	P99              int

	// Connection statistics (CPS)
	AvgConnPerSec float64
	MaxConnPerSec int
	MinConnPerSec int

	SuccessRate float64
	// Latency percentiles (microseconds)
	LatencyEnabled bool
	LatencyP50     int64
	LatencyP95     int64
	LatencyP99     int64
	LatencyMin     int64
	LatencyMax     int64
	LatencyAvg     float64
	LatencyCount   int
}

func (c *Collector) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := atomic.LoadInt64(&c.totalRequests)
	success := atomic.LoadInt64(&c.successRequests)
	failed := atomic.LoadInt64(&c.failedRequests)
	active := atomic.LoadInt32(&c.activeSessions)
	tcpConns := atomic.LoadInt64(&c.tcpConnections)
	timeouts := atomic.LoadInt64(&c.socketTimeouts)
	reconnects := atomic.LoadInt64(&c.socketReconnects)

	stats := Stats{
		Total:            total,
		Success:          success,
		Failed:           failed,
		Active:           active,
		TCPConnections:   tcpConns,
		SocketTimeouts:   timeouts,
		SocketReconnects: reconnects,
		ActiveConnCount:  len(c.activeConnections),
		LatencyEnabled:   c.analyzeLatency,
	}

	if total > 0 {
		stats.SuccessRate = float64(success) / float64(total) * 100
	}

	if len(c.requestsPerSecond) > 0 {
		stats.AvgPerSec = c.calculateAverage()
		stats.StdDev = c.calculateStdDev(stats.AvgPerSec)
		stats.MinPerSec, stats.MaxPerSec = c.calculateMinMax()
		stats.P50, stats.P95, stats.P99 = c.calculatePercentiles()
	}

	if len(c.connectionsPerSecond) > 0 {
		stats.AvgConnPerSec, stats.MinConnPerSec, stats.MaxConnPerSec = c.calculateConnStats()
	}

	if len(c.connectionLifetimes) > 0 {
		stats.AvgConnLifetime, stats.MinConnLifetime, stats.MaxConnLifetime = c.calculateConnectionLifetimes()
	}

	if c.analyzeLatency {
		stats.LatencyP50, stats.LatencyP95, stats.LatencyP99, stats.LatencyMin, stats.LatencyMax, stats.LatencyAvg, stats.LatencyCount = c.calculateLatencyPercentiles()
	}

	return stats
}

func (c *Collector) calculateLatencyPercentiles() (p50, p95, p99, min, max int64, avg float64, count int) {
	c.latencyMu.Lock()
	defer c.latencyMu.Unlock()

	count = len(c.latencies)
	if count == 0 {
		return 0, 0, 0, 0, 0, 0, 0
	}

	sorted := make([]int64, count)
	copy(sorted, c.latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	min = sorted[0]
	max = sorted[count-1]

	var sum int64
	for _, v := range sorted {
		sum += v
	}
	avg = float64(sum) / float64(count)

	p50 = percentileInt64(sorted, 50)
	p95 = percentileInt64(sorted, 95)
	p99 = percentileInt64(sorted, 99)

	return
}

func percentileInt64(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}

	index := int(math.Ceil(float64(len(sorted)) * float64(p) / 100.0))
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	if index < 0 {
		index = 0
	}

	return sorted[index]
}

func (c *Collector) calculateAverage() float64 {
	if len(c.requestsPerSecond) == 0 {
		return 0
	}

	var sum int
	for _, v := range c.requestsPerSecond {
		sum += v
	}
	return float64(sum) / float64(len(c.requestsPerSecond))
}

func (c *Collector) calculateStdDev(avg float64) float64 {
	if len(c.requestsPerSecond) < 2 {
		return 0
	}

	var sum float64
	for _, v := range c.requestsPerSecond {
		diff := float64(v) - avg
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(c.requestsPerSecond)))
}

func (c *Collector) calculateMinMax() (int, int) {
	if len(c.requestsPerSecond) == 0 {
		return 0, 0
	}

	min := c.requestsPerSecond[0]
	max := c.requestsPerSecond[0]

	for _, v := range c.requestsPerSecond[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	return min, max
}

func (c *Collector) calculatePercentiles() (int, int, int) {
	if len(c.requestsPerSecond) == 0 {
		return 0, 0, 0
	}

	sorted := make([]int, len(c.requestsPerSecond))
	copy(sorted, c.requestsPerSecond)
	sort.Ints(sorted)

	p50 := percentile(sorted, 50)
	p95 := percentile(sorted, 95)
	p99 := percentile(sorted, 99)

	return p50, p95, p99
}

func (c *Collector) calculateConnStats() (float64, int, int) {
	if len(c.connectionsPerSecond) == 0 {
		return 0, 0, 0
	}

	var sum int
	min := c.connectionsPerSecond[0]
	max := c.connectionsPerSecond[0]

	for _, v := range c.connectionsPerSecond {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	avg := float64(sum) / float64(len(c.connectionsPerSecond))
	return avg, min, max
}

func (c *Collector) calculateConnectionLifetimes() (time.Duration, time.Duration, time.Duration) {
	if len(c.connectionLifetimes) == 0 {
		return 0, 0, 0
	}

	var sum time.Duration
	min := c.connectionLifetimes[0]
	max := c.connectionLifetimes[0]

	for _, lifetime := range c.connectionLifetimes {
		sum += lifetime
		if lifetime < min {
			min = lifetime
		}
		if lifetime > max {
			max = lifetime
		}
	}

	avg := sum / time.Duration(len(c.connectionLifetimes))
	return avg, min, max
}

func percentile(sorted []int, p int) int {
	if len(sorted) == 0 {
		return 0
	}

	index := int(math.Ceil(float64(len(sorted)) * float64(p) / 100.0))
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	if index < 0 {
		index = 0
	}

	return sorted[index]
}

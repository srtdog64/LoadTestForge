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

	connectionLifetimes []time.Duration
	activeConnections   map[string]*ConnectionInfo

	stopChan chan struct{}
}

type ConnectionInfo struct {
	StartTime      time.Time
	LastActivityTime time.Time
	ReconnectCount int
	RemoteAddr     string
}

func NewCollector() *Collector {
	c := &Collector{
		requestsPerSecond: make([]int, 0, 3600),
		connectionLifetimes: make([]time.Duration, 0, 10000),
		activeConnections: make(map[string]*ConnectionInfo),
		stopChan:          make(chan struct{}),
	}
	go c.recordLoop()
	return c
}

func (c *Collector) RecordSuccess() {
	atomic.AddInt64(&c.totalRequests, 1)
	atomic.AddInt64(&c.successRequests, 1)

	c.mu.Lock()
	c.currentCount++
	c.mu.Unlock()
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
			c.requestsPerSecond = append(c.requestsPerSecond, c.currentCount)
			c.currentCount = 0
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
	SuccessRate      float64
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

	if len(c.connectionLifetimes) > 0 {
		stats.AvgConnLifetime, stats.MinConnLifetime, stats.MaxConnLifetime = c.calculateConnectionLifetimes()
	}

	return stats
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

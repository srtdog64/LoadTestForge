package config

import (
	"time"
)

type Config struct {
	Target      TargetConfig
	Strategy    StrategyConfig
	Performance PerformanceConfig
	Reporting   ReportingConfig
	BindIP      string   // Single IP (legacy)
	BindIPs     []string // Multiple IPs for round-robin binding
}

type TargetConfig struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    string
}

type StrategyConfig struct {
	Type              string
	Timeout           time.Duration
	KeepAliveInterval time.Duration
	ContentLength     int
	ReadSize          int
	WindowSize        int
	PostDataSize      int
	RequestsPerConn   int
	// H2 Flood settings
	MaxStreams  int
	BurstSize   int
	// Heavy Payload settings
	PayloadType  string
	PayloadDepth int
	PayloadSize  int
	// RUDY settings
	ChunkDelayMin    time.Duration
	ChunkDelayMax    time.Duration
	ChunkSizeMin     int
	ChunkSizeMax     int
	PersistConn      bool
	MaxReqPerSession int           // 0 = unlimited (hold until server closes)
	KeepAliveTimeout time.Duration
	SessionLifetime  time.Duration // 0 = unlimited (hold until server closes)
	SendBufferSize   int
	UseJSON          bool
	UseMultipart     bool
	EvasionLevel     int
	// Advanced options
	EnableStealth  bool // Browser fingerprint headers (Sec-Fetch-*)
	RandomizePath  bool // Realistic query strings for cache bypass
	AnalyzeLatency bool // Response time percentile analysis (p50, p95, p99)
	// TCP Flood settings
	SendDataOnConnect bool // Send a byte after TCP connection (tcp-flood)
	TCPKeepAlive      bool // Enable TCP keep-alive (tcp-flood)
}

type PulseConfig struct {
	Enabled  bool
	HighTime time.Duration
	LowTime  time.Duration
	LowRatio float64
	WaveType string // "square", "sine", "sawtooth"
}

type PerformanceConfig struct {
	TargetSessions         int
	SessionsPerSec         int
	Duration               time.Duration
	RampUpDuration         time.Duration
	MaxConsecutiveFailures int // 연속 실패 허용 횟수 (기본값: 5)
	Pulse                  PulseConfig
}

type ReportingConfig struct {
	Interval     time.Duration
	ExportPath   string
	ExportFormat string
}

func DefaultConfig() *Config {
	return &Config{
		Target: TargetConfig{
			Method: "GET",
			Headers: map[string]string{
				"User-Agent": "LoadTestForge/1.0",
			},
		},
		Strategy: StrategyConfig{
			Type:              "normal",
			Timeout:           10 * time.Second,
			KeepAliveInterval: 10 * time.Second,
			ContentLength:     100000,
			ReadSize:          1,
			WindowSize:        64,
			PostDataSize:      1024,
			RequestsPerConn:   100,
			MaxStreams:        100,
			BurstSize:         10,
			PayloadType:       "deep-json",
			PayloadDepth:      50,
			PayloadSize:       10000,
			ChunkDelayMin:     1 * time.Second,
			ChunkDelayMax:     5 * time.Second,
			ChunkSizeMin:      1,
			ChunkSizeMax:      100,
			PersistConn:       true,
			MaxReqPerSession:  0, // 0 = unlimited (hold until server closes)
			KeepAliveTimeout:  600 * time.Second,
			SessionLifetime:   0, // 0 = unlimited (hold until server closes)
			SendBufferSize:    1024,
			UseJSON:           false,
			UseMultipart:      false,
			EvasionLevel:      2,
			SendDataOnConnect: false,
			TCPKeepAlive:      true,
		},
		Performance: PerformanceConfig{
			TargetSessions:         100,
			SessionsPerSec:         10,
			Duration:               60 * time.Second,
			RampUpDuration:         0,
			MaxConsecutiveFailures: 5,
			Pulse: PulseConfig{
				Enabled:  false,
				HighTime: 30 * time.Second,
				LowTime:  30 * time.Second,
				LowRatio: 0.1,
				WaveType: "square",
			},
		},
		Reporting: ReportingConfig{
			Interval:     2 * time.Second,
			ExportFormat: "json",
		},
	}
}

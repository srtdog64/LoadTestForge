package config

import (
	"time"
)

type Config struct {
	Target      TargetConfig
	Strategy    StrategyConfig
	Performance PerformanceConfig
	Reporting   ReportingConfig
	BindIP      string
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
}

type PulseConfig struct {
	Enabled  bool
	HighTime time.Duration
	LowTime  time.Duration
	LowRatio float64
	WaveType string // "square", "sine", "sawtooth"
}

type PerformanceConfig struct {
	TargetSessions int
	SessionsPerSec int
	Duration       time.Duration
	RampUpDuration time.Duration
	Pulse          PulseConfig
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
		},
		Performance: PerformanceConfig{
			TargetSessions: 100,
			SessionsPerSec: 10,
			Duration:       60 * time.Second,
			RampUpDuration: 0,
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

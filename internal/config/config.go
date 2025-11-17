package config

import (
	"time"
)

type Config struct {
	Target         TargetConfig
	Strategy       StrategyConfig
	Performance    PerformanceConfig
	Reporting      ReportingConfig
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
}

type PerformanceConfig struct {
	TargetSessions int
	SessionsPerSec int
	Duration       time.Duration
}

type ReportingConfig struct {
	Interval       time.Duration
	ExportPath     string
	ExportFormat   string
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
		},
		Performance: PerformanceConfig{
			TargetSessions: 100,
			SessionsPerSec: 10,
			Duration:       60 * time.Second,
		},
		Reporting: ReportingConfig{
			Interval:     2 * time.Second,
			ExportFormat: "json",
		},
	}
}

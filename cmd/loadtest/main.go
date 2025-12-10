package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jdw/loadtestforge/internal/config"
	"github.com/jdw/loadtestforge/internal/metrics"
	"github.com/jdw/loadtestforge/internal/session"
	"github.com/jdw/loadtestforge/internal/strategy"
)

func main() {
	cfg := parseFlags()

	if err := validateConfig(cfg); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nShutting down gracefully...")
		cancel()
	}()

	if cfg.Performance.Duration > 0 {
		go func() {
			<-time.After(cfg.Performance.Duration)
			fmt.Println("\n\nDuration limit reached, shutting down...")
			cancel()
		}()
	}

	strat := createStrategy(cfg)
	target := strategy.Target{
		URL:     cfg.Target.URL,
		Method:  cfg.Target.Method,
		Headers: cfg.Target.Headers,
		Body:    []byte(cfg.Target.Body),
	}

	metricsCollector := metrics.NewCollector()
	metricsCollector.SetAnalyzeLatency(cfg.Strategy.AnalyzeLatency)
	defer metricsCollector.Stop()

	manager := session.NewManager(
		strat,
		target,
		cfg.Performance,
		metricsCollector,
	)

	reporter := metrics.NewReporter(metricsCollector)

	go func() {
		reporter.Start(ctx)
	}()

	fmt.Printf("Starting LoadTestForge...\n")
	fmt.Printf("Target: %s\n", cfg.Target.URL)
	fmt.Printf("Strategy: %s\n", cfg.Strategy.Type)
	fmt.Printf("Target Sessions: %d\n", cfg.Performance.TargetSessions)
	fmt.Printf("Sessions/sec: %d\n", cfg.Performance.SessionsPerSec)
	if cfg.Performance.RampUpDuration > 0 {
		fmt.Printf("Ramp-up: %v\n", cfg.Performance.RampUpDuration)
	}
	if cfg.Performance.Pulse.Enabled {
		fmt.Printf("Pulse Mode: %s (high: %v, low: %v, ratio: %.0f%%)\n",
			cfg.Performance.Pulse.WaveType,
			cfg.Performance.Pulse.HighTime,
			cfg.Performance.Pulse.LowTime,
			cfg.Performance.Pulse.LowRatio*100)
	}
	if cfg.Strategy.EnableStealth || cfg.Strategy.RandomizePath || cfg.Strategy.AnalyzeLatency {
		fmt.Printf("Advanced: stealth=%v, randomize=%v, latency-analysis=%v\n",
			cfg.Strategy.EnableStealth,
			cfg.Strategy.RandomizePath,
			cfg.Strategy.AnalyzeLatency)
	}
	if len(cfg.BindIPs) > 0 {
		if len(cfg.BindIPs) == 1 {
			fmt.Printf("Bind IP: %s\n", cfg.BindIPs[0])
		} else {
			fmt.Printf("Bind IPs: %d addresses (round-robin)\n", len(cfg.BindIPs))
			for i, ip := range cfg.BindIPs {
				fmt.Printf("  [%d] %s\n", i+1, ip)
			}
		}
	}
	fmt.Println()

	time.Sleep(2 * time.Second)

	if err := manager.Run(ctx); err != nil && err != context.Canceled {
		log.Printf("Manager error: %v", err)
	}

	time.Sleep(2 * time.Second)
	fmt.Println("\nShutdown complete")
}

func parseFlags() *config.Config {
	cfg := config.DefaultConfig()

	flag.StringVar(&cfg.Target.URL, "target", "", "Target URL (required)")
	flag.StringVar(&cfg.Target.Method, "method", "GET", "HTTP method")
	flag.StringVar(&cfg.Strategy.Type, "strategy", "keepalive", "Attack strategy (normal|keepalive|slowloris|slowloris-keepalive|slow-post|slow-read|http-flood|h2-flood|heavy-payload|rudy)")
	flag.StringVar(&cfg.BindIP, "bind-ip", "", "Source IP address(es) to bind, comma-separated for multiple (e.g., 192.168.1.100,192.168.1.101)")
	flag.IntVar(&cfg.Performance.TargetSessions, "sessions", 100, "Target concurrent sessions")
	flag.IntVar(&cfg.Performance.SessionsPerSec, "rate", 10, "Sessions per second")
	flag.DurationVar(&cfg.Performance.Duration, "duration", 0, "Test duration (0 = infinite)")
	flag.DurationVar(&cfg.Performance.RampUpDuration, "rampup", 0, "Ramp-up duration (e.g., 30s, 2m)")
	flag.DurationVar(&cfg.Strategy.Timeout, "timeout", 10*time.Second, "Request timeout")
	flag.DurationVar(&cfg.Strategy.KeepAliveInterval, "keepalive", 10*time.Second, "Keep-alive ping interval")
	flag.IntVar(&cfg.Strategy.ContentLength, "content-length", 100000, "Content-Length for slow-post")
	flag.IntVar(&cfg.Strategy.ReadSize, "read-size", 1, "Bytes to read per iteration for slow-read")
	flag.IntVar(&cfg.Strategy.WindowSize, "window-size", 64, "TCP window size for slow-read")
	flag.IntVar(&cfg.Strategy.PostDataSize, "post-size", 1024, "POST data size for http-flood")
	flag.IntVar(&cfg.Strategy.RequestsPerConn, "requests-per-conn", 100, "Requests per connection for http-flood")

	// H2 Flood settings
	flag.IntVar(&cfg.Strategy.MaxStreams, "max-streams", 100, "Max concurrent streams per connection for h2-flood")
	flag.IntVar(&cfg.Strategy.BurstSize, "burst-size", 10, "Stream burst size for h2-flood")

	// Heavy Payload settings
	flag.StringVar(&cfg.Strategy.PayloadType, "payload-type", "deep-json", "Payload type for heavy-payload (deep-json|redos|nested-xml|query-flood|multipart)")
	flag.IntVar(&cfg.Strategy.PayloadDepth, "payload-depth", 50, "Nesting depth for heavy-payload")
	flag.IntVar(&cfg.Strategy.PayloadSize, "payload-size", 10000, "Payload size for heavy-payload")

	// RUDY settings
	flag.DurationVar(&cfg.Strategy.ChunkDelayMin, "chunk-delay-min", 1*time.Second, "Minimum delay between chunks for rudy")
	flag.DurationVar(&cfg.Strategy.ChunkDelayMax, "chunk-delay-max", 5*time.Second, "Maximum delay between chunks for rudy")
	flag.IntVar(&cfg.Strategy.ChunkSizeMin, "chunk-size-min", 1, "Minimum chunk size in bytes for rudy")
	flag.IntVar(&cfg.Strategy.ChunkSizeMax, "chunk-size-max", 100, "Maximum chunk size in bytes for rudy")
	flag.BoolVar(&cfg.Strategy.PersistConn, "persist", true, "Enable persistent connections for rudy")
	flag.IntVar(&cfg.Strategy.MaxReqPerSession, "max-req-per-session", 10, "Maximum requests per session for rudy")
	flag.DurationVar(&cfg.Strategy.KeepAliveTimeout, "keepalive-timeout", 600*time.Second, "Keep-alive timeout for rudy")
	flag.BoolVar(&cfg.Strategy.UseJSON, "use-json", false, "Use JSON encoding for rudy")
	flag.BoolVar(&cfg.Strategy.UseMultipart, "use-multipart", false, "Use multipart/form-data encoding for rudy")
	flag.IntVar(&cfg.Strategy.EvasionLevel, "evasion-level", 2, "Evasion level for rudy (1=basic, 2=normal, 3=aggressive)")
	flag.DurationVar(&cfg.Strategy.SessionLifetime, "session-lifetime", 3600*time.Second, "Session lifetime for rudy")
	flag.IntVar(&cfg.Strategy.SendBufferSize, "send-buffer", 1024, "TCP send buffer size for rudy (small = slower)")

	// Session failure settings
	flag.IntVar(&cfg.Performance.MaxConsecutiveFailures, "max-failures", 5, "Max consecutive failures before session terminates")

	// Pulse settings
	flag.BoolVar(&cfg.Performance.Pulse.Enabled, "pulse", false, "Enable pulsing load pattern")
	flag.DurationVar(&cfg.Performance.Pulse.HighTime, "pulse-high", 30*time.Second, "Duration of high load phase")
	flag.DurationVar(&cfg.Performance.Pulse.LowTime, "pulse-low", 30*time.Second, "Duration of low load phase")
	flag.Float64Var(&cfg.Performance.Pulse.LowRatio, "pulse-ratio", 0.1, "Session ratio during low phase (0.1 = 10%)")
	flag.StringVar(&cfg.Performance.Pulse.WaveType, "pulse-wave", "square", "Wave type (square|sine|sawtooth)")

	// Advanced options
	flag.BoolVar(&cfg.Strategy.EnableStealth, "stealth", false, "Enable browser fingerprint headers (Sec-Fetch-*) for WAF bypass")
	flag.BoolVar(&cfg.Strategy.RandomizePath, "randomize", false, "Enable realistic query strings for cache bypass")
	flag.BoolVar(&cfg.Strategy.AnalyzeLatency, "analyze-latency", false, "Enable response time percentile analysis (p50, p95, p99)")

	flag.Parse()

	return cfg
}

func validateConfig(cfg *config.Config) error {
	if cfg.Target.URL == "" {
		return fmt.Errorf("target URL is required")
	}

	// Parse multiple IPs from bind-ip flag
	if cfg.BindIP != "" {
		cfg.BindIPs = parseBindIPs(cfg.BindIP)
		if len(cfg.BindIPs) == 0 {
			return fmt.Errorf("no valid IPs found in bind-ip: %s", cfg.BindIP)
		}
		for _, ip := range cfg.BindIPs {
			if net.ParseIP(ip) == nil {
				return fmt.Errorf("invalid bind IP: %s", ip)
			}
		}
	}

	if cfg.Performance.TargetSessions <= 0 {
		return fmt.Errorf("target sessions must be positive")
	}

	if cfg.Performance.SessionsPerSec <= 0 {
		return fmt.Errorf("sessions per second must be positive")
	}

	if cfg.Performance.SessionsPerSec > cfg.Performance.TargetSessions {
		log.Printf("Warning: sessions/sec (%d) > target sessions (%d), adjusting...",
			cfg.Performance.SessionsPerSec, cfg.Performance.TargetSessions)
		cfg.Performance.SessionsPerSec = cfg.Performance.TargetSessions
	}

	if cfg.Performance.RampUpDuration > 0 && cfg.Performance.Duration > 0 {
		if cfg.Performance.RampUpDuration >= cfg.Performance.Duration {
			return fmt.Errorf("ramp-up duration must be shorter than total duration")
		}
	}

	return nil
}

func createStrategy(cfg *config.Config) strategy.AttackStrategy {
	switch cfg.Strategy.Type {
	case "slowloris":
		return strategy.NewSlowlorisClassic(cfg.Strategy.KeepAliveInterval, cfg.BindIP)
	case "slowloris-keepalive", "keepsloworis":
		return strategy.NewSlowloris(cfg.Strategy.KeepAliveInterval, cfg.BindIP)
	case "keepalive":
		return strategy.NewKeepAliveHTTP(cfg.Strategy.KeepAliveInterval, cfg.BindIP)
	case "normal":
		return strategy.NewNormalHTTP(cfg.Strategy.Timeout, cfg.BindIP)
	case "slow-post":
		return strategy.NewSlowPost(cfg.Strategy.KeepAliveInterval, cfg.Strategy.ContentLength, cfg.BindIP)
	case "slow-read":
		return strategy.NewSlowRead(cfg.Strategy.KeepAliveInterval, cfg.Strategy.ReadSize, cfg.Strategy.WindowSize, cfg.BindIP)
	case "http-flood":
		return strategy.NewHTTPFlood(cfg.Strategy.Timeout, cfg.Target.Method, cfg.Strategy.PostDataSize, cfg.Strategy.RequestsPerConn, cfg.BindIP, cfg.Strategy.EnableStealth, cfg.Strategy.RandomizePath)
	case "h2-flood":
		return strategy.NewH2Flood(cfg.Strategy.MaxStreams, cfg.Strategy.BurstSize, cfg.BindIP)
	case "heavy-payload":
		return strategy.NewHeavyPayload(cfg.Strategy.Timeout, cfg.Strategy.PayloadType, cfg.Strategy.PayloadDepth, cfg.Strategy.PayloadSize, cfg.BindIP)
	case "rudy":
		rudyCfg := strategy.RUDYConfig{
			ContentLength:         cfg.Strategy.ContentLength,
			ChunkDelayMin:         cfg.Strategy.ChunkDelayMin,
			ChunkDelayMax:         cfg.Strategy.ChunkDelayMax,
			ChunkSizeMin:          cfg.Strategy.ChunkSizeMin,
			ChunkSizeMax:          cfg.Strategy.ChunkSizeMax,
			PersistConnections:    cfg.Strategy.PersistConn,
			MaxRequestsPerSession: cfg.Strategy.MaxReqPerSession,
			KeepAliveTimeout:      cfg.Strategy.KeepAliveTimeout,
			SessionLifetime:       cfg.Strategy.SessionLifetime,
			UseJSON:               cfg.Strategy.UseJSON,
			UseMultipart:          cfg.Strategy.UseMultipart,
			RandomizePath:         cfg.Strategy.RandomizePath,
			EvasionLevel:          cfg.Strategy.EvasionLevel,
			ConnectTimeout:        cfg.Strategy.Timeout,
			SendBufferSize:        cfg.Strategy.SendBufferSize,
		}
		return strategy.NewRUDY(rudyCfg, cfg.BindIP)
	default:
		log.Printf("Unknown strategy '%s', using 'keepalive'", cfg.Strategy.Type)
		return strategy.NewKeepAliveHTTP(cfg.Strategy.KeepAliveInterval, cfg.BindIP)
	}
}

// parseBindIPs parses comma/space/semicolon separated IP list.
func parseBindIPs(s string) []string {
	var result []string
	var current string

	for _, c := range s {
		if c == ',' || c == ' ' || c == ';' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/srtdog64/loadtestforge/internal/config"
	"github.com/srtdog64/loadtestforge/internal/metrics"
	"github.com/srtdog64/loadtestforge/internal/session"
	"github.com/srtdog64/loadtestforge/internal/strategy"
)

func main() {
	// Go 1.20+ automatically seeds the global random number generator

	cfg := parseFlags()

	if err := validateConfig(cfg); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Safety check for public IP targets
	if !confirmPublicTarget(cfg.Target.URL) {
		fmt.Println("Test cancelled by user.")
		os.Exit(0)
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

	reporter := metrics.NewReporter(metricsCollector, cfg.Thresholds)

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

	// Target settings
	flag.StringVar(&cfg.Target.URL, "target", "", "Target URL (required)")
	flag.StringVar(&cfg.Target.Method, "method", "GET", "HTTP method")
	flag.StringVar(&cfg.Strategy.Type, "strategy", "keepalive", "Attack strategy (normal|keepalive|slowloris|slowloris-keepalive|slow-post|slow-read|http-flood|h2-flood|heavy-payload|rudy|tcp-flood)")
	flag.StringVar(&cfg.BindIP, "bind-ip", "", "Source IP address(es) to bind, comma-separated for multiple (e.g., 192.168.1.100,192.168.1.101)")
	flag.BoolVar(&cfg.Strategy.BindRandom, "bind-random", false, "Randomize source IP selection from the bind range (default: round-robin)")

	// Performance settings
	flag.IntVar(&cfg.Performance.TargetSessions, "sessions", config.DefaultTargetSessions, "Target concurrent sessions")
	flag.IntVar(&cfg.Performance.SessionsPerSec, "rate", config.DefaultSessionsPerSec, "Sessions per second")
	flag.DurationVar(&cfg.Performance.Duration, "duration", 0, "Test duration (0 = infinite)")
	flag.DurationVar(&cfg.Performance.RampUpDuration, "rampup", 0, "Ramp-up duration (e.g., 30s, 2m)")

	// Connection settings
	flag.DurationVar(&cfg.Strategy.Timeout, "timeout", config.DefaultConnectTimeout, "Request timeout")
	flag.DurationVar(&cfg.Strategy.KeepAliveInterval, "keepalive", config.DefaultKeepAliveInterval, "Keep-alive ping interval")

	// Slow attack settings
	flag.IntVar(&cfg.Strategy.ContentLength, "content-length", config.DefaultContentLength, "Content-Length for slow-post")
	flag.IntVar(&cfg.Strategy.ReadSize, "read-size", config.DefaultReadSize, "Bytes to read per iteration for slow-read")
	flag.IntVar(&cfg.Strategy.WindowSize, "window-size", config.DefaultWindowSize, "TCP window size for slow-read")

	// HTTP Flood settings
	flag.IntVar(&cfg.Strategy.PostDataSize, "post-size", config.DefaultPostDataSize, "POST data size for http-flood")
	flag.IntVar(&cfg.Strategy.RequestsPerConn, "requests-per-conn", config.DefaultRequestsPerConn, "Requests per connection for http-flood")

	// H2 Flood settings
	flag.IntVar(&cfg.Strategy.MaxStreams, "max-streams", config.DefaultMaxStreams, "Max concurrent streams per connection for h2-flood")
	flag.IntVar(&cfg.Strategy.BurstSize, "burst-size", config.DefaultBurstSize, "Stream burst size for h2-flood")

	// Heavy Payload settings
	flag.StringVar(&cfg.Strategy.PayloadType, "payload-type", config.PayloadTypeDeepJSON, "Payload type for heavy-payload (deep-json|redos|nested-xml|query-flood|multipart)")
	flag.IntVar(&cfg.Strategy.PayloadDepth, "payload-depth", config.DefaultPayloadDepth, "Nesting depth for heavy-payload")
	flag.IntVar(&cfg.Strategy.PayloadSize, "payload-size", config.DefaultPayloadSize, "Payload size for heavy-payload")

	// RUDY settings
	flag.DurationVar(&cfg.Strategy.ChunkDelayMin, "chunk-delay-min", config.DefaultChunkDelayMin, "Minimum delay between chunks for rudy")
	flag.DurationVar(&cfg.Strategy.ChunkDelayMax, "chunk-delay-max", config.DefaultChunkDelayMax, "Maximum delay between chunks for rudy")
	flag.IntVar(&cfg.Strategy.ChunkSizeMin, "chunk-size-min", config.DefaultChunkSizeMin, "Minimum chunk size in bytes for rudy")
	flag.IntVar(&cfg.Strategy.ChunkSizeMax, "chunk-size-max", config.DefaultChunkSizeMax, "Maximum chunk size in bytes for rudy")
	flag.BoolVar(&cfg.Strategy.PersistConn, "persist", true, "Enable persistent connections for rudy")
	flag.IntVar(&cfg.Strategy.MaxReqPerSession, "max-req-per-session", config.DefaultMaxReqPerSession, "Maximum requests per session for rudy")
	flag.DurationVar(&cfg.Strategy.KeepAliveTimeout, "keepalive-timeout", config.DefaultKeepAliveTimeout, "Keep-alive timeout for rudy")
	flag.BoolVar(&cfg.Strategy.UseJSON, "use-json", false, "Use JSON encoding for rudy")
	flag.BoolVar(&cfg.Strategy.UseMultipart, "use-multipart", false, "Use multipart/form-data encoding for rudy")
	flag.IntVar(&cfg.Strategy.EvasionLevel, "evasion-level", config.EvasionLevelNormal, "Evasion level for rudy (1=basic, 2=normal, 3=aggressive)")
	flag.DurationVar(&cfg.Strategy.SessionLifetime, "session-lifetime", config.DefaultSessionLifetime, "Session lifetime for rudy")
	flag.IntVar(&cfg.Strategy.SendBufferSize, "send-buffer", config.DefaultSendBufferSize, "TCP send buffer size for rudy (small = slower)")

	// Session failure settings
	flag.IntVar(&cfg.Performance.MaxConsecutiveFailures, "max-failures", config.DefaultMaxConsecutiveFailures, "Max consecutive failures before session terminates")

	// Pulse settings
	flag.BoolVar(&cfg.Performance.Pulse.Enabled, "pulse", false, "Enable pulsing load pattern")
	flag.DurationVar(&cfg.Performance.Pulse.HighTime, "pulse-high", config.DefaultPulseHighTime, "Duration of high load phase")
	flag.DurationVar(&cfg.Performance.Pulse.LowTime, "pulse-low", config.DefaultPulseLowTime, "Duration of low load phase")
	flag.Float64Var(&cfg.Performance.Pulse.LowRatio, "pulse-ratio", config.DefaultPulseLowRatio, "Session ratio during low phase (0.1 = 10%)")
	flag.StringVar(&cfg.Performance.Pulse.WaveType, "pulse-wave", config.WaveTypeSquare, "Wave type (square|sine|sawtooth)")

	// Advanced options
	flag.BoolVar(&cfg.Strategy.EnableStealth, "stealth", false, "Enable browser fingerprint headers (Sec-Fetch-*) for WAF bypass")
	flag.BoolVar(&cfg.Strategy.RandomizePath, "randomize", false, "Enable realistic query strings for cache bypass")
	flag.BoolVar(&cfg.Strategy.AnalyzeLatency, "analyze-latency", false, "Enable response time percentile analysis (p50, p95, p99)")

	// TCP Flood settings
	flag.BoolVar(&cfg.Strategy.SendDataOnConnect, "send-data", false, "Send a byte after TCP connection (tcp-flood)")
	flag.BoolVar(&cfg.Strategy.TCPKeepAlive, "tcp-keepalive", true, "Enable TCP keep-alive (tcp-flood)")

	// TLS settings
	flag.BoolVar(&cfg.Strategy.TLSSkipVerify, "tls-skip-verify", true, "Skip TLS certificate verification")

	// Threshold settings for pass/fail evaluation
	flag.Float64Var(&cfg.Thresholds.MinSuccessRate, "min-success-rate", 90.0, "Minimum success rate (%) for pass")
	flag.Float64Var(&cfg.Thresholds.MaxRateDeviation, "max-rate-deviation", 20.0, "Maximum rate deviation (%) for pass")
	flag.DurationVar(&cfg.Thresholds.MaxP99Latency, "max-p99-latency", 5*time.Second, "Maximum p99 latency for pass")
	flag.Float64Var(&cfg.Thresholds.MaxTimeoutRate, "max-timeout-rate", 10.0, "Maximum timeout rate (%) for pass")

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

	// Validate payload depth to prevent memory exhaustion
	if cfg.Strategy.PayloadDepth < 0 {
		return fmt.Errorf("payload depth cannot be negative")
	}
	if cfg.Strategy.PayloadDepth > 500 {
		log.Printf("Warning: payload depth %d is very high (>500), may cause memory issues", cfg.Strategy.PayloadDepth)
	}

	// Validate payload size
	if cfg.Strategy.PayloadSize < 0 {
		return fmt.Errorf("payload size cannot be negative")
	}
	if cfg.Strategy.PayloadSize > 100*1024*1024 { // 100MB
		return fmt.Errorf("payload size %d exceeds maximum allowed (100MB)", cfg.Strategy.PayloadSize)
	}

	// Validate pulse mode configuration
	if cfg.Performance.Pulse.Enabled {
		if cfg.Performance.Pulse.LowRatio < 0 || cfg.Performance.Pulse.LowRatio > 1 {
			return fmt.Errorf("pulse low ratio must be between 0 and 1")
		}
		if cfg.Performance.Pulse.HighTime <= 0 {
			return fmt.Errorf("pulse high time must be positive")
		}
		if cfg.Performance.Pulse.LowTime <= 0 {
			return fmt.Errorf("pulse low time must be positive")
		}
	}

	// Validate threshold settings
	if cfg.Thresholds.MinSuccessRate < 0 || cfg.Thresholds.MinSuccessRate > 100 {
		return fmt.Errorf("min success rate must be between 0 and 100")
	}
	if cfg.Thresholds.MaxRateDeviation < 0 || cfg.Thresholds.MaxRateDeviation > 100 {
		return fmt.Errorf("max rate deviation must be between 0 and 100")
	}
	if cfg.Thresholds.MaxTimeoutRate < 0 || cfg.Thresholds.MaxTimeoutRate > 100 {
		return fmt.Errorf("max timeout rate must be between 0 and 100")
	}

	return nil
}

func createStrategy(cfg *config.Config) strategy.AttackStrategy {
	factory := strategy.NewStrategyFactory(&cfg.Strategy, cfg.BindIP)

	// Special handling for http-flood to pass target method
	if cfg.Strategy.Type == "http-flood" {
		return factory.CreateWithMethod("http-flood", cfg.Target.Method)
	}

	return factory.Create()
}

// parseBindIPs parses comma/space/semicolon separated IP list and ranges (e.g. 192.168.1.10-20).
// Safety limits are enforced to prevent resource exhaustion from overly large ranges.
func parseBindIPs(s string) []string {
	// First split by delimiters
	parts := strings.FieldsFunc(s, func(c rune) bool {
		return c == ',' || c == ' ' || c == ';'
	})

	var ips []string
	for _, part := range parts {
		// Check total limit early
		if len(ips) >= config.MaxTotalBindIPs {
			log.Printf("Warning: Total bind IPs limited to %d, ignoring remaining", config.MaxTotalBindIPs)
			break
		}

		if strings.Contains(part, "-") {
			// Handle range: 192.168.1.10-20 or 192.168.1.10-192.168.1.20
			ranges := strings.Split(part, "-")
			if len(ranges) != 2 {
				continue // invalid range format
			}
			startIPStr := strings.TrimSpace(ranges[0])
			endRangeStr := strings.TrimSpace(ranges[1])

			startIP := net.ParseIP(startIPStr)
			if startIP == nil {
				continue
			}
			startIPv4 := startIP.To4()
			if startIPv4 == nil {
				continue // IPv6 range not supported yet
			}

			// Check if end part is full IP or just last octet
			var endIP net.IP
			if strings.Contains(endRangeStr, ".") {
				endIP = net.ParseIP(endRangeStr)
			} else {
				// Treat as last octet
				var endOctet int
				_, err := fmt.Sscanf(endRangeStr, "%d", &endOctet)
				if err != nil || endOctet < 0 || endOctet > 255 {
					continue
				}
				endIP = make(net.IP, len(startIPv4))
				copy(endIP, startIPv4)
				endIP[3] = byte(endOctet)
			}

			// Check valid endIP
			endIPv4 := endIP.To4()
			if endIPv4 == nil {
				continue
			}

			// Safety check: ensure start <= end
			if bytesCompare(startIPv4, endIPv4) > 0 {
				log.Printf("Warning: Invalid IP range %s (start > end), skipping", part)
				continue
			}

			// Safety check: limit IPs per range to prevent resource exhaustion
			rangeSize := ipRangeSize(startIPv4, endIPv4)
			if rangeSize > config.MaxIPsPerRange {
				log.Printf("Warning: IP range %s exceeds limit (%d > %d), truncating to %d IPs",
					part, rangeSize, config.MaxIPsPerRange, config.MaxIPsPerRange)
			}

			// Generate IPs with limit
			curr := make(net.IP, len(startIPv4))
			copy(curr, startIPv4)
			rangeCount := 0

			for {
				// Compare current vs end
				if bytesCompare(curr, endIPv4) > 0 {
					break
				}

				// Safety limits
				if rangeCount >= config.MaxIPsPerRange {
					break
				}
				if len(ips) >= config.MaxTotalBindIPs {
					break
				}

				ips = append(ips, curr.String())
				rangeCount++

				// Increment IP
				for i := 3; i >= 0; i-- {
					curr[i]++
					if curr[i] > 0 {
						break
					}
				}
			}

		} else {
			// Single IP
			ips = append(ips, part)
		}
	}
	return ips
}

// ipRangeSize calculates the number of IPs in a range (approximate for safety check).
func ipRangeSize(start, end net.IP) int {
	// Simple calculation for IPv4
	startVal := int(start[0])<<24 | int(start[1])<<16 | int(start[2])<<8 | int(start[3])
	endVal := int(end[0])<<24 | int(end[1])<<16 | int(end[2])<<8 | int(end[3])
	size := endVal - startVal + 1
	if size < 0 {
		return 0
	}
	return size
}

func bytesCompare(a, b []byte) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// confirmPublicTarget checks if the target is a public IP and asks for user confirmation.
// Returns true if the test should proceed, false if cancelled.
func confirmPublicTarget(targetURL string) bool {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return true // Let validation handle invalid URLs
	}

	host := parsed.Hostname()

	// Check if it's localhost
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}

	// Resolve hostname to IP
	ip := net.ParseIP(host)
	if ip == nil {
		// It's a hostname, try to resolve
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			// Can't resolve, show warning anyway
			return promptUserConfirmation(host, "unresolved hostname")
		}
		ip = ips[0]
	}

	// Check if it's a private IP
	if isPrivateIP(ip) {
		return true
	}

	// It's a public IP - require confirmation
	return promptUserConfirmation(host, ip.String())
}

// isPrivateIP checks if an IP address is in private/reserved ranges.
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Check for loopback
	if ip.IsLoopback() {
		return true
	}

	// Check for private ranges
	privateRanges := []string{
		"10.0.0.0/8",     // Class A private
		"172.16.0.0/12",  // Class B private
		"192.168.0.0/16", // Class C private
		"169.254.0.0/16", // Link-local
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// promptUserConfirmation asks the user to confirm testing against a public target.
func promptUserConfirmation(host, resolvedIP string) bool {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    ⚠️  PUBLIC TARGET WARNING ⚠️                    ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Target: %-56s ║\n", host)
	fmt.Printf("║  Resolved IP: %-51s ║\n", resolvedIP)
	fmt.Println("╠══════════════════════════════════════════════════════════════════╣")
	fmt.Println("║  This appears to be a PUBLIC IP address.                         ║")
	fmt.Println("║                                                                  ║")
	fmt.Println("║  LEGAL REMINDER:                                                 ║")
	fmt.Println("║  - You MUST have written authorization to test this target       ║")
	fmt.Println("║  - Unauthorized testing is ILLEGAL in most jurisdictions         ║")
	fmt.Println("║  - You are fully responsible for your actions                    ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Print("Do you have authorization to test this target? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

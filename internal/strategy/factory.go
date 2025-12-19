package strategy

import (
	"fmt"
	"log"
	"time"

	"github.com/srtdog64/loadtestforge/internal/config"
)

// StrategyFactory creates attack strategies based on configuration.
type StrategyFactory struct {
	Config *config.StrategyConfig
	BindIP string
}

// TemplateAliases maps short names to template paths
var TemplateAliases = map[string]string{
	"udp":       "templates/raw/udp_flood.txt",
	"syn":       "templates/raw/tcp_syn.txt",
	"dns":       "templates/raw/dns_query.txt",
	"dns-amp":   "templates/raw/dns_any_query.txt",
	"icmp":      "templates/raw/icmp_echo.txt",
	"icmpv6":    "templates/raw/icmpv6_echo.txt",
	"ntp":       "templates/raw/ntp_monlist.txt",
	"ssdp":      "templates/raw/ssdp_search.txt",
	"memcached": "templates/raw/memcached.txt",
	"arp":       "templates/raw/arp_request.txt",
	"arp-spoof": "templates/raw/arp_reply.txt",
	"igmp":      "templates/raw/igmp_query.txt",
	"stp":       "templates/raw/stp_bpdu.txt",
}

// NewStrategyFactory creates a new StrategyFactory instance.
func NewStrategyFactory(cfg *config.StrategyConfig, bindIP string) *StrategyFactory {
	return &StrategyFactory{
		Config: cfg,
		BindIP: bindIP,
	}
}

// Create creates an AttackStrategy based on the strategy type.
func (f *StrategyFactory) Create() AttackStrategy {
	return f.CreateByType(f.Config.Type)
}

// CreateByType creates an AttackStrategy for the given type.
func (f *StrategyFactory) CreateByType(strategyType string) AttackStrategy {
	switch strategyType {
	case "slowloris":
		return NewSlowlorisClassicWithConfig(f.Config, f.BindIP)

	case "slowloris-keepalive", "keepsloworis":
		return NewSlowlorisWithConfig(f.Config, f.BindIP)

	case "keepalive":
		return NewKeepAliveHTTPWithConfig(f.Config, f.BindIP)

	case "normal":
		return NewNormalHTTPWithConfig(f.Config, f.BindIP)

	case "slow-post":
		return NewSlowPostWithConfig(f.Config, f.BindIP)

	case "slow-read":
		return NewSlowReadWithConfig(f.Config, f.BindIP)

	case "http-flood":
		return NewHTTPFloodWithConfig(f.Config, f.BindIP, "GET")

	case "h2-flood":
		return NewH2FloodWithConfig(f.Config, f.BindIP)

	case "heavy-payload":
		return NewHeavyPayloadWithConfig(f.Config, f.BindIP)

	case "hulk":
		return NewHULK(f.Config, f.BindIP)

	case "rudy":
		rudyCfg := RUDYConfig{
			ContentLength:         f.Config.ContentLength,
			ChunkDelayMin:         f.Config.ChunkDelayMin,
			ChunkDelayMax:         f.Config.ChunkDelayMax,
			ChunkSizeMin:          f.Config.ChunkSizeMin,
			ChunkSizeMax:          f.Config.ChunkSizeMax,
			PersistConnections:    f.Config.PersistConn,
			MaxRequestsPerSession: f.Config.MaxReqPerSession,
			KeepAliveTimeout:      f.Config.KeepAliveTimeout,
			SessionLifetime:       f.Config.SessionLifetime,
			UseJSON:               f.Config.UseJSON,
			UseMultipart:          f.Config.UseMultipart,
			RandomizePath:         f.Config.RandomizePath,
			EvasionLevel:          f.Config.EvasionLevel,
			ConnectTimeout:        f.Config.Timeout,
			SendBufferSize:        f.Config.SendBufferSize,
		}
		return NewRUDY(rudyCfg, f.BindIP)

	case "tcp-flood":
		return NewTCPFloodWithConfig(f.Config, f.BindIP)

	case "raw":
		// Resolve alias if needed
		templatePath := f.Config.PacketTemplate
		if resolved, ok := TemplateAliases[templatePath]; ok {
			templatePath = resolved
		}
		return NewRawStrategy(f.Config, f.BindIP, templatePath)

	default:
		log.Printf("Unknown strategy '%s', using 'keepalive'", strategyType)
		return NewKeepAliveHTTPWithConfig(f.Config, f.BindIP)
	}
}

// CreateWithMethod creates an HTTPFlood strategy with a specific HTTP method.
func (f *StrategyFactory) CreateWithMethod(strategyType, method string) AttackStrategy {
	if strategyType == "http-flood" {
		return NewHTTPFloodWithConfig(f.Config, f.BindIP, method)
	}
	return f.CreateByType(strategyType)
}

// AvailableStrategies returns a list of all available strategy types.
func AvailableStrategies() []StrategyInfo {
	return []StrategyInfo{
		{Name: "normal", Description: "Standard HTTP requests with connection per request"},
		{Name: "keepalive", Description: "HTTP requests with persistent connections (keep-alive)"},
		{Name: "slowloris", Description: "Classic Slowloris attack - slow header transmission"},
		{Name: "slowloris-keepalive", Description: "Slowloris with keep-alive packets"},
		{Name: "slow-post", Description: "Slow POST body transmission (simple RUDY)"},
		{Name: "slow-read", Description: "Slow response reading attack"},
		{Name: "http-flood", Description: "High-volume HTTP request flood"},
		{Name: "h2-flood", Description: "HTTP/2 multiplexed stream flood"},
		{Name: "heavy-payload", Description: "CPU-intensive payload attacks (JSON/XML/ReDoS)"},
		{Name: "hulk", Description: "Enhanced HULK - Dynamic evasion & flood"},
		{Name: "rudy", Description: "R.U.D.Y. attack - advanced slow POST with evasion"},
		{Name: "tcp-flood", Description: "TCP Connection Flood - exhaust server connection limits"},
		{Name: "raw", Description: "Low-Level Packet Flood using templates (UDP/TCP/ICMP)"},
	}
}

// StrategyInfo provides metadata about a strategy.
type StrategyInfo struct {
	Name        string
	Description string
}

// ValidateStrategyType checks if the given strategy type is valid.
func ValidateStrategyType(strategyType string) error {
	validTypes := map[string]bool{
		"normal":              true,
		"keepalive":           true,
		"slowloris":           true,
		"slowloris-keepalive": true,
		"keepsloworis":        true,
		"slow-post":           true,
		"slow-read":           true,
		"http-flood":          true,
		"h2-flood":            true,
		"heavy-payload":       true,
		"hulk":                true,
		"rudy":                true,
		"tcp-flood":           true,
		"raw":                 true,
	}

	if !validTypes[strategyType] {
		return fmt.Errorf("unknown strategy type: %s", strategyType)
	}
	return nil
}

// StrategyDefaults returns default configuration values for a specific strategy.
func StrategyDefaults(strategyType string) map[string]interface{} {
	defaults := map[string]interface{}{
		"timeout":           config.DefaultConnectTimeout,
		"keepalive":         config.DefaultKeepAliveInterval,
		"content-length":    config.DefaultContentLength,
		"read-size":         config.DefaultReadSize,
		"window-size":       config.DefaultWindowSize,
		"post-size":         config.DefaultPostDataSize,
		"requests-per-conn": config.DefaultRequestsPerConn,
		"session-lifetime":  config.DefaultSessionLifetime,
	}

	switch strategyType {
	case "h2-flood":
		defaults["max-streams"] = config.DefaultMaxStreams
		defaults["burst-size"] = config.DefaultBurstSize

	case "heavy-payload":
		defaults["payload-type"] = config.PayloadTypeDeepJSON
		defaults["payload-depth"] = config.DefaultPayloadDepth
		defaults["payload-size"] = config.DefaultPayloadSize

	case "hulk":
		defaults["timeout"] = config.DefaultConnectTimeout
		defaults["requests-per-conn"] = config.DefaultRequestsPerConn

	case "rudy":
		defaults["chunk-delay-min"] = config.DefaultChunkDelayMin
		defaults["chunk-delay-max"] = config.DefaultChunkDelayMax
		defaults["chunk-size-min"] = config.DefaultChunkSizeMin
		defaults["chunk-size-max"] = config.DefaultChunkSizeMax
		defaults["max-req-per-session"] = config.DefaultMaxReqPerSession
		defaults["keepalive-timeout"] = config.DefaultKeepAliveTimeout
		defaults["session-lifetime"] = config.DefaultSessionLifetime
		defaults["send-buffer"] = config.DefaultSendBufferSize
		defaults["evasion-level"] = config.EvasionLevelNormal

	case "slow-read":
		defaults["read-size"] = config.DefaultReadSize
		defaults["window-size"] = config.DefaultWindowSize

	case "tcp-flood":
		defaults["session-lifetime"] = config.DefaultSessionLifetime
		defaults["tcp-keepalive"] = true
		defaults["send-data"] = false
	}

	return defaults
}

// IsSlowAttack returns true if the strategy is a slow/low-bandwidth attack.
func IsSlowAttack(strategyType string) bool {
	slowAttacks := map[string]bool{
		"slowloris":           true,
		"slowloris-keepalive": true,
		"keepsloworis":        true,
		"slow-post":           true,
		"slow-read":           true,
		"rudy":                true,
	}
	return slowAttacks[strategyType]
}

// IsFloodAttack returns true if the strategy is a high-volume flood attack.
func IsFloodAttack(strategyType string) bool {
	floodAttacks := map[string]bool{
		"http-flood":    true,
		"h2-flood":      true,
		"heavy-payload": true,
		"hulk":          true,
		"tcp-flood":     true,
		"raw":           true,
	}
	return floodAttacks[strategyType]
}

// RecommendedSessions returns recommended session counts for strategy type.
func RecommendedSessions(strategyType string, baseCount int) (targetSessions, sessionsPerSec int) {
	if IsSlowAttack(strategyType) {
		// Slow attacks: more sessions, slower creation rate
		return baseCount * 2, baseCount / 10
	}
	if IsFloodAttack(strategyType) {
		// Flood attacks: fewer sessions, faster rate
		return baseCount / 2, baseCount
	}
	// Default
	return baseCount, baseCount / 5
}

// EstimateResourceUsage estimates resource usage for given configuration.
func EstimateResourceUsage(strategyType string, sessions int, duration time.Duration) ResourceEstimate {
	estimate := ResourceEstimate{
		Strategy:           strategyType,
		Sessions:           sessions,
		Duration:           duration,
		EstimatedConns:     sessions,
		EstimatedMemMB:     float64(sessions) * 0.1, // ~100KB per session baseline
		EstimatedBandwidth: "varies",
	}

	switch strategyType {
	case "slowloris", "slowloris-keepalive", "slow-post", "slow-read", "rudy":
		estimate.EstimatedConns = sessions
		estimate.EstimatedMemMB = float64(sessions) * 0.05 // Low memory per conn
		estimate.EstimatedBandwidth = "< 1 Mbps"

	case "http-flood":
		estimate.EstimatedConns = sessions * 10 // High conn turnover
		estimate.EstimatedMemMB = float64(sessions) * 0.2
		estimate.EstimatedBandwidth = "10-100 Mbps"

	case "h2-flood":
		estimate.EstimatedConns = sessions
		estimate.EstimatedMemMB = float64(sessions) * 0.5 // HTTP/2 overhead
		estimate.EstimatedBandwidth = "50-500 Mbps"

	case "heavy-payload":
		estimate.EstimatedConns = sessions
		estimate.EstimatedMemMB = float64(sessions) * 1.0 // Large payloads
		estimate.EstimatedBandwidth = "10-50 Mbps"

	case "hulk":
		estimate.EstimatedConns = sessions
		estimate.EstimatedMemMB = float64(sessions) * 0.3
		estimate.EstimatedBandwidth = "10-200 Mbps"

	case "tcp-flood":
		estimate.EstimatedConns = sessions
		estimate.EstimatedMemMB = float64(sessions) * 0.02 // Minimal per conn
		estimate.EstimatedBandwidth = "< 1 Mbps"
	}

	return estimate
}

// ResourceEstimate provides estimated resource usage.
type ResourceEstimate struct {
	Strategy           string
	Sessions           int
	Duration           time.Duration
	EstimatedConns     int
	EstimatedMemMB     float64
	EstimatedBandwidth string
}

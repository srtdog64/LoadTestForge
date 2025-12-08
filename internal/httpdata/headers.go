package httpdata

import (
	"fmt"
	"math/rand"
	"net/url"
	"strings"
)

// AcceptHeaders contains common Accept header values.
var AcceptHeaders = []string{
	"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
	"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
	"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
	"*/*",
}

// AcceptLanguages contains common Accept-Language header values.
var AcceptLanguages = []string{
	"ko-KR,ko;q=0.9,en-US;q=0.8,en;q=0.7",
	"en-US,en;q=0.9,ko;q=0.8",
	"ja-JP,ja;q=0.9,en-US;q=0.8,en;q=0.7",
	"en-US,en;q=0.9",
	"en-US,en;q=0.5",
	"en-GB,en;q=0.9,en-US;q=0.8",
	"zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7",
}

// AcceptEncodings contains common Accept-Encoding header values.
var AcceptEncodings = []string{
	"gzip, deflate, br",
	"gzip, deflate, br, zstd",
	"gzip, deflate",
	"gzip",
	"identity",
}

// CacheControlOptions contains common Cache-Control header values.
var CacheControlOptions = []string{
	"no-cache",
	"max-age=0",
	"no-store",
	"must-revalidate",
}

// Charsets contains common character encoding values.
var Charsets = []string{
	"UTF-8",
	"utf-8",
	"ISO-8859-1",
	"windows-1251",
	"EUC-KR",
	"Shift_JIS",
	"GB2312",
	"GBK",
}

// DeviceTypes contains device type identifiers for query parameters.
var DeviceTypes = []string{
	"desktop",
	"mobile",
	"tablet",
}

// ChromeVersions contains recent Chrome version numbers for Client Hints.
var ChromeVersions = []string{
	"121",
	"120",
	"119",
	"118",
}

// Platforms contains platform identifiers for Sec-CH-UA-Platform header.
var Platforms = []string{
	"Windows",
	"macOS",
	"Linux",
	"Android",
	"iOS",
}

// PlatformWeights defines the probability weights for each platform.
var PlatformWeights = []int{50, 20, 10, 15, 5}

// SecFetchSites contains Sec-Fetch-Site header values.
var SecFetchSites = []string{
	"none",
	"same-origin",
	"same-site",
	"cross-site",
}

// SecFetchSiteWeights defines the probability weights for Sec-Fetch-Site values.
var SecFetchSiteWeights = []int{40, 30, 20, 10}

// RandomAccept returns a random Accept header value.
func RandomAccept() string {
	return AcceptHeaders[rand.Intn(len(AcceptHeaders))]
}

// RandomAcceptLanguage returns a random Accept-Language header value.
func RandomAcceptLanguage() string {
	return AcceptLanguages[rand.Intn(len(AcceptLanguages))]
}

// RandomAcceptEncoding returns a random Accept-Encoding header value.
func RandomAcceptEncoding() string {
	return AcceptEncodings[rand.Intn(len(AcceptEncodings))]
}

// RandomCacheControl returns a random Cache-Control header value.
func RandomCacheControl() string {
	return CacheControlOptions[rand.Intn(len(CacheControlOptions))]
}

// RandomCharset returns a random character encoding value.
func RandomCharset() string {
	return Charsets[rand.Intn(len(Charsets))]
}

// RandomDeviceType returns a random device type.
func RandomDeviceType() string {
	return DeviceTypes[rand.Intn(len(DeviceTypes))]
}

// RandomChromeVersion returns a random Chrome version.
func RandomChromeVersion() string {
	return ChromeVersions[rand.Intn(len(ChromeVersions))]
}

// WeightedChoice selects a value from choices based on weights.
func WeightedChoice(choices []string, weights []int) string {
	total := 0
	for _, w := range weights {
		total += w
	}
	r := rand.Intn(total)
	cumulative := 0
	for i, w := range weights {
		cumulative += w
		if r < cumulative {
			return choices[i]
		}
	}
	return choices[0]
}

// RandomPlatform returns a weighted random platform.
func RandomPlatform() string {
	return WeightedChoice(Platforms, PlatformWeights)
}

// RandomSecFetchSite returns a weighted random Sec-Fetch-Site value.
func RandomSecFetchSite() string {
	return WeightedChoice(SecFetchSites, SecFetchSiteWeights)
}

// RandomMobile returns a random Sec-CH-UA-Mobile value.
func RandomMobile() string {
	if rand.Float32() < 0.25 {
		return "?1"
	}
	return "?0"
}

// RandomFakeIP generates a random fake IP address.
func RandomFakeIP() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		rand.Intn(223)+1, rand.Intn(256), rand.Intn(256), rand.Intn(254)+1)
}

// GenerateSessionID generates a random 16-character session ID.
func GenerateSessionID() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, 16)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

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

	hs.Add("Host", parsedURL.Host)
	hs.Add("User-Agent", userAgent)
	hs.Add("Accept", r.randomAccept())
	hs.Add("Accept-Language", RandomAcceptLanguage())
	hs.Add("Accept-Encoding", r.randomAcceptEncoding())
	hs.Add("Connection", "keep-alive")

	if r.AddDecoyHeaders {
		r.addDecoyHeaders(hs)
	}

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

	hs.Add("Host", parsedURL.Host)
	hs.Add("User-Agent", userAgent)
	hs.Add("Content-Type", contentType)
	hs.Add("Content-Length", fmt.Sprintf("%d", contentLength))
	hs.Add("Accept", r.randomAccept())
	hs.Add("Accept-Language", RandomAcceptLanguage())
	hs.Add("Accept-Encoding", r.randomAcceptEncoding())
	hs.Add("Connection", "keep-alive")

	if r.AddDecoyHeaders {
		r.addDecoyHeaders(hs)
	}

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

	hs.Add("Host", parsedURL.Host)
	hs.Add("User-Agent", userAgent)
	hs.Add("Accept", r.randomAccept())
	hs.Add("Accept-Language", RandomAcceptLanguage())
	hs.Add("Accept-Encoding", r.randomAcceptEncoding())
	hs.Add("Connection", "keep-alive")

	if r.AddDecoyHeaders {
		r.addDecoyHeaders(hs)
	}

	if r.ShuffleOrder {
		hs.Shuffle()
	}

	return fmt.Sprintf("GET %s?%d HTTP/1.1\r\n%s",
		path,
		rand.Intn(100000),
		hs.String(),
	)
}

func (r *HeaderRandomizer) addDecoyHeaders(hs *HeaderSet) {
	if rand.Intn(2) == 0 {
		hs.Add("Sec-Fetch-Dest", randomChoice([]string{"document", "empty", "image"}))
		hs.Add("Sec-Fetch-Mode", randomChoice([]string{"navigate", "cors", "no-cors"}))
		hs.Add("Sec-Fetch-Site", RandomSecFetchSite())
	}

	if rand.Intn(3) == 0 {
		hs.Add("DNT", "1")
	}

	if rand.Intn(2) == 0 {
		hs.Add("Upgrade-Insecure-Requests", "1")
	}

	if cache := randomChoice(append(CacheControlOptions, "")); cache != "" {
		hs.Add("Cache-Control", cache)
	}

	if rand.Intn(4) == 0 {
		hs.Add("Pragma", "no-cache")
	}

	if rand.Intn(5) == 0 {
		hs.Add("X-Requested-With", "XMLHttpRequest")
	}

	if rand.Intn(3) == 0 {
		hs.Add("Referer", RandomReferer())
	}
}

func (r *HeaderRandomizer) randomAccept() string {
	if !r.VaryAccept {
		return "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
	}
	return RandomAccept()
}

func (r *HeaderRandomizer) randomAcceptEncoding() string {
	if !r.VaryAccept {
		return "gzip, deflate"
	}
	return RandomAcceptEncoding()
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
		return fmt.Sprintf("X-Forwarded-For: %s\r\n", RandomFakeIP())
	case 3:
		return fmt.Sprintf("Cookie: sess=%s\r\n", GenerateSessionID())
	case 4:
		headerNames := []string{"Cache-Control", "Pragma", "DNT", "Upgrade-Insecure-Requests"}
		headerName := headerNames[rand.Intn(len(headerNames))]
		return fmt.Sprintf("X-%s: %d\r\n", headerName, rand.Intn(99999))
	default:
		return fmt.Sprintf("X-Request-ID: %d\r\n", rand.Intn(999999999))
	}
}

// =============================================================================
// Evasion Headers (for WAF/Bot Detection Bypass)
// =============================================================================

// EvasionLevel defines the sophistication of evasion headers.
const (
	EvasionLevelBasic      = 1 // Basic headers only
	EvasionLevelNormal     = 2 // Add Sec-Fetch-* headers
	EvasionLevelAggressive = 3 // Add Client Hints and sophisticated headers
)

// EvasionHeaderGenerator generates headers to evade WAF/bot detection.
type EvasionHeaderGenerator struct {
	Level int
}

// NewEvasionHeaderGenerator creates a new evasion header generator.
func NewEvasionHeaderGenerator(level int) *EvasionHeaderGenerator {
	if level < EvasionLevelBasic {
		level = EvasionLevelBasic
	}
	if level > EvasionLevelAggressive {
		level = EvasionLevelAggressive
	}
	return &EvasionHeaderGenerator{Level: level}
}

// GenerateEvasionHeaders returns a slice of evasion header strings.
// Headers are randomly selected based on the evasion level.
func (e *EvasionHeaderGenerator) GenerateEvasionHeaders() []string {
	var headers []string

	// Level 2+: Sec-Fetch headers
	if e.Level >= EvasionLevelNormal {
		secFetchHeaders := []string{
			"DNT: 1",
			"Upgrade-Insecure-Requests: 1",
			"Sec-Fetch-Dest: document",
			"Sec-Fetch-Mode: navigate",
			fmt.Sprintf("Sec-Fetch-Site: %s", RandomSecFetchSite()),
			"Sec-Fetch-User: ?1",
		}

		// Randomly select 2-4 headers
		count := rand.Intn(3) + 2
		perm := rand.Perm(len(secFetchHeaders))
		for i := 0; i < count && i < len(perm); i++ {
			headers = append(headers, secFetchHeaders[perm[i]])
		}
	}

	// Level 3: Client Hints and sophisticated headers
	if e.Level >= EvasionLevelAggressive {
		chromeVer := RandomChromeVersion()
		clientHints := []string{
			fmt.Sprintf(`Sec-CH-UA: "Chromium";v="%s", "Google Chrome";v="%s", "Not-A.Brand";v="99"`, chromeVer, chromeVer),
			fmt.Sprintf("Sec-CH-UA-Mobile: %s", RandomMobile()),
			fmt.Sprintf(`Sec-CH-UA-Platform: "%s"`, RandomPlatform()),
			fmt.Sprintf("X-Request-ID: %s", GenerateSessionID()),
			"TE: Trailers",
		}

		// Randomly select 1-3 headers
		count := rand.Intn(3) + 1
		perm := rand.Perm(len(clientHints))
		for i := 0; i < count && i < len(perm); i++ {
			headers = append(headers, clientHints[perm[i]])
		}
	}

	return headers
}

// GenerateFullEvasionHeaders returns all possible evasion headers for the level.
func (e *EvasionHeaderGenerator) GenerateFullEvasionHeaders() []string {
	var headers []string

	if e.Level >= EvasionLevelNormal {
		headers = append(headers,
			"DNT: 1",
			"Upgrade-Insecure-Requests: 1",
			"Sec-Fetch-Dest: document",
			"Sec-Fetch-Mode: navigate",
			fmt.Sprintf("Sec-Fetch-Site: %s", RandomSecFetchSite()),
			"Sec-Fetch-User: ?1",
		)
	}

	if e.Level >= EvasionLevelAggressive {
		chromeVer := RandomChromeVersion()
		headers = append(headers,
			fmt.Sprintf(`Sec-CH-UA: "Chromium";v="%s", "Google Chrome";v="%s", "Not-A.Brand";v="99"`, chromeVer, chromeVer),
			fmt.Sprintf("Sec-CH-UA-Mobile: %s", RandomMobile()),
			fmt.Sprintf(`Sec-CH-UA-Platform: "%s"`, RandomPlatform()),
		)
	}

	return headers
}

// AddEvasionHeadersToSet adds evasion headers to a HeaderSet.
func (e *EvasionHeaderGenerator) AddEvasionHeadersToSet(hs *HeaderSet) {
	for _, h := range e.GenerateEvasionHeaders() {
		parts := strings.SplitN(h, ": ", 2)
		if len(parts) == 2 {
			hs.Add(parts[0], parts[1])
		}
	}
}

// =============================================================================
// Stealth Headers (Browser Fingerprinting)
// =============================================================================

// StealthHeaderSet generates a complete set of browser-like headers.
type StealthHeaderSet struct {
	UserAgent    string
	EvasionLevel int
	AddOrigin    bool
	AddReferer   bool
	AddCSRF      bool
}

// NewStealthHeaderSet creates a new stealth header configuration.
func NewStealthHeaderSet(userAgent string, evasionLevel int) *StealthHeaderSet {
	return &StealthHeaderSet{
		UserAgent:    userAgent,
		EvasionLevel: evasionLevel,
		AddOrigin:    rand.Float32() < 0.7,
		AddReferer:   rand.Float32() < 0.8,
		AddCSRF:      rand.Float32() < 0.5,
	}
}

// GenerateHeaders returns a slice of stealth header strings for POST requests.
func (s *StealthHeaderSet) GenerateHeaders(host, path, contentType string, contentLength int) []string {
	var headers []string

	// Essential headers
	headers = append(headers, fmt.Sprintf("Host: %s", host))
	headers = append(headers, fmt.Sprintf("User-Agent: %s", s.UserAgent))
	headers = append(headers, fmt.Sprintf("Content-Type: %s", contentType))
	headers = append(headers, fmt.Sprintf("Content-Length: %d", contentLength))

	// Standard browser headers
	headers = append(headers, fmt.Sprintf("Accept: %s", RandomAccept()))
	headers = append(headers, fmt.Sprintf("Accept-Language: %s", RandomAcceptLanguage()))
	headers = append(headers, fmt.Sprintf("Accept-Encoding: %s", RandomAcceptEncoding()))
	headers = append(headers, "Connection: keep-alive")

	// Conditional headers
	if s.AddOrigin {
		headers = append(headers, fmt.Sprintf("Origin: https://%s", host))
	}

	if s.AddReferer {
		headers = append(headers, fmt.Sprintf("Referer: https://%s%s", host, path))
	}

	if s.AddCSRF {
		headers = append(headers, fmt.Sprintf("X-CSRF-Token: %s", GenerateSessionID()))
		headers = append(headers, "X-Requested-With: XMLHttpRequest")
	}

	// Evasion headers
	if s.EvasionLevel >= EvasionLevelNormal {
		evasion := NewEvasionHeaderGenerator(s.EvasionLevel)
		headers = append(headers, evasion.GenerateEvasionHeaders()...)
	}

	return headers
}

// ShuffleHeaders randomly reorders a slice of headers.
func ShuffleHeaders(headers []string) []string {
	shuffled := make([]string, len(headers))
	copy(shuffled, headers)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	return shuffled
}

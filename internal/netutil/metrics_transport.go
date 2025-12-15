package netutil

import (
	"net/http"
	"time"
)

// MetricsReporter defines the interface for reporting metrics.
// This interface matches strategy.MetricsCallback to avoid import cycles.
type MetricsReporter interface {
	RecordSuccessWithLatency(duration time.Duration)
	RecordFailure()
}

// MetricsTransport Wraps an existing RoundTripper and reports request metrics
// (success, failure, latency) to the provided callback.
type MetricsTransport struct {
	BaseTransport http.RoundTripper
	Metrics       MetricsReporter
}

// NewMetricsTransport creates a new MetricsTransport.
func NewMetricsTransport(base http.RoundTripper, metrics MetricsReporter) *MetricsTransport {
	return &MetricsTransport{
		BaseTransport: base,
		Metrics:       metrics,
	}
}

// RoundTrip executes the HTTP transaction and records metrics.
func (t *MetricsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	startTime := time.Now()

	// Use BaseTransport or DefaultTransport if nil
	transport := t.BaseTransport
	if transport == nil {
		transport = http.DefaultTransport
	}

	resp, err := transport.RoundTrip(req)
	latency := time.Since(startTime)

	if t.Metrics != nil {
		if err != nil {
			t.Metrics.RecordFailure()
		} else {
			// Check status code for success/failure
			// Standard LoadTestForge logic: < 400 is usually success
			if resp.StatusCode > 0 && resp.StatusCode < 400 {
				t.Metrics.RecordSuccessWithLatency(latency)
			} else {
				t.Metrics.RecordFailure()
			}
		}
	}

	return resp, err
}

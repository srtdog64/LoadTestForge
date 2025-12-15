package errors

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorType
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: ErrorTypeUnknown,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: ErrorTypeCanceled,
		},
		{
			name:     "deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: ErrorTypeTimeout,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: ErrorTypeNetwork,
		},
		{
			name:     "connection reset",
			err:      errors.New("connection reset by peer"),
			expected: ErrorTypeNetwork,
		},
		{
			name:     "tls error",
			err:      errors.New("tls: handshake failure"),
			expected: ErrorTypeTLS,
		},
		{
			name:     "certificate error",
			err:      errors.New("x509: certificate signed by unknown authority"),
			expected: ErrorTypeTLS,
		},
		{
			name:     "malformed response",
			err:      errors.New("malformed HTTP response"),
			expected: ErrorTypeProtocol,
		},
		{
			name:     "timeout error",
			err:      errors.New("i/o timeout"),
			expected: ErrorTypeTimeout,
		},
		{
			name:     "dns lookup error",
			err:      errors.New("lookup failed"),
			expected: ErrorTypeNetwork,
		},
		{
			name:     "dial tcp error",
			err:      errors.New("dial tcp: connection refused"),
			expected: ErrorTypeNetwork,
		},
		{
			name:     "unexpected EOF",
			err:      errors.New("unexpected EOF"),
			expected: ErrorTypeProtocol,
		},
		{
			name:     "unknown error",
			err:      errors.New("some random error"),
			expected: ErrorTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Classify(tt.err)
			if result != tt.expected {
				t.Errorf("Classify(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestErrorTypeString(t *testing.T) {
	tests := []struct {
		errType  ErrorType
		expected string
	}{
		{ErrorTypeUnknown, "unknown"},
		{ErrorTypeNetwork, "network"},
		{ErrorTypeTimeout, "timeout"},
		{ErrorTypeHTTP, "http"},
		{ErrorTypeTLS, "tls"},
		{ErrorTypeProtocol, "protocol"},
		{ErrorTypeCanceled, "canceled"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.errType.String(); got != tt.expected {
				t.Errorf("ErrorType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestClassifiedError(t *testing.T) {
	baseErr := errors.New("base error")
	ce := NewClassifiedError(ErrorTypeNetwork, baseErr, "connection failed")

	// Test Error() method
	expected := "[network] connection failed: base error"
	if ce.Error() != expected {
		t.Errorf("ClassifiedError.Error() = %v, want %v", ce.Error(), expected)
	}

	// Test Unwrap() method
	if ce.Unwrap() != baseErr {
		t.Errorf("ClassifiedError.Unwrap() = %v, want %v", ce.Unwrap(), baseErr)
	}

	// Test Is() method
	if !ce.Is(baseErr) {
		t.Error("ClassifiedError.Is(baseErr) should return true")
	}
}

func TestClassifiedErrorNoMessage(t *testing.T) {
	baseErr := errors.New("base error")
	ce := NewClassifiedError(ErrorTypeTimeout, baseErr, "")

	expected := "[timeout] base error"
	if ce.Error() != expected {
		t.Errorf("ClassifiedError.Error() = %v, want %v", ce.Error(), expected)
	}
}

func TestClassifyAndWrap(t *testing.T) {
	err := errors.New("connection refused")
	ce := ClassifyAndWrap(err, "dial failed")

	if ce == nil {
		t.Fatal("ClassifyAndWrap should not return nil for non-nil error")
	}

	if ce.Type != ErrorTypeNetwork {
		t.Errorf("ClassifyAndWrap error type = %v, want %v", ce.Type, ErrorTypeNetwork)
	}

	if ce.Message != "dial failed" {
		t.Errorf("ClassifyAndWrap message = %v, want %v", ce.Message, "dial failed")
	}
}

func TestClassifyAndWrapNil(t *testing.T) {
	ce := ClassifyAndWrap(nil, "test")
	if ce != nil {
		t.Error("ClassifyAndWrap(nil) should return nil")
	}
}

func TestIsHelpers(t *testing.T) {
	timeoutErr := errors.New("i/o timeout")
	networkErr := errors.New("connection refused")
	tlsErr := errors.New("tls: handshake failure")
	canceledErr := context.Canceled

	if !IsTimeout(timeoutErr) {
		t.Error("IsTimeout should return true for timeout error")
	}

	if !IsNetwork(networkErr) {
		t.Error("IsNetwork should return true for network error")
	}

	if !IsTLS(tlsErr) {
		t.Error("IsTLS should return true for TLS error")
	}

	if !IsCanceled(canceledErr) {
		t.Error("IsCanceled should return true for canceled error")
	}
}

func TestIsHelpersWithNil(t *testing.T) {
	if IsTimeout(nil) {
		t.Error("IsTimeout(nil) should return false")
	}
	if IsNetwork(nil) {
		t.Error("IsNetwork(nil) should return false")
	}
	if IsTLS(nil) {
		t.Error("IsTLS(nil) should return false")
	}
	if IsCanceled(nil) {
		t.Error("IsCanceled(nil) should return false")
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil error", nil, false},
		{"timeout error", errors.New("i/o timeout"), true},
		{"network error", errors.New("connection refused"), true},
		{"tls error", errors.New("tls: handshake failure"), false},
		{"protocol error", errors.New("malformed response"), false},
		{"canceled", context.Canceled, false},
		{"unknown error", errors.New("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.retryable {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.retryable)
			}
		})
	}
}

func TestHTTPError(t *testing.T) {
	httpErr := NewHTTPError(404, "Not Found", "page not found")

	if httpErr.Error() != "HTTP 404 Not Found: page not found" {
		t.Errorf("HTTPError.Error() = %v", httpErr.Error())
	}

	if httpErr.IsClientError() != true {
		t.Error("404 should be client error")
	}

	if httpErr.IsServerError() != false {
		t.Error("404 should not be server error")
	}

	serverErr := NewHTTPError(500, "Internal Server Error", "")
	if serverErr.IsClientError() != false {
		t.Error("500 should not be client error")
	}
	if serverErr.IsServerError() != true {
		t.Error("500 should be server error")
	}

	if serverErr.Error() != "HTTP 500 Internal Server Error" {
		t.Errorf("HTTPError without message = %v", serverErr.Error())
	}
}

func TestIsHTTPError(t *testing.T) {
	httpErr := NewHTTPError(500, "Internal Server Error", "")
	regularErr := errors.New("not an http error")

	if !IsHTTPError(httpErr) {
		t.Error("IsHTTPError should return true for HTTPError")
	}

	if IsHTTPError(regularErr) {
		t.Error("IsHTTPError should return false for regular error")
	}
}

func TestGetHTTPError(t *testing.T) {
	httpErr := NewHTTPError(500, "Internal Server Error", "")

	got := GetHTTPError(httpErr)
	if got != httpErr {
		t.Error("GetHTTPError should return the HTTPError")
	}

	regularErr := errors.New("not an http error")
	if GetHTTPError(regularErr) != nil {
		t.Error("GetHTTPError should return nil for regular error")
	}
}

func TestErrorStats(t *testing.T) {
	stats := &ErrorStats{}

	stats.Record(errors.New("connection refused"))
	stats.Record(errors.New("i/o timeout"))
	stats.Record(errors.New("tls: error"))
	stats.Record(errors.New("malformed"))
	stats.Record(context.Canceled)
	stats.Record(errors.New("unknown"))
	stats.Record(nil)

	if stats.Network != 1 {
		t.Errorf("Network errors = %d, want 1", stats.Network)
	}
	if stats.Timeout != 1 {
		t.Errorf("Timeout errors = %d, want 1", stats.Timeout)
	}
	if stats.TLS != 1 {
		t.Errorf("TLS errors = %d, want 1", stats.TLS)
	}
	if stats.Protocol != 1 {
		t.Errorf("Protocol errors = %d, want 1", stats.Protocol)
	}
	if stats.Canceled != 1 {
		t.Errorf("Canceled errors = %d, want 1", stats.Canceled)
	}
	if stats.Unknown != 1 {
		t.Errorf("Unknown errors = %d, want 1", stats.Unknown)
	}

	if stats.Total() != 6 {
		t.Errorf("Total errors = %d, want 6", stats.Total())
	}
}

func TestClassifyWithNetError(t *testing.T) {
	// Test with net.OpError
	opErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: errors.New("connection refused"),
	}

	if Classify(opErr) != ErrorTypeNetwork {
		t.Error("net.OpError should be classified as network error")
	}

	// Test with net.DNSError
	dnsErr := &net.DNSError{
		Err:  "no such host",
		Name: "example.invalid",
	}

	if Classify(dnsErr) != ErrorTypeNetwork {
		t.Error("net.DNSError should be classified as network error")
	}
}

func TestClassifyWithClassifiedError(t *testing.T) {
	ce := NewClassifiedError(ErrorTypeHTTP, errors.New("404"), "not found")

	if IsTimeout(ce) {
		t.Error("HTTP error should not be timeout")
	}

	// Create a classified timeout error
	timeoutCE := NewClassifiedError(ErrorTypeTimeout, errors.New("deadline exceeded"), "request timed out")

	if !IsTimeout(timeoutCE) {
		t.Error("Classified timeout error should be recognized")
	}
}

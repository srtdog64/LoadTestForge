package errors

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
)

// ErrorType represents the category of an error.
type ErrorType int

const (
	// ErrorTypeUnknown is an unclassified error.
	ErrorTypeUnknown ErrorType = iota
	// ErrorTypeNetwork represents network-level errors (DNS, connection refused, etc.)
	ErrorTypeNetwork
	// ErrorTypeTimeout represents timeout errors (deadline exceeded)
	ErrorTypeTimeout
	// ErrorTypeHTTP represents HTTP-level errors (4xx, 5xx responses)
	ErrorTypeHTTP
	// ErrorTypeTLS represents TLS/SSL errors
	ErrorTypeTLS
	// ErrorTypeProtocol represents protocol errors (malformed response, etc.)
	ErrorTypeProtocol
	// ErrorTypeCanceled represents context cancellation
	ErrorTypeCanceled
)

// String returns a human-readable representation of the error type.
func (e ErrorType) String() string {
	switch e {
	case ErrorTypeNetwork:
		return "network"
	case ErrorTypeTimeout:
		return "timeout"
	case ErrorTypeHTTP:
		return "http"
	case ErrorTypeTLS:
		return "tls"
	case ErrorTypeProtocol:
		return "protocol"
	case ErrorTypeCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}

// ClassifiedError wraps an error with its classification.
type ClassifiedError struct {
	Type    ErrorType
	Err     error
	Message string
}

// Error implements the error interface.
func (e *ClassifiedError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %v", e.Type, e.Err)
}

// Unwrap returns the underlying error.
func (e *ClassifiedError) Unwrap() error {
	return e.Err
}

// Is checks if the error matches the target.
func (e *ClassifiedError) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// NewClassifiedError creates a new ClassifiedError.
func NewClassifiedError(errType ErrorType, err error, message string) *ClassifiedError {
	return &ClassifiedError{
		Type:    errType,
		Err:     err,
		Message: message,
	}
}

// Classify analyzes an error and returns its classification.
func Classify(err error) ErrorType {
	if err == nil {
		return ErrorTypeUnknown
	}

	errStr := err.Error()

	// Check for context cancellation
	if errors.Is(err, syscall.ECANCELED) || strings.Contains(errStr, "context canceled") {
		return ErrorTypeCanceled
	}

	// Check for timeout
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return ErrorTypeTimeout
	}
	if strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "i/o timeout") {
		return ErrorTypeTimeout
	}

	// Check for TLS errors
	if strings.Contains(errStr, "tls:") ||
		strings.Contains(errStr, "certificate") ||
		strings.Contains(errStr, "x509:") ||
		strings.Contains(errStr, "handshake") {
		return ErrorTypeTLS
	}

	// Check for network errors
	if _, ok := err.(*net.OpError); ok {
		return ErrorTypeNetwork
	}
	if _, ok := err.(*net.DNSError); ok {
		return ErrorTypeNetwork
	}
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no route to host") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "connection failed") ||
		strings.Contains(errStr, "dial tcp") ||
		strings.Contains(errStr, "lookup") {
		return ErrorTypeNetwork
	}

	// Check for protocol errors
	if strings.Contains(errStr, "malformed") ||
		strings.Contains(errStr, "invalid") ||
		strings.Contains(errStr, "unexpected EOF") ||
		strings.Contains(errStr, "protocol error") {
		return ErrorTypeProtocol
	}

	return ErrorTypeUnknown
}

// ClassifyAndWrap classifies an error and wraps it with the classification.
func ClassifyAndWrap(err error, message string) *ClassifiedError {
	if err == nil {
		return nil
	}
	return NewClassifiedError(Classify(err), err, message)
}

// IsTimeout returns true if the error is a timeout error.
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	if ce, ok := err.(*ClassifiedError); ok {
		return ce.Type == ErrorTypeTimeout
	}
	return Classify(err) == ErrorTypeTimeout
}

// IsNetwork returns true if the error is a network error.
func IsNetwork(err error) bool {
	if err == nil {
		return false
	}
	if ce, ok := err.(*ClassifiedError); ok {
		return ce.Type == ErrorTypeNetwork
	}
	return Classify(err) == ErrorTypeNetwork
}

// IsTLS returns true if the error is a TLS error.
func IsTLS(err error) bool {
	if err == nil {
		return false
	}
	if ce, ok := err.(*ClassifiedError); ok {
		return ce.Type == ErrorTypeTLS
	}
	return Classify(err) == ErrorTypeTLS
}

// IsCanceled returns true if the error is due to context cancellation.
func IsCanceled(err error) bool {
	if err == nil {
		return false
	}
	if ce, ok := err.(*ClassifiedError); ok {
		return ce.Type == ErrorTypeCanceled
	}
	return Classify(err) == ErrorTypeCanceled
}

// IsRetryable returns true if the error type suggests the operation can be retried.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	errType := Classify(err)
	if ce, ok := err.(*ClassifiedError); ok {
		errType = ce.Type
	}

	switch errType {
	case ErrorTypeTimeout, ErrorTypeNetwork:
		return true
	case ErrorTypeTLS, ErrorTypeProtocol, ErrorTypeCanceled:
		return false
	default:
		return false
	}
}

// HTTPError represents an HTTP-level error with status code.
type HTTPError struct {
	StatusCode int
	Status     string
	Message    string
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("HTTP %d %s: %s", e.StatusCode, e.Status, e.Message)
	}
	return fmt.Sprintf("HTTP %d %s", e.StatusCode, e.Status)
}

// NewHTTPError creates a new HTTPError.
func NewHTTPError(statusCode int, status, message string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Status:     status,
		Message:    message,
	}
}

// IsClientError returns true if the status code is 4xx.
func (e *HTTPError) IsClientError() bool {
	return e.StatusCode >= 400 && e.StatusCode < 500
}

// IsServerError returns true if the status code is 5xx.
func (e *HTTPError) IsServerError() bool {
	return e.StatusCode >= 500 && e.StatusCode < 600
}

// IsHTTPError checks if an error is an HTTPError.
func IsHTTPError(err error) bool {
	var httpErr *HTTPError
	return errors.As(err, &httpErr)
}

// GetHTTPError extracts HTTPError from an error if present.
func GetHTTPError(err error) *HTTPError {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr
	}
	return nil
}

// ErrorStats tracks error statistics by type.
type ErrorStats struct {
	Network  int64
	Timeout  int64
	HTTP     int64
	TLS      int64
	Protocol int64
	Canceled int64
	Unknown  int64
}

// Record records an error in the statistics.
func (s *ErrorStats) Record(err error) {
	if err == nil {
		return
	}

	errType := Classify(err)
	if ce, ok := err.(*ClassifiedError); ok {
		errType = ce.Type
	}

	switch errType {
	case ErrorTypeNetwork:
		s.Network++
	case ErrorTypeTimeout:
		s.Timeout++
	case ErrorTypeHTTP:
		s.HTTP++
	case ErrorTypeTLS:
		s.TLS++
	case ErrorTypeProtocol:
		s.Protocol++
	case ErrorTypeCanceled:
		s.Canceled++
	default:
		s.Unknown++
	}
}

// Total returns the total number of errors.
func (s *ErrorStats) Total() int64 {
	return s.Network + s.Timeout + s.HTTP + s.TLS + s.Protocol + s.Canceled + s.Unknown
}

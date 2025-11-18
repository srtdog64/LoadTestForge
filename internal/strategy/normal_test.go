package strategy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNormalHTTP_Execute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	strategy := NewNormalHTTP(5*time.Second, "")
	target := Target{
		URL:    server.URL,
		Method: "GET",
		Headers: map[string]string{
			"User-Agent": "test",
		},
	}

	ctx := context.Background()
	err := strategy.Execute(ctx, target)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestNormalHTTP_ExecuteWithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	strategy := NewNormalHTTP(500*time.Millisecond, "")
	target := Target{
		URL:    server.URL,
		Method: "GET",
	}

	ctx := context.Background()
	err := strategy.Execute(ctx, target)

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

func TestNormalHTTP_ExecuteWithError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	strategy := NewNormalHTTP(5*time.Second, "")
	target := Target{
		URL:    server.URL,
		Method: "GET",
	}

	ctx := context.Background()
	err := strategy.Execute(ctx, target)

	if err == nil {
		t.Error("Expected error for 500 status, got nil")
	}
}

func TestNormalHTTP_Name(t *testing.T) {
	strategy := NewNormalHTTP(5*time.Second, "")
	if strategy.Name() != "normal-http" {
		t.Errorf("Expected name 'normal-http', got '%s'", strategy.Name())
	}
}

func BenchmarkNormalHTTP_Execute(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	strategy := NewNormalHTTP(5*time.Second, "")
	target := Target{
		URL:    server.URL,
		Method: "GET",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.Execute(ctx, target)
	}
}

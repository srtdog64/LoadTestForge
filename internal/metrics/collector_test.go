package metrics

import (
	"testing"
	"time"
)

func TestCollector_RecordSuccess(t *testing.T) {
	collector := NewCollector()
	defer collector.Stop()

	collector.RecordSuccess()
	collector.RecordSuccess()

	stats := collector.GetStats()

	if stats.Total != 2 {
		t.Errorf("Expected 2 total requests, got %d", stats.Total)
	}

	if stats.Success != 2 {
		t.Errorf("Expected 2 successful requests, got %d", stats.Success)
	}

	if stats.Failed != 0 {
		t.Errorf("Expected 0 failed requests, got %d", stats.Failed)
	}
}

func TestCollector_RecordFailure(t *testing.T) {
	collector := NewCollector()
	defer collector.Stop()

	collector.RecordFailure()

	stats := collector.GetStats()

	if stats.Total != 1 {
		t.Errorf("Expected 1 total request, got %d", stats.Total)
	}

	if stats.Failed != 1 {
		t.Errorf("Expected 1 failed request, got %d", stats.Failed)
	}
}

func TestCollector_ActiveSessions(t *testing.T) {
	collector := NewCollector()
	defer collector.Stop()

	collector.IncrementActive()
	collector.IncrementActive()

	stats := collector.GetStats()
	if stats.Active != 2 {
		t.Errorf("Expected 2 active sessions, got %d", stats.Active)
	}

	collector.DecrementActive()

	stats = collector.GetStats()
	if stats.Active != 1 {
		t.Errorf("Expected 1 active session, got %d", stats.Active)
	}
}

func TestCollector_SuccessRate(t *testing.T) {
	collector := NewCollector()
	defer collector.Stop()

	collector.RecordSuccess()
	collector.RecordSuccess()
	collector.RecordSuccess()
	collector.RecordFailure()

	stats := collector.GetStats()

	expectedRate := 75.0
	if stats.SuccessRate != expectedRate {
		t.Errorf("Expected success rate %.2f%%, got %.2f%%", expectedRate, stats.SuccessRate)
	}
}

func TestCollector_RequestsPerSecond(t *testing.T) {
	collector := NewCollector()
	defer collector.Stop()

	for i := 0; i < 10; i++ {
		collector.RecordSuccess()
	}

	time.Sleep(1100 * time.Millisecond)

	stats := collector.GetStats()

	if stats.AvgPerSec < 9 || stats.AvgPerSec > 11 {
		t.Errorf("Expected avg per sec around 10, got %.2f", stats.AvgPerSec)
	}
}

func BenchmarkCollector_RecordSuccess(b *testing.B) {
	collector := NewCollector()
	defer collector.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordSuccess()
	}
}

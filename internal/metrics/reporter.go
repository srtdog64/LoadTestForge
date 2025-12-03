package metrics

import (
	"context"
	"fmt"
	"math"
	"time"
)

type Reporter struct {
	collector *Collector
}

func NewReporter(collector *Collector) *Reporter {
	return &Reporter{
		collector: collector,
	}
}

func (r *Reporter) Start(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			r.printFinalReport(startTime)
			return
		case <-ticker.C:
			r.printStats(startTime)
		}
	}
}

func (r *Reporter) printStats(startTime time.Time) {
	stats := r.collector.GetStats()
	elapsed := time.Since(startTime)

	fmt.Print("\033[H\033[2J")

	fmt.Println("=== LoadTestForge Live Stats ===")
	fmt.Printf("Elapsed Time:      %v\n", elapsed.Round(time.Second))
	fmt.Println()

	fmt.Println("--- Session Metrics ---")
	fmt.Printf("Active Goroutines: %d\n", stats.Active)
	fmt.Printf("TCP Connections:   %d (open sockets)\n", stats.TCPConnections)
	fmt.Printf("Active Conns:      %d (tracked)\n", stats.ActiveConnCount)

	if stats.Active > 0 && stats.TCPConnections > 0 {
		accuracy := float64(stats.TCPConnections) / float64(stats.Active) * 100
		fmt.Printf("Session Accuracy:  %.2f%%\n", accuracy)
	}
	fmt.Println()

	fmt.Println("--- Connection Health ---")
	fmt.Printf("Socket Timeouts:   %d\n", stats.SocketTimeouts)
	fmt.Printf("Socket Reconnects: %d\n", stats.SocketReconnects)

	if stats.AvgConnLifetime > 0 {
		fmt.Printf("Avg Conn Lifetime: %v\n", stats.AvgConnLifetime.Round(time.Second))
		fmt.Printf("Min/Max Lifetime:  %v / %v\n",
			stats.MinConnLifetime.Round(time.Second),
			stats.MaxConnLifetime.Round(time.Second))
	}
	fmt.Println()

	fmt.Println("--- Request Metrics ---")
	fmt.Printf("Total Requests:    %d\n", stats.Total)
	fmt.Printf("Success:           %d (%.2f%%)\n", stats.Success, stats.SuccessRate)
	fmt.Printf("Failed:            %d\n", stats.Failed)
	fmt.Println()

	fmt.Printf("Requests/sec:      %.2f (sigma=%.2f)\n", stats.AvgPerSec, stats.StdDev)
	fmt.Printf("Min/Max:           %d / %d\n", stats.MinPerSec, stats.MaxPerSec)
	fmt.Printf("Percentiles:       p50=%d, p95=%d, p99=%d\n", stats.P50, stats.P95, stats.P99)
	fmt.Println()

	if stats.LatencyEnabled && stats.LatencyCount > 0 {
		fmt.Println("--- Response Latency ---")
		fmt.Printf("Samples:           %d\n", stats.LatencyCount)
		fmt.Printf("Average:           %.2f ms\n", stats.LatencyAvg/1000.0)
		fmt.Printf("Min/Max:           %.2f ms / %.2f ms\n",
			float64(stats.LatencyMin)/1000.0,
			float64(stats.LatencyMax)/1000.0)
		fmt.Printf("Percentiles:       p50=%.2f ms, p95=%.2f ms, p99=%.2f ms\n",
			float64(stats.LatencyP50)/1000.0,
			float64(stats.LatencyP95)/1000.0,
			float64(stats.LatencyP99)/1000.0)
		fmt.Println()
	}

	fmt.Println("--- Status ---")
	if stats.AvgPerSec > 0 {
		deviation := (stats.StdDev / stats.AvgPerSec) * 100
		fmt.Printf("Rate Deviation:    %.2f%%\n", deviation)

		if deviation <= 10 {
			fmt.Println("Rate Status:       [OK] Within target (+/-10%)")
		} else {
			fmt.Println("Rate Status:       [WARN] Exceeds target (+/-10%)")
		}
	}

	if stats.Active > 0 && stats.TCPConnections > 0 {
		sessionDeviation := math.Abs(float64(stats.TCPConnections-int64(stats.Active))) / float64(stats.Active) * 100
		if sessionDeviation <= 10 {
			fmt.Println("Session Status:    [OK] Within target (+/-10%)")
		} else {
			fmt.Printf("Session Status:    [WARN] Deviation %.2f%%\n", sessionDeviation)
		}
	}

	if stats.SocketTimeouts > 0 {
		timeoutRate := float64(stats.SocketTimeouts) / float64(stats.Total) * 100
		if timeoutRate > 5 {
			fmt.Printf("[ALERT] High timeout rate (%.2f%%)\n", timeoutRate)
		}
	}

	if stats.LatencyEnabled && stats.LatencyP99 > 3000000 {
		fmt.Printf("[ALERT] High p99 latency (%.2f ms)\n", float64(stats.LatencyP99)/1000.0)
	}
}

// TestResult represents the overall pass/fail verdict
type TestResult struct {
	Passed   bool
	Failures []string
}

// EvaluateTestResult determines if the test passed based on metrics
func EvaluateTestResult(stats Stats) TestResult {
	result := TestResult{Passed: true, Failures: make([]string, 0)}

	// 성공률이 90% 미만이면 실패
	if stats.Total > 0 && stats.SuccessRate < 90.0 {
		result.Passed = false
		result.Failures = append(result.Failures, fmt.Sprintf("Success rate %.2f%% below 90%% threshold", stats.SuccessRate))
	}

	// 요청률 편차가 20% 초과면 실패
	if stats.AvgPerSec > 0 {
		deviation := (stats.StdDev / stats.AvgPerSec) * 100
		if deviation > 20 {
			result.Passed = false
			result.Failures = append(result.Failures, fmt.Sprintf("Rate deviation %.2f%% exceeds 20%% threshold", deviation))
		}
	}

	// p99 레이턴시가 5초 초과면 실패
	if stats.LatencyEnabled && stats.LatencyP99 > 5000000 {
		result.Passed = false
		result.Failures = append(result.Failures, fmt.Sprintf("p99 latency %.2f ms exceeds 5000ms threshold", float64(stats.LatencyP99)/1000.0))
	}

	// 타임아웃 비율이 10% 초과면 실패
	if stats.Total > 0 {
		timeoutRate := float64(stats.SocketTimeouts) / float64(stats.Total) * 100
		if timeoutRate > 10 {
			result.Passed = false
			result.Failures = append(result.Failures, fmt.Sprintf("Timeout rate %.2f%% exceeds 10%% threshold", timeoutRate))
		}
	}

	return result
}

func (r *Reporter) printFinalReport(startTime time.Time) {
	stats := r.collector.GetStats()
	elapsed := time.Since(startTime)

	fmt.Println("\n=== LoadTestForge Final Report ===")
	fmt.Printf("Total Duration:    %v\n", elapsed.Round(time.Millisecond))
	fmt.Println()

	fmt.Println("--- Session Summary ---")
	fmt.Printf("Active Goroutines: %d\n", stats.Active)
	fmt.Printf("TCP Connections:   %d\n", stats.TCPConnections)
	fmt.Printf("Active Conns:      %d\n", stats.ActiveConnCount)

	if stats.Active > 0 && stats.TCPConnections > 0 {
		accuracy := float64(stats.TCPConnections) / float64(stats.Active) * 100
		fmt.Printf("Session Accuracy:  %.2f%%\n", accuracy)
	}
	fmt.Println()

	fmt.Println("--- Connection Summary ---")
	fmt.Printf("Socket Timeouts:   %d\n", stats.SocketTimeouts)
	fmt.Printf("Socket Reconnects: %d\n", stats.SocketReconnects)

	if stats.SocketTimeouts > 0 || stats.SocketReconnects > 0 {
		if stats.Total > 0 {
			timeoutRate := float64(stats.SocketTimeouts) / float64(stats.Total) * 100
			reconnectRate := float64(stats.SocketReconnects) / float64(stats.Active) * 100
			fmt.Printf("Timeout Rate:      %.2f%%\n", timeoutRate)
			fmt.Printf("Reconnect Rate:    %.2f%%\n", reconnectRate)
		}
	}

	if stats.AvgConnLifetime > 0 {
		fmt.Printf("Avg Conn Lifetime: %v\n", stats.AvgConnLifetime.Round(time.Second))
		fmt.Printf("Min/Max Lifetime:  %v / %v\n",
			stats.MinConnLifetime.Round(time.Second),
			stats.MaxConnLifetime.Round(time.Second))
	}
	fmt.Println()

	fmt.Println("--- Request Summary ---")
	fmt.Printf("Total Requests:    %d\n", stats.Total)
	fmt.Printf("Success:           %d (%.2f%%)\n", stats.Success, stats.SuccessRate)
	fmt.Printf("Failed:            %d\n", stats.Failed)
	fmt.Println()

	fmt.Printf("Avg Req/sec:       %.2f\n", stats.AvgPerSec)
	fmt.Printf("Std Deviation:     %.2f\n", stats.StdDev)
	fmt.Printf("Min/Max:           %d / %d\n", stats.MinPerSec, stats.MaxPerSec)
	fmt.Printf("Percentiles:       p50=%d, p95=%d, p99=%d\n", stats.P50, stats.P95, stats.P99)
	fmt.Println()

	if stats.LatencyEnabled && stats.LatencyCount > 0 {
		fmt.Println("--- Response Latency Summary ---")
		fmt.Printf("Samples:           %d\n", stats.LatencyCount)
		fmt.Printf("Average:           %.2f ms\n", stats.LatencyAvg/1000.0)
		fmt.Printf("Min/Max:           %.2f ms / %.2f ms\n",
			float64(stats.LatencyMin)/1000.0,
			float64(stats.LatencyMax)/1000.0)
		fmt.Printf("p50:               %.2f ms\n", float64(stats.LatencyP50)/1000.0)
		fmt.Printf("p95:               %.2f ms\n", float64(stats.LatencyP95)/1000.0)
		fmt.Printf("p99:               %.2f ms\n", float64(stats.LatencyP99)/1000.0)
		fmt.Println()

		if stats.LatencyP99 > 3000000 {
			fmt.Println("[ALERT] High p99 latency indicates server performance degradation")
		}
		if stats.LatencyP95 > 1000000 {
			fmt.Println("[INFO] Elevated p95 latency detected")
		}
	}

	if stats.AvgPerSec > 0 {
		deviation := (stats.StdDev / stats.AvgPerSec) * 100
		fmt.Printf("Rate Deviation:    %.2f%%\n", deviation)
	}

	// 최종 Pass/Fail 판정
	fmt.Println()
	fmt.Println("=== Test Verdict ===")
	result := EvaluateTestResult(stats)
	if result.Passed {
		fmt.Println("Result: PASS")
	} else {
		fmt.Println("Result: FAIL")
		fmt.Println("Failure reasons:")
		for _, reason := range result.Failures {
			fmt.Printf("  - %s\n", reason)
		}
	}
}

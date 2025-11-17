package metrics

import (
	"context"
	"fmt"
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
	fmt.Printf("Elapsed Time: %v\n", elapsed.Round(time.Second))
	fmt.Println()

	fmt.Printf("Active Sessions: %d\n", stats.Active)
	fmt.Printf("Total Requests:  %d\n", stats.Total)
	fmt.Printf("Success:         %d (%.2f%%)\n", stats.Success, stats.SuccessRate)
	fmt.Printf("Failed:          %d\n", stats.Failed)
	fmt.Println()

	fmt.Printf("Requests/sec:    %.2f (σ=%.2f)\n", stats.AvgPerSec, stats.StdDev)
	fmt.Printf("Min/Max:         %d / %d\n", stats.MinPerSec, stats.MaxPerSec)
	fmt.Printf("Percentiles:     p50=%d, p95=%d, p99=%d\n", stats.P50, stats.P95, stats.P99)
	fmt.Println()

	if stats.AvgPerSec > 0 {
		deviation := (stats.StdDev / stats.AvgPerSec) * 100
		fmt.Printf("Deviation:       %.2f%%\n", deviation)

		if deviation <= 10 {
			fmt.Println("Status:          ✓ Within target (±10%)")
		} else {
			fmt.Println("Status:          ✗ Exceeds target (±10%)")
		}
	}
}

func (r *Reporter) printFinalReport(startTime time.Time) {
	stats := r.collector.GetStats()
	elapsed := time.Since(startTime)

	fmt.Println("\n=== LoadTestForge Final Report ===")
	fmt.Printf("Total Duration:  %v\n", elapsed.Round(time.Millisecond))
	fmt.Println()

	fmt.Printf("Total Requests:  %d\n", stats.Total)
	fmt.Printf("Success:         %d (%.2f%%)\n", stats.Success, stats.SuccessRate)
	fmt.Printf("Failed:          %d\n", stats.Failed)
	fmt.Println()

	fmt.Printf("Avg Req/sec:     %.2f\n", stats.AvgPerSec)
	fmt.Printf("Std Deviation:   %.2f\n", stats.StdDev)
	fmt.Printf("Min/Max:         %d / %d\n", stats.MinPerSec, stats.MaxPerSec)
	fmt.Printf("Percentiles:     p50=%d, p95=%d, p99=%d\n", stats.P50, stats.P95, stats.P99)
	fmt.Println()

	if stats.AvgPerSec > 0 {
		deviation := (stats.StdDev / stats.AvgPerSec) * 100
		fmt.Printf("Deviation:       %.2f%%\n", deviation)
	}
}

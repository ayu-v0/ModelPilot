package benchmark

import (
	"testing"

	"github.com/ayu-v0/ModelPilot/internal/domain"
)

func TestCompareDetectsRegression(t *testing.T) {
	baseline := domain.BenchmarkResult{
		Task:  domain.BenchmarkTask{ID: "baseline"},
		Model: "model-v1",
		Summary: domain.BenchmarkSummary{
			AverageLatency: 100,
			P50Latency:     90,
			P95Latency:     150,
			P99Latency:     200,
			SuccessRate:    1,
			Throughput:     10,
		},
		Quality: map[string]float64{"exact_match": 0.9},
	}
	candidate := domain.BenchmarkResult{
		Task:  domain.BenchmarkTask{ID: "candidate"},
		Model: "model-v2",
		Summary: domain.BenchmarkSummary{
			AverageLatency: 120,
			P50Latency:     90,
			P95Latency:     160,
			P99Latency:     200,
			SuccessRate:    0.98,
			Throughput:     9,
		},
		Quality: map[string]float64{"exact_match": 0.88},
	}

	report := Compare(baseline, candidate, DefaultComparisonConfig())
	if report.Passed {
		t.Fatal("expected comparison to fail")
	}
	if len(report.Regressions) == 0 {
		t.Fatal("expected regressions")
	}
}

func TestComparePassesWithinThreshold(t *testing.T) {
	baseline := domain.BenchmarkResult{Task: domain.BenchmarkTask{ID: "baseline"}, Summary: domain.BenchmarkSummary{P95Latency: 100, SuccessRate: 1, Throughput: 10}, Quality: map[string]float64{"exact_match": 0.9}}
	candidate := domain.BenchmarkResult{Task: domain.BenchmarkTask{ID: "candidate"}, Summary: domain.BenchmarkSummary{P95Latency: 104, SuccessRate: 0.995, Throughput: 9.6}, Quality: map[string]float64{"exact_match": 0.895}}

	report := Compare(baseline, candidate, DefaultComparisonConfig())
	if !report.Passed {
		t.Fatalf("expected comparison to pass: %+v", report.Regressions)
	}
}

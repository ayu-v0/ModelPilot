package metrics

import (
	"testing"
	"time"

	"github.com/ayu-v0/ModelPilot/internal/domain"
)

func TestAggregatorPerformanceMetrics(t *testing.T) {
	results := []domain.RequestResult{
		{Success: true, DurationMillis: 10, InputTokens: 2, OutputTokens: 3},
		{Success: true, DurationMillis: 20, InputTokens: 4, OutputTokens: 6},
		{Success: false, Error: "timeout", DurationMillis: 100},
		{Success: true, DurationMillis: 30, InputTokens: 1, OutputTokens: 2},
	}

	summary := (Aggregator{}).Summary(results, 2*time.Second)
	if summary.TotalRequests != 4 || summary.SuccessRequests != 3 || summary.ErrorCount != 1 {
		t.Fatalf("unexpected counts: %+v", summary)
	}
	if summary.MinLatency != 10 || summary.MaxLatency != 100 {
		t.Fatalf("unexpected min/max latency: %d/%d", summary.MinLatency, summary.MaxLatency)
	}
	if summary.P90Latency != 100 || summary.P95Latency != 100 || summary.P99Latency != 100 {
		t.Fatalf("unexpected latency percentiles: %+v", summary)
	}
	if summary.InputTokens != 7 || summary.OutputTokens != 11 || summary.TotalTokens != 18 {
		t.Fatalf("unexpected token metrics: %+v", summary)
	}
	if summary.Throughput != 2 || summary.TokenThroughput != 9 {
		t.Fatalf("unexpected throughput metrics: %+v", summary)
	}
}

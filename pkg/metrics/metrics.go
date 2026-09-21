package metrics

import (
	"sort"
	"strings"
	"time"

	"github.com/ayu-v0/ModelPilot/internal/domain"
)

type QualityMetric interface {
	Name() string
	Score(domain.Sample, domain.ModelResponse) float64
}

type ExactMatch struct{}

func (ExactMatch) Name() string { return "exact_match" }

func (ExactMatch) Score(sample domain.Sample, response domain.ModelResponse) float64 {
	if strings.TrimSpace(sample.Expected) == strings.TrimSpace(response.Output) {
		return 1
	}
	return 0
}

type Aggregator struct{}

func (Aggregator) Summary(results []domain.RequestResult, elapsed time.Duration) domain.BenchmarkSummary {
	summary := domain.BenchmarkSummary{TotalRequests: len(results)}
	if len(results) == 0 {
		return summary
	}

	latencies := make([]int64, 0, len(results))
	var latencyTotal int64
	var inputTokens int
	var outputTokens int
	for _, result := range results {
		if result.Success {
			summary.SuccessRequests++
		}
		if result.Error != "" {
			summary.ErrorCount++
		}
		latencies = append(latencies, result.DurationMillis)
		latencyTotal += result.DurationMillis
		inputTokens += result.InputTokens
		outputTokens += result.OutputTokens
	}

	summary.FailedRequests = summary.TotalRequests - summary.SuccessRequests
	summary.SuccessRate = float64(summary.SuccessRequests) / float64(summary.TotalRequests)
	summary.ErrorRate = float64(summary.ErrorCount) / float64(summary.TotalRequests)
	summary.AverageLatency = float64(latencyTotal) / float64(len(latencies))
	summary.InputTokens = inputTokens
	summary.OutputTokens = outputTokens
	summary.TotalTokens = inputTokens + outputTokens
	if len(latencies) > 0 {
		summary.MinLatency = latencies[0]
		summary.MaxLatency = latencies[0]
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	for _, latency := range latencies[1:] {
		if latency < summary.MinLatency {
			summary.MinLatency = latency
		}
		if latency > summary.MaxLatency {
			summary.MaxLatency = latency
		}
	}
	summary.P50Latency = percentile(latencies, 0.50)
	summary.P90Latency = percentile(latencies, 0.90)
	summary.P95Latency = percentile(latencies, 0.95)
	summary.P99Latency = percentile(latencies, 0.99)

	if elapsed > 0 {
		summary.Throughput = float64(summary.TotalRequests) / elapsed.Seconds()
		summary.TokenThroughput = float64(summary.TotalTokens) / elapsed.Seconds()
		summary.RequestDuration = elapsed.Milliseconds()
	}
	if summary.TotalRequests > 0 {
		summary.AverageInputTokens = float64(inputTokens) / float64(summary.TotalRequests)
		summary.AverageOutputTokens = float64(outputTokens) / float64(summary.TotalRequests)
	}
	return summary
}

func percentile(values []int64, percentile float64) int64 {
	if len(values) == 0 {
		return 0
	}
	index := int(percentile*float64(len(values)-1) + 0.5)
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

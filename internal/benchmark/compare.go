package benchmark

import (
	"fmt"
	"sort"

	"github.com/ayu-v0/ModelPilot/internal/domain"
)

// ComparisonConfig controls the relative degradation allowed for each metric.
// A comparison passes when no metric exceeds its corresponding threshold.
type ComparisonConfig struct {
	LatencyThresholdPercent    float64
	ThroughputThresholdPercent float64
	SuccessThresholdPercent    float64
	QualityThresholdPercent    float64
}

func DefaultComparisonConfig() ComparisonConfig {
	return ComparisonConfig{
		LatencyThresholdPercent:    5,
		ThroughputThresholdPercent: 5,
		SuccessThresholdPercent:    1,
		QualityThresholdPercent:    1,
	}
}

type MetricComparison struct {
	Name              string  `json:"name"`
	Direction         string  `json:"direction"`
	Baseline          float64 `json:"baseline"`
	Candidate         float64 `json:"candidate"`
	Delta             float64 `json:"delta"`
	DeltaPercent      float64 `json:"delta_percent"`
	ThresholdPercent  float64 `json:"threshold_percent"`
	Regression        bool    `json:"regression"`
	RegressionMessage string  `json:"regression_message,omitempty"`
}

type ComparisonReport struct {
	BaselineTaskID  string             `json:"baseline_task_id"`
	CandidateTaskID string             `json:"candidate_task_id"`
	BaselineModel   string             `json:"baseline_model"`
	CandidateModel  string             `json:"candidate_model"`
	Passed          bool               `json:"passed"`
	Regressions     []string           `json:"regressions,omitempty"`
	Metrics         []MetricComparison `json:"metrics"`
}

func Compare(baseline, candidate domain.BenchmarkResult, config ComparisonConfig) ComparisonReport {
	report := ComparisonReport{
		BaselineTaskID:  baseline.Task.ID,
		CandidateTaskID: candidate.Task.ID,
		BaselineModel:   baseline.Model,
		CandidateModel:  candidate.Model,
		Passed:          true,
		Metrics:         make([]MetricComparison, 0),
	}

	latencyThreshold := config.LatencyThresholdPercent
	throughputThreshold := config.ThroughputThresholdPercent
	successThreshold := config.SuccessThresholdPercent
	qualityThreshold := config.QualityThresholdPercent

	latencyMetrics := []struct {
		name      string
		baseline  float64
		candidate float64
	}{
		{"average_latency_ms", baseline.Summary.AverageLatency, candidate.Summary.AverageLatency},
		{"p50_latency_ms", float64(baseline.Summary.P50Latency), float64(candidate.Summary.P50Latency)},
		{"p90_latency_ms", float64(baseline.Summary.P90Latency), float64(candidate.Summary.P90Latency)},
		{"p95_latency_ms", float64(baseline.Summary.P95Latency), float64(candidate.Summary.P95Latency)},
		{"p99_latency_ms", float64(baseline.Summary.P99Latency), float64(candidate.Summary.P99Latency)},
	}
	for _, metric := range latencyMetrics {
		report.add(metricComparison(metric.name, metric.baseline, metric.candidate, "lower", latencyThreshold))
	}

	report.add(metricComparison("success_rate", baseline.Summary.SuccessRate, candidate.Summary.SuccessRate, "higher", successThreshold))
	report.add(metricComparison("throughput_rps", baseline.Summary.Throughput, candidate.Summary.Throughput, "higher", throughputThreshold))

	qualityNames := make([]string, 0, len(baseline.Quality)+len(candidate.Quality))
	seen := make(map[string]struct{})
	for name := range baseline.Quality {
		seen[name] = struct{}{}
		qualityNames = append(qualityNames, name)
	}
	for name := range candidate.Quality {
		if _, ok := seen[name]; !ok {
			qualityNames = append(qualityNames, name)
		}
	}
	sort.Strings(qualityNames)
	for _, name := range qualityNames {
		report.add(metricComparison(
			"quality."+name,
			baseline.Quality[name],
			candidate.Quality[name],
			"higher",
			qualityThreshold,
		))
	}

	return report
}

func (r *ComparisonReport) add(metric MetricComparison) {
	r.Metrics = append(r.Metrics, metric)
	if metric.Regression {
		r.Passed = false
		r.Regressions = append(r.Regressions, metric.Name)
	}
}

func metricComparison(name string, baseline, candidate float64, direction string, threshold float64) MetricComparison {
	metric := MetricComparison{
		Name:             name,
		Direction:        direction,
		Baseline:         baseline,
		Candidate:        candidate,
		Delta:            candidate - baseline,
		DeltaPercent:     relativeChange(baseline, candidate),
		ThresholdPercent: threshold,
	}

	if direction == "lower" {
		metric.Regression = isLowerMetricRegression(baseline, candidate, threshold)
	} else {
		metric.Regression = isHigherMetricRegression(baseline, candidate, threshold)
	}
	if metric.Regression {
		metric.RegressionMessage = fmt.Sprintf("%s changed from %.4f to %.4f beyond %.2f%% threshold", name, baseline, candidate, threshold)
	}
	return metric
}

func relativeChange(baseline, candidate float64) float64 {
	if baseline == 0 {
		if candidate == 0 {
			return 0
		}
		return 100
	}
	return (candidate - baseline) / baseline * 100
}

func isLowerMetricRegression(baseline, candidate, threshold float64) bool {
	if baseline == 0 {
		return candidate > 0
	}
	return candidate > baseline*(1+threshold/100)
}

func isHigherMetricRegression(baseline, candidate, threshold float64) bool {
	if baseline == 0 {
		return candidate == 0
	}
	return candidate < baseline*(1-threshold/100)
}

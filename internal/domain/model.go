package domain

import "context"

type ModelRequest struct {
	Input    string            `json:"input"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type ModelResponse struct {
	Output       string `json:"output"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
}

type Model interface {
	Name() string
	Invoke(ctx context.Context, request ModelRequest) (ModelResponse, error)
}

type Sample struct {
	ID       string            `json:"id"`
	Input    string            `json:"input"`
	Expected string            `json:"expected,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Dataset struct {
	Name    string   `json:"name"`
	Version string   `json:"version,omitempty"`
	Samples []Sample `json:"samples"`
}

type BenchmarkTask struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Model          string   `json:"model"`
	Dataset        string   `json:"dataset"`
	Concurrency    int      `json:"concurrency"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	QualityMetrics []string `json:"quality_metrics,omitempty"`
}

type RequestResult struct {
	SampleID       string `json:"sample_id"`
	Success        bool   `json:"success"`
	StatusCode     int    `json:"status_code,omitempty"`
	ErrorType      string `json:"error_type,omitempty"`
	Error          string `json:"error,omitempty"`
	DurationMillis int64  `json:"duration_ms"`
	InputTokens    int    `json:"input_tokens,omitempty"`
	OutputTokens   int    `json:"output_tokens,omitempty"`
	Output         string `json:"output,omitempty"`
}

type BenchmarkSummary struct {
	TotalRequests       int     `json:"total_requests"`
	SuccessRequests     int     `json:"success_requests"`
	FailedRequests      int     `json:"failed_requests"`
	ErrorCount          int     `json:"error_count"`
	SuccessRate         float64 `json:"success_rate"`
	ErrorRate           float64 `json:"error_rate"`
	RequestDuration     int64   `json:"request_duration_ms"`
	MinLatency          int64   `json:"min_latency_ms"`
	AverageLatency      float64 `json:"average_latency_ms"`
	P50Latency          int64   `json:"p50_latency_ms"`
	P90Latency          int64   `json:"p90_latency_ms"`
	P95Latency          int64   `json:"p95_latency_ms"`
	P99Latency          int64   `json:"p99_latency_ms"`
	MaxLatency          int64   `json:"max_latency_ms"`
	Throughput          float64 `json:"throughput_rps"`
	TokenThroughput     float64 `json:"token_throughput"`
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	TotalTokens         int     `json:"total_tokens"`
	AverageInputTokens  float64 `json:"average_input_tokens"`
	AverageOutputTokens float64 `json:"average_output_tokens"`
}

type BenchmarkResult struct {
	Task      BenchmarkTask      `json:"task"`
	Model     string             `json:"model"`
	StartedAt string             `json:"started_at"`
	EndedAt   string             `json:"ended_at"`
	Summary   BenchmarkSummary   `json:"summary"`
	Quality   map[string]float64 `json:"quality,omitempty"`
	Requests  []RequestResult    `json:"requests"`
}

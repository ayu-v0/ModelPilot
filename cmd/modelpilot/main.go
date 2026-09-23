package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ayu-v0/ModelPilot/internal/benchmark"
	"github.com/ayu-v0/ModelPilot/internal/config"
	"github.com/ayu-v0/ModelPilot/internal/domain"
	"github.com/ayu-v0/ModelPilot/internal/storage"
	"github.com/ayu-v0/ModelPilot/pkg/model"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "", "YAML or JSON configuration path")
	datasetPath := flag.String("dataset", "", "JSONL dataset path")
	output := flag.String("output", "", "result directory")
	concurrency := flag.Int("concurrency", 0, "request concurrency")
	modelType := flag.String("model-type", "", "model type: mock, http, openai")
	modelName := flag.String("model-name", "", "model name")
	endpoint := flag.String("endpoint", "", "HTTP model endpoint")
	token := flag.String("token", "", "Bearer token; prefer MODELPILOT_TOKEN")
	timeout := flag.Int("timeout", 0, "per-request timeout in seconds")
	limit := flag.Int("limit", 0, "maximum number of dataset samples; 0 means all")
	temperature := flag.Float64("temperature", -1, "OpenAI-compatible sampling temperature")
	maxTokens := flag.Int("max-tokens", -1, "OpenAI-compatible maximum output tokens")

	baselineID := flag.String("baseline", "", "baseline result id for comparison, without .json suffix")
	candidateID := flag.String("candidate", "", "candidate result id for comparison, without .json suffix")
	latencyThreshold := flag.Float64("latency-threshold", -1, "allowed latency regression percentage")
	throughputThreshold := flag.Float64("throughput-threshold", -1, "allowed throughput regression percentage")
	successThreshold := flag.Float64("success-threshold", -1, "allowed success rate regression percentage")
	qualityThreshold := flag.Float64("quality-threshold", -1, "allowed quality metric regression percentage")
	jsonOutput := flag.Bool("json", false, "print comparison report as JSON")
	flag.Parse()

	loaded, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	applyConfig(&loaded, datasetPath, output, concurrency, modelType, modelName, endpoint, token, timeout, limit, temperature, maxTokens, baselineID, candidateID, latencyThreshold, throughputThreshold, successThreshold, qualityThreshold)
	if *token == "" {
		*token = os.Getenv("MODELPILOT_TOKEN")
	}

	if *baselineID != "" || *candidateID != "" {
		if *baselineID == "" || *candidateID == "" {
			log.Fatal("both -baseline and -candidate are required for comparison")
		}
		exitCode, err := compareResults(*output, *baselineID, *candidateID, benchmark.ComparisonConfig{
			LatencyThresholdPercent:    valueOrDefault(*latencyThreshold, 5),
			ThroughputThresholdPercent: valueOrDefault(*throughputThreshold, 5),
			SuccessThresholdPercent:    valueOrDefault(*successThreshold, 1),
			QualityThresholdPercent:    valueOrDefault(*qualityThreshold, 1),
		}, *jsonOutput)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(exitCode)
	}

	if *datasetPath == "" {
		fmt.Println("ModelPilot benchmark runner")
		fmt.Println("Version:", version)
		fmt.Println("Benchmark usage: modelpilot -dataset ./dataset.jsonl [-model-type mock|http|openai]")
		fmt.Println("Config usage:    modelpilot -config ./configs/benchmark.example.yaml")
		fmt.Println("Compare usage:   modelpilot -baseline benchmark-1 -candidate benchmark-2 [-output ./data/results]")
		return
	}

	dataset, err := benchmark.LoadJSONL(*datasetPath)
	if err != nil {
		log.Fatal(err)
	}
	if *limit > 0 && *limit < len(dataset.Samples) {
		dataset.Samples = dataset.Samples[:*limit]
	}

	selectedModel, err := buildModel(*modelType, *modelName, *endpoint, *token, valueOrDefault(*temperature, 0), valueOrDefaultInt(*maxTokens, 4096))
	if err != nil {
		log.Fatal(err)
	}

	benchmarkConcurrency := valueOrDefaultInt(*concurrency, 1)
	benchmarkTimeout := valueOrDefaultInt(*timeout, 30)
	resultDir := *output
	if resultDir == "" {
		resultDir = "./data/results"
	}
	task := domain.BenchmarkTask{
		ID:             fmt.Sprintf("benchmark-%d", time.Now().UnixNano()),
		Name:           "offline benchmark",
		Model:          selectedModel.Name(),
		Dataset:        dataset.Name,
		Concurrency:    benchmarkConcurrency,
		TimeoutSeconds: benchmarkTimeout,
		QualityMetrics: []string{"exact_match"},
	}

	result, err := (benchmark.Runner{
		Model: selectedModel,
		Store: storage.FileStore{Dir: resultDir},
	}).Run(context.Background(), task, dataset)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("task=%s model=%s requests=%d success_rate=%.2f error_rate=%.2f avg=%.2fms p90=%dms p95=%dms p99=%dms throughput=%.2frps token_throughput=%.2ftps quality=%v\n",
		result.Task.ID, result.Model, result.Summary.TotalRequests, result.Summary.SuccessRate, result.Summary.ErrorRate,
		result.Summary.AverageLatency, result.Summary.P90Latency, result.Summary.P95Latency, result.Summary.P99Latency,
		result.Summary.Throughput, result.Summary.TokenThroughput, result.Quality)
}

func applyConfig(c *config.Config, dataset, output *string, concurrency *int, modelType, modelName, endpoint, token *string, timeout, limit *int, temperature *float64, maxTokens *int, baseline, candidate *string, latency, throughput, success, quality *float64) {
	if *dataset == "" {
		*dataset = c.Dataset
	}
	if *output == "" {
		*output = c.Output
	}
	if *concurrency == 0 {
		*concurrency = c.Benchmark.Concurrency
	}
	if *timeout == 0 {
		*timeout = c.Benchmark.TimeoutSeconds
	}
	if *limit == 0 {
		*limit = c.Benchmark.Limit
	}
	if *modelType == "" {
		*modelType = c.Model.Type
	}
	if *modelName == "" {
		*modelName = c.Model.Name
	}
	if *endpoint == "" {
		*endpoint = c.Model.Endpoint
	}
	if *token == "" {
		*token = expandEnv(c.Model.Token)
	}
	if *temperature < 0 {
		*temperature = c.Model.Temperature
	}
	if *maxTokens < 0 {
		*maxTokens = c.Model.MaxTokens
	}
	if *baseline == "" {
		*baseline = c.Compare.BaselineID
	}
	if *candidate == "" {
		*candidate = c.Compare.CandidateID
	}
	if *latency < 0 && c.Compare.LatencyThreshold > 0 {
		*latency = c.Compare.LatencyThreshold
	}
	if *throughput < 0 && c.Compare.ThroughputThreshold > 0 {
		*throughput = c.Compare.ThroughputThreshold
	}
	if *success < 0 && c.Compare.SuccessThreshold > 0 {
		*success = c.Compare.SuccessThreshold
	}
	if *quality < 0 && c.Compare.QualityThreshold > 0 {
		*quality = c.Compare.QualityThreshold
	}
}

func expandEnv(value string) string {
	return os.Expand(value, func(key string) string { return os.Getenv(key) })
}

func valueOrDefault(value, fallback float64) float64 {
	if value < 0 {
		return fallback
	}
	return value
}
func valueOrDefaultInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func compareResults(resultDir, baselineID, candidateID string, config benchmark.ComparisonConfig, asJSON bool) (int, error) {
	if resultDir == "" {
		resultDir = "./data/results"
	}
	store := storage.FileStore{Dir: resultDir}
	baseline, err := store.Get(context.Background(), baselineID)
	if err != nil {
		return 1, fmt.Errorf("load baseline result: %w", err)
	}
	candidate, err := store.Get(context.Background(), candidateID)
	if err != nil {
		return 1, fmt.Errorf("load candidate result: %w", err)
	}
	report := benchmark.Compare(baseline, candidate, config)
	if asJSON {
		payload, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return 1, err
		}
		fmt.Println(string(payload))
	} else {
		printComparison(report)
	}
	if report.Passed {
		return 0, nil
	}
	return 2, nil
}

func printComparison(report benchmark.ComparisonReport) {
	status := "PASSED"
	if !report.Passed {
		status = "FAILED"
	}
	fmt.Printf("comparison=%s baseline=%s candidate=%s\n", status, report.BaselineTaskID, report.CandidateTaskID)
	fmt.Printf("baseline_model=%s candidate_model=%s\n", report.BaselineModel, report.CandidateModel)
	for _, metric := range report.Metrics {
		marker := "OK"
		if metric.Regression {
			marker = "REGRESSION"
		}
		fmt.Printf("[%s] %-24s baseline=%10.4f candidate=%10.4f delta=%10.4f delta_pct=%8.2f%% threshold=%5.2f%% direction=%s\n", marker, metric.Name, metric.Baseline, metric.Candidate, metric.Delta, metric.DeltaPercent, metric.ThresholdPercent, metric.Direction)
	}
}

func buildModel(modelType, name, endpoint, token string, temperature float64, maxTokens int) (domain.Model, error) {
	switch modelType {
	case "", "mock":
		return model.Mock{ModelName: name}, nil
	case "http":
		if endpoint == "" {
			return nil, fmt.Errorf("-endpoint is required for model type http")
		}
		return model.HTTP{ModelName: name, Endpoint: endpoint, Token: token}, nil
	case "openai":
		if endpoint == "" {
			return nil, fmt.Errorf("-endpoint is required for model type openai")
		}
		return model.OpenAICompatible{ModelName: name, Endpoint: endpoint, Token: token, Temperature: temperature, MaxTokens: maxTokens}, nil
	default:
		return nil, fmt.Errorf("unsupported model type %q", modelType)
	}
}

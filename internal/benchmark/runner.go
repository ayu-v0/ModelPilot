package benchmark

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ayu-v0/ModelPilot/internal/domain"
	"github.com/ayu-v0/ModelPilot/pkg/metrics"
	"github.com/ayu-v0/ModelPilot/pkg/model"
)

type ResultStore interface {
	Save(context.Context, domain.BenchmarkResult) error
}

type Runner struct {
	Model domain.Model
	Store ResultStore
}

func (r Runner) Run(ctx context.Context, task domain.BenchmarkTask, dataset domain.Dataset) (domain.BenchmarkResult, error) {
	if r.Model == nil {
		return domain.BenchmarkResult{}, fmt.Errorf("model is required")
	}

	concurrency := task.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	timeout := time.Duration(task.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	started := time.Now()
	jobs := make(chan domain.Sample)
	results := make(chan domain.RequestResult, len(dataset.Samples))
	var workers sync.WaitGroup

	worker := func() {
		defer workers.Done()
		for sample := range jobs {
			select {
			case <-ctx.Done():
				results <- domain.RequestResult{SampleID: sample.ID, Error: ctx.Err().Error()}
				continue
			default:
			}

			requestContext, cancel := context.WithTimeout(ctx, timeout)
			begin := time.Now()
			response, err := r.Model.Invoke(requestContext, domain.ModelRequest{
				Input:    sample.Input,
				Metadata: sample.Metadata,
			})
			cancel()

			result := domain.RequestResult{
				SampleID:       sample.ID,
				Success:        err == nil,
				DurationMillis: time.Since(begin).Milliseconds(),
			}
			if err != nil {
				result.Error = err.Error()
				result.ErrorType = classifyError(err)
				var httpErr *model.HTTPError
				if errors.As(err, &httpErr) {
					result.StatusCode = httpErr.StatusCode
				}
			} else {
				result.Output = response.Output
				result.InputTokens = response.InputTokens
				result.OutputTokens = response.OutputTokens
			}
			results <- result
		}
	}

	for i := 0; i < concurrency; i++ {
		workers.Add(1)
		go worker()
	}
	go func() {
		defer close(jobs)
		for _, sample := range dataset.Samples {
			select {
			case jobs <- sample:
			case <-ctx.Done():
				return
			}
		}
	}()

	workers.Wait()
	close(results)

	result := domain.BenchmarkResult{
		Task:      task,
		Model:     r.Model.Name(),
		StartedAt: started.UTC().Format(time.RFC3339Nano),
		EndedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		Requests:  make([]domain.RequestResult, 0, len(dataset.Samples)),
	}
	for requestResult := range results {
		result.Requests = append(result.Requests, requestResult)
	}

	aggregator := metrics.Aggregator{}
	result.Summary = aggregator.Summary(result.Requests, time.Since(started))
	result.Quality = Quality(dataset, result.Requests, task.QualityMetrics)
	if r.Store != nil {
		if err := r.Store.Save(ctx, result); err != nil {
			return domain.BenchmarkResult{}, err
		}
	}
	return result, nil
}

func classifyError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	var httpErr *model.HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
			return "http_4xx"
		}
		if httpErr.StatusCode >= 500 {
			return "http_5xx"
		}
		return "http_error"
	}
	return "model_error"
}

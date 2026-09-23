package benchmark

import (
	"context"
	"testing"

	"github.com/ayu-v0/ModelPilot/internal/domain"
	"github.com/ayu-v0/ModelPilot/pkg/model"
)

func TestRunner(t *testing.T) {
	r := Runner{Model: model.Mock{ModelName: "mock"}}
	result, err := r.Run(context.Background(), domain.BenchmarkTask{ID: "test", Concurrency: 2}, domain.Dataset{Samples: []domain.Sample{{ID: "1", Input: "hello", Expected: "hello"}, {ID: "2", Input: "world", Expected: "world"}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.SuccessRate != 1 {
		t.Fatalf("success rate = %v", result.Summary.SuccessRate)
	}
}

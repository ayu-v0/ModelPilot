package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ayu-v0/ModelPilot/internal/domain"
)

type ResultStore interface {
	Save(context.Context, domain.BenchmarkResult) error
	Get(context.Context, string) (domain.BenchmarkResult, error)
}
type FileStore struct{ Dir string }

func (s FileStore) Save(ctx context.Context, result domain.BenchmarkResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if result.Task.ID == "" {
		return fmt.Errorf("task id is required")
	}
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, result.Task.ID+".json"), b, 0644)
}
func (s FileStore) Get(ctx context.Context, id string) (domain.BenchmarkResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.BenchmarkResult{}, err
	}
	b, err := os.ReadFile(filepath.Join(s.Dir, id+".json"))
	if err != nil {
		return domain.BenchmarkResult{}, err
	}
	var r domain.BenchmarkResult
	err = json.Unmarshal(b, &r)
	return r, err
}

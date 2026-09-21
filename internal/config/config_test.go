package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "benchmark.yaml")
	content := []byte("dataset: ./data.jsonl\nmodel:\n  type: mock\nbenchmark:\n  concurrency: 4\n")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Dataset != "./data.jsonl" || loaded.Model.Type != "mock" || loaded.Benchmark.Concurrency != 4 {
		t.Fatalf("unexpected config: %+v", loaded)
	}
}

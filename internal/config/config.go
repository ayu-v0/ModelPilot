package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Dataset   string          `json:"dataset,omitempty" yaml:"dataset,omitempty"`
	Output    string          `json:"output,omitempty" yaml:"output,omitempty"`
	Model     ModelConfig     `json:"model,omitempty" yaml:"model,omitempty"`
	Benchmark BenchmarkConfig `json:"benchmark,omitempty" yaml:"benchmark,omitempty"`
	Compare   CompareConfig   `json:"compare,omitempty" yaml:"compare,omitempty"`
}

type ModelConfig struct {
	Type        string  `json:"type,omitempty" yaml:"type,omitempty"`
	Name        string  `json:"name,omitempty" yaml:"name,omitempty"`
	Endpoint    string  `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Token       string  `json:"token,omitempty" yaml:"token,omitempty"`
	Temperature float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
}

type BenchmarkConfig struct {
	Concurrency    int `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
	TimeoutSeconds int `json:"timeout_seconds,omitempty" yaml:"timeout_seconds,omitempty"`
	Limit          int `json:"limit,omitempty" yaml:"limit,omitempty"`
}

type CompareConfig struct {
	BaselineID          string  `json:"baseline,omitempty" yaml:"baseline,omitempty"`
	CandidateID         string  `json:"candidate,omitempty" yaml:"candidate,omitempty"`
	LatencyThreshold    float64 `json:"latency_threshold,omitempty" yaml:"latency_threshold,omitempty"`
	ThroughputThreshold float64 `json:"throughput_threshold,omitempty" yaml:"throughput_threshold,omitempty"`
	SuccessThreshold    float64 `json:"success_threshold,omitempty" yaml:"success_threshold,omitempty"`
	QualityThreshold    float64 `json:"quality_threshold,omitempty" yaml:"quality_threshold,omitempty"`
}

func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var result Config
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &result)
	default:
		err = json.Unmarshal(data, &result)
	}
	if err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	return result, nil
}

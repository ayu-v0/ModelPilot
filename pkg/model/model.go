package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ayu-v0/ModelPilot/internal/domain"
)

type Adapter interface {
	Name() string
	Invoke(context.Context, domain.ModelRequest) (domain.ModelResponse, error)
}

type Mock struct {
	ModelName string
}

func (m Mock) Name() string {
	if m.ModelName == "" {
		return "mock"
	}
	return m.ModelName
}

func (m Mock) Invoke(ctx context.Context, req domain.ModelRequest) (domain.ModelResponse, error) {
	select {
	case <-ctx.Done():
		return domain.ModelResponse{}, ctx.Err()
	default:
	}

	tokens := len(strings.Fields(req.Input))
	return domain.ModelResponse{
		Output:       req.Input,
		InputTokens:  tokens,
		OutputTokens: tokens,
	}, nil
}

type HTTP struct {
	ModelName string
	Endpoint  string
	Token     string
	Client    *http.Client
}

func (m HTTP) Name() string {
	if m.ModelName == "" {
		return "http"
	}
	return m.ModelName
}

func (m HTTP) Invoke(ctx context.Context, req domain.ModelRequest) (domain.ModelResponse, error) {
	if m.Endpoint == "" {
		return domain.ModelResponse{}, fmt.Errorf("http endpoint is required")
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return domain.ModelResponse{}, err
	}

	body, err := m.postJSON(ctx, payload)
	if err != nil {
		return domain.ModelResponse{}, err
	}

	var result domain.ModelResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.ModelResponse{}, err
	}
	return result, nil
}

func (m HTTP) postJSON(ctx context.Context, payload []byte) ([]byte, error) {
	httpClient := m.Client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if m.Token != "" {
		req.Header.Set("Authorization", "Bearer "+m.Token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if readErr != nil {
			return nil, fmt.Errorf("model endpoint returned status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("model endpoint returned status %d: %s", resp.StatusCode, string(body))
	}
	if readErr != nil {
		return nil, readErr
	}
	return body, nil
}

type OpenAICompatible struct {
	ModelName   string
	Endpoint    string
	Token       string
	Temperature float64
	MaxTokens   int
	Client      *http.Client
}

func (m OpenAICompatible) Name() string {
	if m.ModelName == "" {
		return "openai-compatible"
	}
	return m.ModelName
}

func (m OpenAICompatible) Invoke(ctx context.Context, req domain.ModelRequest) (domain.ModelResponse, error) {
	if m.Endpoint == "" {
		return domain.ModelResponse{}, fmt.Errorf("openai compatible endpoint is required")
	}

	payload := openAIChatRequest{
		Model: m.ModelName,
		Messages: []openAIMessage{
			{Role: "user", Content: req.Input},
		},
		Temperature: m.Temperature,
		MaxTokens:   m.MaxTokens,
	}
	if payload.Model == "" {
		payload.Model = "default"
	}

	body, err := HTTP{
		Endpoint: m.Endpoint,
		Token:    m.Token,
		Client:   m.Client,
	}.postJSON(ctx, mustJSON(payload))
	if err != nil {
		return domain.ModelResponse{}, err
	}

	var result openAIChatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.ModelResponse{}, err
	}
	if len(result.Choices) == 0 {
		return domain.ModelResponse{}, fmt.Errorf("openai compatible response has no choices")
	}

	return domain.ModelResponse{
		Output:       result.Choices[0].Message.Content,
		InputTokens:  result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
	}, nil
}

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func mustJSON(v any) []byte {
	payload, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return payload
}

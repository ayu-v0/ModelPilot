package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/your-org/modelpilot/internal/domain"
)

func TestHTTPAdapter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("missing authorization header")
		}
		var request domain.ModelRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(domain.ModelResponse{Output: request.Input, InputTokens: 1, OutputTokens: 1})
	}))
	defer server.Close()

	adapter := HTTP{ModelName: "test-http", Endpoint: server.URL, Token: "test-token"}
	response, err := adapter.Invoke(context.Background(), domain.ModelRequest{Input: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Output != "hello" {
		t.Fatalf("output = %q", response.Output)
	}
}

func TestOpenAICompatibleAdapter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request openAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Messages[0].Content != "hello" {
			t.Fatalf("input = %q", request.Messages[0].Content)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]string{"role": "assistant", "content": "hi"},
			}},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	adapter := OpenAICompatible{ModelName: "test-model", Endpoint: server.URL}
	response, err := adapter.Invoke(context.Background(), domain.ModelRequest{Input: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Output != "hi" || response.InputTokens != 1 || response.OutputTokens != 1 {
		t.Fatalf("response = %+v", response)
	}
}
